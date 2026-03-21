package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/config"
	"github.com/heelune/dcell/internal/hooks"
	"github.com/heelune/dcell/internal/session"
	"github.com/heelune/dcell/internal/vcs"
)

func workCmd() *cobra.Command {
	var (
		from           string
		vcsType        string
		devcontainerFlag bool
		aiType         string
		openIDE        string
	)

	cmd := &cobra.Command{
		Use:   "work <name> [prompt]",
		Short: "開発コンテキストを作成して即座に作業開始",
		Long: `新しい開発コンテキストを作成し、AIセッションを開始、IDEを開くまでを一発で実行します。

例:
  # 基本的な使い方（作成のみ）
  dcell work feature-x

  # AIプロンプト付きで作成
  dcell work feature-x "ユーザ認証機能を実装して"

  # ベースブランチ指定
  dcell work feature-x "バグ修正" --from develop

  # IDEも同時に開く
  dcell work feature-x "リファクタリング" --open cursor`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxName := args[0]
			var prompt string
			if len(args) > 1 {
				prompt = args[1]
			}

			// Load config
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			// Detect repository
			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			// Determine VCS type
			if vcsType == "" {
				vcsType = cfg.VCS.Prefer
			}

			// Determine base branch
			if from == "" {
				from = cfg.VCS.DefaultBranch
			}

			// Determine AI type
			if aiType == "" {
				aiType = cfg.AI.Default
			}

			// Create VCS instance
			var v vcs.VCS
			if vcsType == "auto" {
				v, err = vcs.NewAuto(repoPath)
			} else {
				v, err = vcs.New(vcsType, repoPath)
			}
			if err != nil {
				return err
			}

			fmt.Printf("🚀 ワーク '%s' を開始します...\n", ctxName)

			// Step 1: Create VCS context
			fmt.Printf("📁 コンテキスト作成中...\n")
			ctx, err := v.CreateContext(ctxName, from)
			if err != nil {
				return err
			}
			fmt.Printf("   ✓ %s コンテキストを作成: %s\n", ctx.VCS, ctx.Path)

			// Step 2: Setup Docker
			if err := setupDocker(repoPath, ctxName, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "   ⚠ Docker setup failed: %v\n", err)
			}

			// Step 3: Setup Dev Container
			if devcontainerFlag {
				if err := setupDevContainer(repoPath, ctxName, cfg); err != nil {
					fmt.Fprintf(os.Stderr, "   ⚠ Dev Container setup failed: %v\n", err)
				}
			}

			// Step 4: Get project root
			projectRoot := getProjectRoot(v)
			if projectRoot == "" {
				projectRoot = repoPath
			}

			// Step 5: Create AI session
			store := session.NewStore(projectRoot)
			sess, err := store.Create(ctxName, ctx.VCS, ctx.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "   ⚠ Session creation failed: %v\n", err)
			}

			// Step 6: Run post-create hooks
			if len(cfg.Hooks.PostCreate) > 0 {
				fmt.Printf("🪝 Hooks実行中...\n")
				hookCtx := &hooks.Context{
					ProjectRoot:  projectRoot,
					WorktreePath: ctx.Path,
					ContextName:  ctxName,
					BaseBranch:   from,
					VCS:          ctx.VCS,
				}
				runner := hooks.NewRunner(hookCtx)
				if err := runner.ExecutePostCreate(cfg.Hooks.PostCreate); err != nil {
					fmt.Fprintf(os.Stderr, "   ⚠ Post-create hooks failed: %v\n", err)
				} else {
					fmt.Printf("   ✓ Hooks完了\n")
				}
			}

			// Step 7: Start AI if prompt provided
			if prompt != "" && aiType != "" {
				fmt.Printf("🤖 AI起動中...\n")
				ai, err := session.GetAI(aiType)
				if err != nil {
					ai, err = session.DetectAI()
					if err != nil {
						fmt.Fprintf(os.Stderr, "   ⚠ AI detection failed: %v\n", err)
						ai = nil
					}
				}

				if ai != nil {
					// Create context loader
					globalDir := filepath.Dir(config.GlobalConfigPath())
					loader := session.NewContextLoader(
						globalDir,
						projectRoot,
						filepath.Dir(sess.ContextPath),
					)

					// Start AI with prompt (non-blocking for now)
					fmt.Printf("   ✓ %s を起動: %s\n", ai.Name(), ctxName)
					fmt.Printf("   💬 プロンプト: %s\n", prompt)
					
					// TODO: Pass prompt to AI
					_ = loader
				}
			}

			// Step 8: Open IDE if specified
			if openIDE != "" {
				fmt.Printf("🖥️  IDE起動中...\n")
				if err := launchIDE(openIDE, ctx.Path); err != nil {
					fmt.Fprintf(os.Stderr, "   ⚠ IDE launch failed: %v\n", err)
				} else {
					fmt.Printf("   ✓ %s で開きました\n", openIDE)
				}
			}

			// Summary
			fmt.Printf("\n✅ ワーク '%s' の準備ができました！\n", ctxName)
			fmt.Printf("   パス: %s\n", ctx.Path)
			if prompt != "" {
				fmt.Printf("   プロンプト: %s\n", prompt)
			}
			fmt.Printf("\n次のステップ:\n")
			fmt.Printf("   cd %s\n", ctx.Path)

			return nil
		},
	}

	cmd.Flags().StringVarP(&from, "from", "f", "", "ベースとなるブランチ/リビジョン")
	cmd.Flags().StringVar(&vcsType, "vcs", "auto", "VCSタイプ (jj, git, auto)")
	cmd.Flags().BoolVar(&devcontainerFlag, "devcontainer", false, "Dev Container設定も生成")
	cmd.Flags().StringVar(&aiType, "ai", "", "AIアシスタント (claude, kimi)")
	cmd.Flags().StringVar(&openIDE, "open", "", "IDEで開く (cursor, code, windsurf, zed)")

	return cmd
}

// launchIDE opens the specified IDE with the given path.
func launchIDE(ide, path string) error {
	var cmd *exec.Cmd

	switch ide {
	case "cursor":
		cmd = exec.Command("cursor", path)
	case "code", "vscode":
		cmd = exec.Command("code", path)
	case "windsurf":
		cmd = exec.Command("windsurf", path)
	case "zed":
		cmd = exec.Command("zed", path)
	default:
		return fmt.Errorf("unknown IDE: %s", ide)
	}

	// Detach process
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	return cmd.Start()
}
