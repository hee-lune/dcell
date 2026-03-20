package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/config"
	"github.com/heelune/dcell/internal/docker"
	"github.com/heelune/dcell/internal/session"
	"github.com/heelune/dcell/internal/vcs"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "dcell",
		Short: "開発コンテキスト管理ツール",
		Long: `dcell は開発コンテキスト（Development Cell）を管理するツールです：
- Git/JJ worktree の管理
- Docker環境のポート自動割り当て
- AIアシスタントとのセッション管理`,
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "設定ファイル（デフォルト: $HOME/.config/dcell/config.toml）")

	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(createCmd())
	rootCmd.AddCommand(switchCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(removeCmd())
	rootCmd.AddCommand(aiCmd())
}

func createCmd() *cobra.Command {
	var from string
	var vcsType string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "新しい開発コンテキストを作成",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxName := args[0]
			
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

			fmt.Printf("コンテキスト '%s' を %s で作成中...\n", ctxName, v.Name())

			// Create VCS context
			ctx, err := v.CreateContext(ctxName, from)
			if err != nil {
				return err
			}

			fmt.Printf("%s コンテキストを作成しました: %s\n", ctx.VCS, ctx.Path)

			// Setup Docker environment
			if err := setupDocker(repoPath, ctxName, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Docker setup failed: %v\n", err)
			}

			// Create AI session
			store := session.NewStore(cfg.AI.SessionDir)
			if _, err := store.Create(ctxName, ctx.VCS, ctx.Path); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Session creation failed: %v\n", err)
			}

			fmt.Printf("コンテキスト '%s' の準備ができました！\n", ctxName)
			fmt.Printf("  パス: %s\n", ctx.Path)
			fmt.Printf("  切り替え: dcell switch %s\n", ctxName)

			return nil
		},
	}

	cmd.Flags().StringVarP(&from, "from", "f", "", "ベースとなるブランチ/リビジョン")
	cmd.Flags().StringVar(&vcsType, "vcs", "auto", "VCSタイプ (jj, git, auto)")

	return cmd
}

func switchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <name>",
		Short: "開発コンテキストに切り替え",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxName := args[0]

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			// Load VCS
			v, err := vcs.NewAuto(repoPath)
			if err != nil {
				return err
			}

			fmt.Printf("コンテキスト '%s' に切り替え中...\n", ctxName)

			if err := v.SwitchContext(ctxName); err != nil {
				return err
			}

			// Update session
			store := session.NewStore(cfg.AI.SessionDir)
			if err := store.Update(ctxName); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Session update failed: %v\n", err)
			}

			ctxPath := filepath.Join(repoPath, "..", ctxName)
			fmt.Printf("切り替え完了: %s\n", ctxPath)
			fmt.Println("以下のコマンドでディレクトリを変更してください：")
			fmt.Printf("  cd %s\n", ctxPath)

			return nil
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "開発コンテキストの一覧表示",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			v, err := vcs.NewAuto(repoPath)
			if err != nil {
				return err
			}

			contexts, err := v.ListContexts()
			if err != nil {
				return err
			}

			current, _ := v.CurrentContext()

			fmt.Printf("開発コンテキスト一覧 (%s):\n\n", v.Name())
			
			for _, ctx := range contexts {
				prefix := "  "
				if current != nil && ctx.Name == current.Name {
					prefix = "* "
				}
				
				fmt.Printf("%s%s", prefix, ctx.Name)
				if ctx.BaseBranch != "" {
					fmt.Printf(" (from %s)", ctx.BaseBranch)
				}
				fmt.Printf("\n  %s\n\n", ctx.Path)
			}

			return nil
		},
	}
}

func removeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "開発コンテキストを削除",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxName := args[0]

			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			v, err := vcs.NewAuto(repoPath)
			if err != nil {
				return err
			}

			fmt.Printf("コンテキスト '%s' を削除中...\n", ctxName)

			// Cleanup Docker
			composeMgr := docker.NewComposeManager(repoPath)
			composeMgr.Cleanup(ctxName)

			// Remove VCS context
			if err := v.RemoveContext(ctxName); err != nil {
				return err
			}

			// Remove session
			store := session.NewStore(cfg.AI.SessionDir)
			store.Remove(ctxName)

			fmt.Printf("コンテキスト '%s' を削除しました。\n", ctxName)

			return nil
		},
	}
}

func aiCmd() *cobra.Command {
	var aiType string

	cmd := &cobra.Command{
		Use:   "ai [context-name]",
		Short: "AIアシスタントを起動",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			// Determine context name
			var ctxName string
			if len(args) > 0 {
				ctxName = args[0]
			} else {
				// Try to get current context from directory
				v, err := vcs.NewAuto(repoPath)
				if err != nil {
					return fmt.Errorf("no context specified and not in a dcell: %w", err)
				}
				current, err := v.CurrentContext()
				if err != nil {
					return fmt.Errorf("no context specified and not in a dcell: %w", err)
				}
				ctxName = current.Name
			}

			// Get AI
			if aiType == "" {
				aiType = cfg.AI.Default
			}
			
			ai, err := session.GetAI(aiType)
			if err != nil {
				ai, err = session.DetectAI()
				if err != nil {
					return err
				}
			}

			// Load session
			store := session.NewStore(cfg.AI.SessionDir)
			sess, err := store.Load(ctxName)
			if err != nil {
				return err
			}

			ctxPath := filepath.Join(repoPath, "..", ctxName)
			
			fmt.Printf("%s をコンテキスト '%s' で起動中...\n", ai.Name(), ctxName)
			
			return ai.Start(ctxPath, sess)
		},
	}

	cmd.Flags().StringVar(&aiType, "type", "", "AIタイプ (claude または kimi)")

	return cmd
}

func loadConfig() (*config.Config, error) {
	cfg := config.Default()

	// Load global config
	if globalCfg, err := config.LoadGlobal(); err == nil {
		cfg.Merge(globalCfg)
	}

	// Load project config if available
	if projectCfg, err := config.LoadProject("."); err == nil {
		cfg.Merge(projectCfg)
	}

	return cfg, nil
}

func setupDocker(repoPath string, ctxName string, cfg *config.Config) error {
	// Load port state
	portState, err := docker.LoadPortState(repoPath)
	if err != nil {
		return err
	}

	// Allocate port index
	idx := portState.Allocate(ctxName)
	if err := portState.Save(repoPath); err != nil {
		return err
	}

	// Get ports
	pm := docker.NewPortManager(cfg.Docker.PortBase, cfg.Docker.PortStep)
	ports := pm.GetPorts(idx)

	// Generate docker-compose override
	composeMgr := docker.NewComposeManager(repoPath)
	if err := composeMgr.GenerateOverride(ctxName, ports); err != nil {
		return err
	}

	// Generate env file
	if err := composeMgr.GenerateEnv(ctxName, ports); err != nil {
		return err
	}

	return nil
}
