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
.bare/ ディレクトリと .dcell/ 設定のみ作成します（worktreeは作成しません）。

作成される構造:
  my-project/
    .bare/          # bareリポジトリ
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

			// Use absolute paths to avoid issues
			absProjectDir, _ := filepath.Abs(projectDir)
			absBarePath := filepath.Join(absProjectDir, ".bare")

			switch vcsType {
			case "git":
				g := &vcs.Git{}
				if cloneURL != "" {
					// Clone mode: clone as bare only (no worktree)
					fmt.Printf("クローン中: %s\n", cloneURL)
					if err := g.CloneBare(cloneURL, absBarePath, branch); err != nil {
						os.RemoveAll(projectDir)
						return err
					}
				} else {
					// Init mode: create bare repo only (no worktree)
					if err := g.Init(absBarePath, true); err != nil {
						os.RemoveAll(projectDir)
						return err
					}
				}

			case "jj":
				j := &vcs.JJ{}
				if cloneURL != "" {
					// Clone mode: clone as bare only (no workspace)
					fmt.Printf("クローン中: %s\n", cloneURL)
					if err := j.CloneBare(cloneURL, absBarePath, branch); err != nil {
						os.RemoveAll(projectDir)
						return err
					}
				} else {
					// Init mode: create bare repo only (no workspace)
					if err := j.Init(absBarePath, true); err != nil {
						os.RemoveAll(projectDir)
						return err
					}
				}

			default:
				os.RemoveAll(projectDir)
				return fmt.Errorf("不明なVCSタイプ: %s", vcsType)
			}

			fmt.Printf("%s リポジトリを作成しました\n", vcsType)

			// Create .git file in project root to point to .bare
			gitFilePath := filepath.Join(absProjectDir, ".git")
			if err := os.WriteFile(gitFilePath, []byte("gitdir: ./.bare\n"), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "警告: .gitファイルの作成に失敗しました: %v\n", err)
			}

			// Create dcell config in project root
			cfg := config.Default()
			cfg.VCS.Prefer = vcsType
			if err := cfg.SaveProject(projectDir); err != nil {
				fmt.Fprintf(os.Stderr, "警告: 設定ファイルの作成に失敗しました: %v\n", err)
			}

			// Output summary
			fmt.Printf("\nプロジェクト '%s' の準備ができました！\n", projectName)
			fmt.Printf("  Bareリポジトリ: %s\n", absBarePath)

			fmt.Printf("\n次のステップ:\n")
			fmt.Printf("  cd %s\n", projectDir)
			fmt.Printf("  dcell create <context-name>\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&cloneURL, "clone", "", "クローンするリモートURL")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "クローンするブランチ（デフォルト: main/master）")
	cmd.Flags().StringVar(&vcsType, "vcs", "git", "VCSタイプ (git または jj)")

	return cmd
}
