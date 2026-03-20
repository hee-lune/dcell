package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/config"
	"github.com/heelune/dcell/internal/vcs"
)

func migrateCmd() *cobra.Command {
	var vcsType string

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "既存リポジトリをbare構造に移行",
		Long: `既存のGitリポジトリをdcellのbare-first構造に移行します。

移行内容:
  - .git/ を .bare/ に移動
  - 現在のディレクトリを main/ worktree として再構成
  - .dcell/config.toml を作成

注意:
  この操作は破壊的変更です。移行前にバックアップを推奨します。

例:
  # 通常のgitリポジトリで実行
  cd my-project
  dcell migrate

  # 作成される構造:
  # my-project/
  #   .bare/       ← 元の.git/
  #   main/        ← 現在の内容がここに移動
  #   .dcell/      ← dcell設定`,
		RunE: func(cmd *cobra.Command, args []string) error {
			currentDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("現在のディレクトリを取得できません: %w", err)
			}

			// Check if this is already a dcell project
			if _, err := os.Stat(filepath.Join(currentDir, ".bare")); err == nil {
				return fmt.Errorf("このディレクトリは既にdcellプロジェクトです (.bare/ が存在します)")
			}

			// Check if this is a git repository
			gitDir := filepath.Join(currentDir, ".git")
			if _, err := os.Stat(gitDir); err != nil {
				return fmt.Errorf("このディレクトリはgitリポジトリではありません (.git/ が見つかりません)")
			}

			projectName := filepath.Base(currentDir)
			fmt.Printf("'%s' をdcellプロジェクトに移行します...\n", projectName)

			// Determine VCS type
			if vcsType == "" {
				// Auto-detect: try jj first
				if _, err := os.Stat(filepath.Join(currentDir, ".jj")); err == nil {
					vcsType = "jj"
				} else {
					vcsType = "git"
				}
			}

			// Create temp directory for staging
			tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("dcell-migrate-%s-%d", projectName, os.Getpid()))
			if err := os.MkdirAll(tempDir, 0755); err != nil {
				return fmt.Errorf("一時ディレクトリの作成に失敗しました: %w", err)
			}
			defer os.RemoveAll(tempDir)

			// Move current directory contents to temp
			fmt.Println("現在のディレクトリを一時保存中...")
			entries, err := os.ReadDir(currentDir)
			if err != nil {
				return fmt.Errorf("ディレクトリの読み取りに失敗しました: %w", err)
			}

			for _, entry := range entries {
				if entry.Name() == ".git" {
					continue // Skip .git, we'll handle it separately
				}
				src := filepath.Join(currentDir, entry.Name())
				dst := filepath.Join(tempDir, entry.Name())
				if err := os.Rename(src, dst); err != nil {
					// Try copy if rename fails (cross-device)
					if err := copyDir(src, dst); err != nil {
						return fmt.Errorf("ファイルの移動に失敗しました (%s): %w", entry.Name(), err)
					}
					os.RemoveAll(src)
				}
			}

			// Move .git to .bare
			barePath := filepath.Join(currentDir, ".bare")
			fmt.Println(".git/ を .bare/ に移動中...")
			if err := os.Rename(gitDir, barePath); err != nil {
				// Rollback
				rollbackMigrate(currentDir, tempDir)
				return fmt.Errorf(".git/ の移動に失敗しました: %w", err)
			}

			// Create main directory and move contents back
			mainPath := filepath.Join(currentDir, "main")
			fmt.Println("main/ ディレクトリを作成中...")
			if err := os.MkdirAll(mainPath, 0755); err != nil {
				rollbackMigrate(currentDir, tempDir)
				return fmt.Errorf("main/ の作成に失敗しました: %w", err)
			}

			// Move contents from temp to main/
			entries, _ = os.ReadDir(tempDir)
			for _, entry := range entries {
				src := filepath.Join(tempDir, entry.Name())
				dst := filepath.Join(mainPath, entry.Name())
				if err := os.Rename(src, dst); err != nil {
					if err := copyDir(src, dst); err != nil {
						rollbackMigrate(currentDir, tempDir)
						return fmt.Errorf("ファイルの復元に失敗しました (%s): %w", entry.Name(), err)
					}
					os.RemoveAll(src)
				}
			}

			// Add main as worktree
			fmt.Println("main/ をworktreeとして登録中...")
			switch vcsType {
			case "git":
				g := &vcs.Git{RepoPath: barePath}
				// Update gitdir link in main/
				if err := g.AddMainWorktree(barePath, mainPath); err != nil {
					// If adding fails, the main worktree might already exist
					// Just continue
					fmt.Printf("警告: main worktreeの登録で問題が発生しました: %v\n", err)
				}

			case "jj":
				j := &vcs.JJ{RepoPath: barePath}
				// For JJ, we need to colocate
				if err := j.AddMainWorktree(barePath, mainPath); err != nil {
					fmt.Printf("警告: main workspaceの登録で問題が発生しました: %v\n", err)
				}

			default:
				rollbackMigrate(currentDir, tempDir)
				return fmt.Errorf("不明なVCSタイプ: %s", vcsType)
			}

			// Create dcell config
			cfg := config.Default()
			cfg.VCS.Prefer = vcsType
			if err := cfg.SaveProject(currentDir); err != nil {
				fmt.Fprintf(os.Stderr, "警告: 設定ファイルの作成に失敗しました: %v\n", err)
			}

			fmt.Printf("\n✅ 移行完了！\n")
			fmt.Printf("\n新しい構造:\n")
			fmt.Printf("  %s/\n", projectName)
			fmt.Printf("    .bare/      ← bareリポジトリ\n")
			fmt.Printf("    main/        ← 元の内容\n")
			fmt.Printf("    .dcell/      ← dcell設定\n")
			fmt.Printf("\n次のステップ:\n")
			fmt.Printf("  cd main\n")
			fmt.Printf("  dcell create feature-x\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&vcsType, "vcs", "", "VCSタイプ (git または jj, 未指定で自動検出)")

	return cmd
}

// rollbackMigrate attempts to restore the original state on failure
func rollbackMigrate(currentDir, tempDir string) {
	fmt.Println("エラーが発生したため、ロールバックを試みます...")

	// Move .git back if it exists
	barePath := filepath.Join(currentDir, ".bare")
	gitPath := filepath.Join(currentDir, ".git")
	if _, err := os.Stat(barePath); err == nil {
		os.Rename(barePath, gitPath)
	}

	// Move contents from temp back
	entries, _ := os.ReadDir(tempDir)
	for _, entry := range entries {
		src := filepath.Join(tempDir, entry.Name())
		dst := filepath.Join(currentDir, entry.Name())
		os.Rename(src, dst)
	}

	// Remove main/ if it exists
	mainPath := filepath.Join(currentDir, "main")
	os.RemoveAll(mainPath)

	fmt.Println("ロールバック完了。手動での確認を推奨します。")
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}

		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			srcPath := filepath.Join(src, entry.Name())
			dstPath := filepath.Join(dst, entry.Name())
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		}
	} else {
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, info.Mode()); err != nil {
			return err
		}
	}

	return nil
}
