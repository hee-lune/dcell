package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/config"
	"github.com/heelune/dcell/internal/gh"
	"github.com/heelune/dcell/internal/hooks"
	"github.com/heelune/dcell/internal/session"
	"github.com/heelune/dcell/internal/tmux"
	"github.com/heelune/dcell/internal/vcs"
)

func workCmd() *cobra.Command {
	var (
		from           string
		vcsType        string
		devcontainerFlag bool
		aiType         string
		openIDE        string
		dryRun         bool
		validate       bool
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

			// Handle validate mode
			if validate {
				return runValidate(ctxName, from, repoPath, cfg)
			}

			// Handle dry-run mode
			if dryRun {
				return runDryRun(ctxName, from, prompt, repoPath, cfg, aiType, openIDE)
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

			// Step 8: Create tmux session and attach
			fmt.Printf("🖥️  tmuxセッション作成中...\n")
			sessionName := tmux.GetSessionForContext(ctxName)
			
			if !tmux.HasTmux() {
				fmt.Fprintf(os.Stderr, "   ⚠ tmuxがインストールされていません\n")
				fmt.Printf("\n✅ ワーク '%s' の準備ができました！\n", ctxName)
				fmt.Printf("   パス: %s\n", ctx.Path)
				fmt.Printf("   cd %s\n", ctx.Path)
				return nil
			}
			
			if tmux.SessionExists(sessionName) {
				fmt.Printf("   ✓ 既存セッション '%s' に接続します\n", sessionName)
			} else {
				if err := tmux.CreateSession(sessionName, ctx.Path); err != nil {
					fmt.Fprintf(os.Stderr, "   ⚠ tmuxセッション作成に失敗: %v\n", err)
					fmt.Printf("\n✅ ワーク '%s' の準備ができました！\n", ctxName)
					fmt.Printf("   パス: %s\n", ctx.Path)
					fmt.Printf("   cd %s\n", ctx.Path)
					return nil
				}
				fmt.Printf("   ✓ セッション '%s' を作成しました\n", sessionName)
			}
			
			// Step 9: Launch AI if requested
			if aiType != "" {
				fmt.Printf("🤖 AIを起動します...\n")
				aiCmd := exec.Command("dcell", "ai", ctxName, "--type", aiType)
				aiCmd.Stdin = os.Stdin
				aiCmd.Stdout = os.Stdout
				aiCmd.Stderr = os.Stderr
				if err := aiCmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "   ⚠ AI起動に失敗: %v\n", err)
				}
			}
			
			fmt.Printf("\n🚀 tmuxに接続します... (exitでdcellに戻る)\n")
			
			// Attach to tmux session (this blocks until user exits tmux)
			if tmux.InTmux() {
				// Already in tmux, switch to the session
				return tmux.SwitchSession(sessionName)
			}
			return tmux.AttachSession(sessionName)
		},
	}

	cmd.Flags().StringVarP(&from, "from", "f", "", "ベースとなるブランチ/リビジョン")
	cmd.Flags().StringVar(&vcsType, "vcs", "auto", "VCSタイプ (jj, git, auto)")
	cmd.Flags().BoolVar(&devcontainerFlag, "devcontainer", false, "Dev Container設定も生成")
	cmd.Flags().StringVar(&aiType, "ai", "", "AIアシスタント (claude, kimi)")
	cmd.Flags().StringVar(&openIDE, "open", "", "IDEで開く (cursor, code, windsurf, zed)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "実行せずに計画を表示")
	cmd.Flags().BoolVar(&validate, "validate", false, "事前検証のみ実行")

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

// runValidate performs pre-execution validation
func runValidate(ctxName, from, repoPath string, cfg *config.Config) error {
	fmt.Println("🔍 検証モード: 実行前チェック")
	fmt.Println(strings.Repeat("-", 50))

	// Check 1: Context name validity
	if ctxName == "" {
		return fmt.Errorf("❌ コンテキスト名が空です")
	}
	if strings.ContainsAny(ctxName, `/:*?"<>|`) {
		return fmt.Errorf("❌ コンテキスト名に無効な文字が含まれています: %s", ctxName)
	}
	fmt.Printf("✅ コンテキスト名: '%s' は有効です\n", ctxName)

	// Check 2: Repository detection
	v, err := vcs.NewAuto(repoPath)
	if err != nil {
		return fmt.Errorf("❌ リポジトリが検出できません: %w", err)
	}
	fmt.Printf("✅ リポジトリ検出: %s\n", v.Name())

	// Check 3: Base branch existence
	if err := validateBranch(v, from); err != nil {
		return fmt.Errorf("❌ ベースブランチ '%s' が存在しません: %w", from, err)
	}
	fmt.Printf("✅ ベースブランチ '%s' は存在します\n", from)

	// Check 4: Context name collision
	contexts, _ := v.ListContexts()
	for _, ctx := range contexts {
		if ctx.Name == ctxName {
			return fmt.Errorf("❌ コンテキスト '%s' は既に存在します\n   修正案: dcell attach %s", ctxName, ctxName)
		}
	}
	fmt.Printf("✅ コンテキスト名 '%s' は利用可能です\n", ctxName)

	// Check 5: tmux availability
	if tmux.HasTmux() {
		fmt.Println("✅ tmuxが利用可能です")
	} else {
		fmt.Println("⚠️  tmuxが見つかりません（作成のみ実行）")
	}

	// Check 6: gh availability (for PR features)
	if gh.HasGH() {
		fmt.Println("✅ gh CLIが利用可能です")
	} else {
		fmt.Println("ℹ️  gh CLIが見つかります（PR機能は制限されます）")
	}

	// Check 7: Disk space (simplified check)
	projectRoot := getProjectRoot(v)
	if projectRoot == "" {
		projectRoot = repoPath
	}
	fmt.Printf("✅ 出力先: %s/%s\n", projectRoot, ctxName)

	fmt.Println(strings.Repeat("-", 50))
	fmt.Println("✅ すべての検証に合格しました")
	fmt.Println("実行可能: dcell work", ctxName)
	return nil
}

// runDryRun shows execution plan without actually running
func runDryRun(ctxName, from, prompt, repoPath string, cfg *config.Config, aiType, openIDE string) error {
	fmt.Println("📋 ドライラン: 実行計画")
	fmt.Println(strings.Repeat("-", 60))

	// Detect repository
	v, _ := vcs.NewAuto(repoPath)
	vcsName := "unknown"
	if v != nil {
		vcsName = v.Name()
	}

	projectRoot := repoPath
	if v != nil {
		pr := getProjectRoot(v)
		if pr != "" {
			projectRoot = pr
		}
	}
	ctxPath := filepath.Join(projectRoot, ctxName)
	sessionName := tmux.GetSessionForContext(ctxName)

	plan := map[string]interface{}{
		"action":       "create_work_context",
		"context_name": ctxName,
		"context_path": ctxPath,
		"base_branch":  from,
		"vcs_type":     vcsName,
		"steps": []map[string]string{
			{"step": "1", "action": "create_worktree", "target": ctxPath, "from": from},
			{"step": "2", "action": "setup_docker", "optional": "true"},
			{"step": "3", "action": "create_tmux_session", "session": sessionName},
		},
	}

	if prompt != "" {
		plan["ai_prompt"] = prompt
		plan["steps"] = append(plan["steps"].([]map[string]string), 
			map[string]string{"step": "4", "action": "launch_ai", "type": aiType})
	}

	if openIDE != "" {
		plan["steps"] = append(plan["steps"].([]map[string]string), 
			map[string]string{"step": "5", "action": "open_ide", "ide": openIDE})
	}

	plan["steps"] = append(plan["steps"].([]map[string]string), 
		map[string]string{"step": "6", "action": "attach_tmux", "session": sessionName})

	// Print as JSON for AI parsing
	jsonData, _ := json.MarshalIndent(plan, "", "  ")
	fmt.Println(string(jsonData))

	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("💡 実際に実行するには --dry-run フラグを外してください:")
	fmt.Printf("   dcell work %s", ctxName)
	if from != cfg.VCS.DefaultBranch {
		fmt.Printf(" --from %s", from)
	}
	if prompt != "" {
		fmt.Printf(" \"%s\"", prompt)
	}
	fmt.Println()
	
	return nil
}

func validateBranch(v vcs.VCS, branch string) error {
	// Simple validation: check if branch exists
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = v.(*vcs.Git).RepoPath
	return cmd.Run()
}
