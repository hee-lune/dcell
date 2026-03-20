package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/config"
	"github.com/heelune/dcell/internal/vcs"
)

func initCmd() *cobra.Command {
	var (
		cloneURL string
		branch   string
		vcsType  string
	)

	cmd := &cobra.Command{
		Use:   "init <directory>",
		Short: "新規プロジェクトを初期化（bareリポジトリ）",
		Long: `新規プロジェクトをbareリポジトリとして初期化します。
必ず .bare/ ディレクトリと main/ worktree を作成します。

作成される構造:
  my-project/
    .bare/          # bareリポジトリ
    main/           # main worktree
    .dcell/         # dcell設定

例:
  # 新規プロジェクトを作成
  dcell init my-project

  # 既存リポジトリをクローン
  dcell init my-project --clone https://github.com/user/repo.git

  # Jujutsuを使用
  dcell init my-project --vcs jj`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]
			projectDir := filepath.Join(".", projectName)

			// Check if directory already exists
			if _, err := os.Stat(projectDir); err == nil {
				return fmt.Errorf("ディレクトリ '%s' は既に存在します", projectName)
			}

			// Determine VCS type
			if vcsType == "" {
				vcsType = "git" // default
			}

			// Create project directory structure
			if err := os.MkdirAll(projectDir, 0755); err != nil {
				return fmt.Errorf("プロジェクトディレクトリの作成に失敗しました: %w", err)
			}

			barePath := filepath.Join(projectDir, ".bare")
			var mainPath string
			var v vcs.VCS
			var err error

			switch vcsType {
			case "git":
				g := &vcs.Git{}
				if cloneURL != "" {
					// Clone mode: clone as bare then add main worktree
					fmt.Printf("クローン中: %s\n", cloneURL)
					if err := g.CloneBare(cloneURL, barePath, branch); err != nil {
						os.RemoveAll(projectDir)
						return err
					}
					// Add main worktree
					mainPath = filepath.Join(projectDir, "main")
					if err := g.AddMainWorktree(barePath, mainPath); err != nil {
						os.RemoveAll(projectDir)
						return err
					}
					v = g
				} else {
					// Init mode: create bare repo with main worktree
					mainPath, err = g.InitBareProject(projectDir)
					if err != nil {
						os.RemoveAll(projectDir)
						return err
					}
					v = g
				}

			case "jj":
				j := &vcs.JJ{}
				if cloneURL != "" {
					fmt.Printf("クローン中: %s\n", cloneURL)
					if err := j.CloneBare(cloneURL, barePath, branch); err != nil {
						os.RemoveAll(projectDir)
						return err
					}
					mainPath = filepath.Join(projectDir, "main")
					if err := j.AddMainWorktree(barePath, mainPath); err != nil {
						os.RemoveAll(projectDir)
						return err
					}
					v = j
				} else {
					mainPath, err = j.InitBareProject(projectDir)
					if err != nil {
						os.RemoveAll(projectDir)
						return err
					}
					v = j
				}

			default:
				os.RemoveAll(projectDir)
				return fmt.Errorf("不明なVCSタイプ: %s", vcsType)
			}

			fmt.Printf("%s リポジトリを作成しました\n", v.Name())

			// Create dcell config in project root (not in main/)
			cfg := config.Default()
			cfg.VCS.Prefer = vcsType
			if err := cfg.SaveProject(projectDir); err != nil {
				fmt.Fprintf(os.Stderr, "警告: 設定ファイルの作成に失敗しました: %v\n", err)
			}

			// Output summary
			fmt.Printf("\nプロジェクト '%s' の準備ができました！\n", projectName)
			fmt.Printf("  Bareリポジトリ: %s\n", barePath)
			fmt.Printf("  Main worktree:  %s\n", mainPath)

			fmt.Printf("\n次のステップ:\n")
			fmt.Printf("  cd %s\n", mainPath)
			fmt.Printf("  dcell create feature-x\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&cloneURL, "clone", "", "クローンするリモートURL")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "クローンするブランチ（デフォルト: main/master）")
	cmd.Flags().StringVar(&vcsType, "vcs", "git", "VCSタイプ (git または jj)")

	return cmd
}
