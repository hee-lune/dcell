package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/config"
	"github.com/heelune/dcell/internal/devcontainer"
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
	rootCmd.AddCommand(migrateCmd())
	rootCmd.AddCommand(createCmd())
	rootCmd.AddCommand(switchCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(removeCmd())
	rootCmd.AddCommand(aiCmd())
	rootCmd.AddCommand(contextCmd())
	rootCmd.AddCommand(devcontainerCmd())
	rootCmd.AddCommand(snapshotCmd())
	rootCmd.AddCommand(composeCmd())
}

func createCmd() *cobra.Command {
	var (
		from        string
		vcsType     string
		devcontainerFlag bool
	)

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

			// Determine base branch
			if from == "" {
				from = cfg.VCS.DefaultBranch
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

			// Setup Dev Container if requested
			if devcontainerFlag {
				if err := setupDevContainer(repoPath, ctxName, cfg); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Dev Container setup failed: %v\n", err)
				}
			}

			// Create AI session
			projectRoot := getProjectRoot(v)
			if projectRoot == "" {
				projectRoot = repoPath
			}
			store := session.NewStore(projectRoot)
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
	cmd.Flags().BoolVar(&devcontainerFlag, "devcontainer", false, "Dev Container設定も生成")

	return cmd
}

func switchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <name>",
		Short: "開発コンテキストに切り替え",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxName := args[0]

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
			projectRoot := getProjectRoot(v)
			if projectRoot == "" {
				projectRoot = repoPath
			}
			store := session.NewStore(projectRoot)
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
	var force bool

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "開発コンテキストを削除",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxName := args[0]

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

			// Cleanup Dev Container
			projectName := filepath.Base(repoPath)
			dcGenerator := devcontainer.NewGenerator(projectName, repoPath)
			dcGenerator.Cleanup(ctxName)

			// Remove VCS context
			if err := v.RemoveContext(ctxName, force); err != nil {
				return err
			}

			// Remove session
			projectRoot := getProjectRoot(v)
			if projectRoot == "" {
				projectRoot = repoPath
			}
			store := session.NewStore(projectRoot)
			store.Remove(ctxName)

			fmt.Printf("コンテキスト '%s' を削除しました。\n", ctxName)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "強制削除（未コミットの変更も削除）")

	return cmd
}

func aiCmd() *cobra.Command {
	var aiType string

	cmd := &cobra.Command{
		Use:   "ai [context-name]",
		Short: "AIアシスタントを起動",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			// Determine context name and detect repository first
			var ctxName string
			var v vcs.VCS
			if len(args) > 0 {
				ctxName = args[0]
				// Context specified, but still try to detect repo for project root
				v, _ = vcs.NewAuto(repoPath)
			} else {
				// Try to get current context from directory
				var err error
				v, err = vcs.NewAuto(repoPath)
				if err != nil {
					return fmt.Errorf("no context specified and not in a dcell: %w", err)
				}
				current, err := v.CurrentContext()
				if err != nil {
					return fmt.Errorf("no context specified and not in a dcell: %w", err)
				}
				ctxName = current.Name
			}

			// Get project root for config loading
			projectRoot := getProjectRoot(v)
			if projectRoot == "" {
				projectRoot = repoPath
			}

			// Load config from project root (works from any worktree)
			cfg, err := loadConfigForPath(projectRoot)
			if err != nil {
				return err
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
			
			// Get context path (flat structure: directly under project root)
			ctxPath := filepath.Join(projectRoot, ctxName)
			
			// Load or create session
			store := session.NewStore(projectRoot)
			sess, err := store.Load(ctxName)
			if err != nil {
				// Session doesn't exist, create it
				fmt.Printf("セッション '%s' を新規作成します...\n", ctxName)
				sess, err = store.Create(ctxName, v.Name(), ctxPath)
				if err != nil {
					return fmt.Errorf("failed to create session: %w", err)
				}
			}
			
			// Create context loader for layered context
			globalDir := filepath.Dir(config.GlobalConfigPath())
			loader := session.NewContextLoader(
				globalDir,                  // Global: ~/.config/dcell/
				projectRoot,                // Project: dcell/
				filepath.Dir(sess.ContextPath), // Session: .dcell-session/
			)
			
			fmt.Printf("%s をコンテキスト '%s' で起動中...\n", ai.Name(), ctxName)
			
			return ai.Start(ctxPath, sess, loader)
		},
	}

	cmd.Flags().StringVar(&aiType, "type", "", "AIタイプ (claude または kimi)")

	return cmd
}

// getProjectRoot returns the project root path (parent of .bare directory)
func getProjectRoot(v vcs.VCS) string {
	switch typedV := v.(type) {
	case *vcs.Git:
		return filepath.Dir(typedV.RepoPath)
	case *vcs.JJ:
		return filepath.Dir(typedV.RepoPath)
	}
	return ""
}

func loadConfig() (*config.Config, error) {
	return loadConfigForPath(".")
}

func loadConfigForPath(projectPath string) (*config.Config, error) {
	cfg := config.Default()

	// Load global config
	if globalCfg, err := config.LoadGlobal(); err == nil {
		cfg.Merge(globalCfg)
	}

	// Load project config if available
	if projectCfg, err := config.LoadProject(projectPath); err == nil {
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

func setupDevContainer(repoPath string, ctxName string, cfg *config.Config) error {
	// Load port state
	portState, err := docker.LoadPortState(repoPath)
	if err != nil {
		return err
	}

	// Get port index for context
	idx := portState.GetIndex(ctxName)
	if idx < 0 {
		// Allocate new if not exists
		idx = portState.Allocate(ctxName)
		if err := portState.Save(repoPath); err != nil {
			return err
		}
	}

	// Get ports
	pm := docker.NewPortManager(cfg.Docker.PortBase, cfg.Docker.PortStep)
	ports := pm.GetPorts(idx)

	// Generate devcontainer config
	projectName := filepath.Base(repoPath)
	generator := devcontainer.NewGenerator(projectName, repoPath)
	if err := generator.GenerateForWorktree(ctxName, "app", ports); err != nil {
		return err
	}

	fmt.Printf("  Dev Container設定を作成しました: %s/../%s/.devcontainer/devcontainer.json\n", repoPath, ctxName)
	return nil
}

func composeCmd() *cobra.Command {
	var ctxName string

	cmd := &cobra.Command{
		Use:                "compose [flags] [docker-compose-args...]",
		Short:              "docker compose のラッパー（dcell設定自動適用）",
		DisableFlagParsing: false,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			// Detect VCS
			v, err := vcs.NewAuto(repoPath)
			if err != nil {
				return fmt.Errorf("failed to detect repository: %w", err)
			}

			// Get project root
			projectRoot := getProjectRoot(v)
			if projectRoot == "" {
				projectRoot = repoPath
			}

			// Determine context name and path
			var ctxPath string
			if ctxName != "" {
				// Context specified explicitly
				ctxPath = filepath.Join(projectRoot, ctxName)
			} else {
				// Try to get current context from directory
				current, err := v.CurrentContext()
				if err != nil {
					return fmt.Errorf("no context specified and not in a dcell context: %w", err)
				}
				ctxName = current.Name
				ctxPath = current.Path
			}

			// Check if context exists
			if _, err := os.Stat(ctxPath); os.IsNotExist(err) {
				return fmt.Errorf("context '%s' not found at %s", ctxName, ctxPath)
			}

			// Build docker compose command with dcell override
			dockerArgs := []string{"compose", "-f", "docker-compose.yml", "-f", "docker-compose.dcell.yml"}
			dockerArgs = append(dockerArgs, args...)

			execCmd := exec.Command("docker", dockerArgs...)
			execCmd.Dir = ctxPath
			execCmd.Stdin = os.Stdin
			execCmd.Stdout = os.Stdout
			execCmd.Stderr = os.Stderr

			return execCmd.Run()
		},
	}

	cmd.Flags().StringVarP(&ctxName, "context", "c", "", "コンテキスト名（指定しない場合はカレントディレクトリから推定）")

	return cmd
}
