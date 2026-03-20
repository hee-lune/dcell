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
		bare     bool
		vcsType  string
	)

	cmd := &cobra.Command{
		Use:   "init <directory>",
		Short: "新規プロジェクトを初期化",
		Long: `新規プロジェクトを初期化します：
- 空のリポジトリを作成 (--bare オプション対応)
- 既存リポジトリをクローン
- dcell設定ファイルを自動生成

例:
  # 新規ローカルリポジトリを作成
  dcell init my-project

  # 既存リポジトリをクローン
  dcell init my-project --clone https://github.com/user/repo.git

  # bareリポジトリとして作成
  dcell init my-project --bare`,
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

			var v vcs.VCS
			var mainPath string
			var err error

			switch vcsType {
			case "git":
				g := &vcs.Git{}
				if cloneURL != "" {
					// Clone mode
					fmt.Printf("クローン中: %s\n", cloneURL)
					if err := g.Clone(cloneURL, projectDir, branch); err != nil {
						return err
					}
					mainPath = projectDir
					v = g
				} else {
					// Init mode
					mainPath, err = g.InitAndSetup(projectDir, bare)
					if err != nil {
						return err
					}
					v = g
				}

			case "jj":
				j := &vcs.JJ{}
				if cloneURL != "" {
					// Clone mode
					fmt.Printf("クローン中: %s\n", cloneURL)
					if err := j.Clone(cloneURL, projectDir, branch); err != nil {
						return err
					}
					mainPath = projectDir
					v = j
				} else {
					// Init mode
					mainPath, err = j.InitAndSetup(projectDir, bare)
					if err != nil {
						return err
					}
					v = j
				}

			default:
				return fmt.Errorf("不明なVCSタイプ: %s", vcsType)
			}

			fmt.Printf("%s リポジトリを作成しました\n", v.Name())

			// Create dcell config
			cfg := config.Default()
			cfg.VCS.Prefer = vcsType

			if bare {
				// For bare repos, config goes in the main worktree
				if err := cfg.SaveProject(mainPath); err != nil {
					fmt.Fprintf(os.Stderr, "警告: 設定ファイルの作成に失敗しました: %v\n", err)
				}
			} else {
				// For non-bare, config goes in the project root
				if err := cfg.SaveProject(projectDir); err != nil {
					fmt.Fprintf(os.Stderr, "警告: 設定ファイルの作成に失敗しました: %v\n", err)
				}
			}

			// Output summary
			fmt.Printf("\nプロジェクト '%s' の準備ができました！\n", projectName)

			if bare {
				barePath := projectDir + ".git"
				fmt.Printf("  Bareリポジトリ: %s\n", barePath)
				fmt.Printf("  Main worktree:  %s\n", mainPath)
			} else {
				fmt.Printf("  パス: %s\n", mainPath)
			}

			fmt.Printf("\n次のステップ:\n")
			fmt.Printf("  cd %s\n", mainPath)
			if !bare {
				fmt.Printf("  dcell create feature-x\n")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&cloneURL, "clone", "", "クローンするリモートURL")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "クローンするブランチ（デフォルト: main/master）")
	cmd.Flags().BoolVar(&bare, "bare", false, "Bareリポジトリとして作成")
	cmd.Flags().StringVar(&vcsType, "vcs", "git", "VCSタイプ (git または jj)")

	return cmd
}
