package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/config"
	"github.com/heelune/dcell/internal/session"
	"github.com/heelune/dcell/internal/vcs"
)

func contextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "コンテキスト管理",
		Long: `階層型コンテキストを管理します。

3階層のコンテキスト:
  1. Global     (~/.config/dcell/context.md) - 共通設定
  2. Project    (dcell/.context.md)          - プロジェクト固有
  3. Session    (.dcell-session/context.md)  - 作業単位

これらのファイルが自動的にAIに渡されます。`,
	}

	cmd.AddCommand(contextInitCmd())
	cmd.AddCommand(contextEditCmd())
	cmd.AddCommand(contextShowCmd())

	return cmd
}

func contextInitCmd() *cobra.Command {
	var global, project bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "コンテキストファイルを初期化",
		Long: `コンテキストファイルを作成します。

例:
  # グローバルコンテキストを作成
  dcell context init --global

  # プロジェクトコンテキストを作成
  dcell context init --project

  # 両方作成
  dcell context init --global --project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !global && !project {
				return fmt.Errorf("--global または --project を指定してください")
			}

			if global {
				globalDir := filepath.Dir(config.GlobalConfigPath())
				if err := session.InitGlobalContext(globalDir); err != nil {
					return fmt.Errorf("グローバルコンテキストの作成に失敗: %w", err)
				}
				fmt.Printf("✅ グローバルコンテキストを作成しました: %s\n", globalDir)
			}

			if project {
				// Find project root (.bare parent)
				repoPath, err := os.Getwd()
				if err != nil {
					return err
				}

				v, err := vcs.NewAuto(repoPath)
				if err != nil {
					return fmt.Errorf("リポジトリが見つかりません: %w", err)
				}

				var barePath string
				switch typedV := v.(type) {
				case *vcs.Git:
					barePath = typedV.RepoPath
				case *vcs.JJ:
					barePath = typedV.RepoPath
				}

				if barePath == "" {
					return fmt.Errorf("リポジトリパスを特定できません")
				}

				projectRoot := filepath.Dir(barePath)
				if err := session.InitProjectContext(projectRoot); err != nil {
					return fmt.Errorf("プロジェクトコンテキストの作成に失敗: %w", err)
				}
				fmt.Printf("✅ プロジェクトコンテキストを作成しました: %s\n", projectRoot)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "グローバルコンテキストを作成")
	cmd.Flags().BoolVar(&project, "project", false, "プロジェクトコンテキストを作成")

	return cmd
}

func contextEditCmd() *cobra.Command {
	var global, project, session bool

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "コンテキストファイルを編集",
		RunE: func(cmd *cobra.Command, args []string) error {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "code" // fallback to VSCode
			}

			var targetFile string

			switch {
			case global:
				globalDir := filepath.Dir(config.GlobalConfigPath())
				targetFile = filepath.Join(globalDir, "context.md")

			case project:
				repoPath, err := os.Getwd()
				if err != nil {
					return err
				}
				v, err := vcs.NewAuto(repoPath)
				if err != nil {
					return err
				}
				var barePath string
				switch typedV := v.(type) {
				case *vcs.Git:
					barePath = typedV.RepoPath
				case *vcs.JJ:
					barePath = typedV.RepoPath
				}
				projectRoot := filepath.Dir(barePath)
				targetFile = filepath.Join(projectRoot, ".context.md")

			case session:
				repoPath, err := os.Getwd()
				if err != nil {
					return err
				}
				v, err := vcs.NewAuto(repoPath)
				if err != nil {
					return err
				}
				current, err := v.CurrentContext()
				if err != nil {
					return err
				}
				targetFile = filepath.Join(current.Path, ".dcell-session", "context.md")

			default:
				return fmt.Errorf("--global, --project, --session のいずれかを指定してください")
			}

			execCmd := exec.Command(editor, targetFile)
			execCmd.Stdin = os.Stdin
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr
			return execCmd.Run()
		},
	}

	cmd.Flags().BoolVar(&global, "global", false, "グローバルコンテキストを編集")
	cmd.Flags().BoolVar(&project, "project", false, "プロジェクトコンテキストを編集")
	cmd.Flags().BoolVar(&session, "session", false, "セッションコンテキストを編集")

	return cmd
}

func contextShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "現在のコンテキストを表示",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			v, err := vcs.NewAuto(repoPath)
			if err != nil {
				return err
			}

			var barePath string
			switch typedV := v.(type) {
			case *vcs.Git:
				barePath = typedV.RepoPath
			case *vcs.JJ:
				barePath = typedV.RepoPath
			}

			projectRoot := filepath.Dir(barePath)
			globalDir := filepath.Dir(config.GlobalConfigPath())

			loader := session.NewContextLoader(globalDir, projectRoot, "")
			content, err := loader.LoadContext()
			if err != nil {
				return err
			}

			fmt.Println(content)
			return nil
		},
	}

	return cmd
}
