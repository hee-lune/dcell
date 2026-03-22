package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/heelune/dcell/internal/config"
	"github.com/heelune/dcell/internal/devcontainer"
	"github.com/heelune/dcell/internal/docker"
	"github.com/heelune/dcell/internal/gh"
	"github.com/heelune/dcell/internal/hooks"
	"github.com/heelune/dcell/internal/session"
	"github.com/heelune/dcell/internal/tmux"
	"github.com/heelune/dcell/internal/vcs"
)

// JSONHelp represents command help information in JSON format
type JSONHelp struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Usage       string     `json:"usage"`
	Args        []ArgInfo  `json:"args"`
	Flags       []FlagInfo `json:"flags"`
	Subcommands []JSONHelp `json:"subcommands"`
}

// ArgInfo represents argument information
type ArgInfo struct {
	Name        string `json:"name"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// FlagInfo represents flag information
type FlagInfo struct {
	Name        string `json:"name"`
	Short       string `json:"short"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Description string `json:"description"`
}

var (
	cfgFile    string
	jsonOutput bool
	rootCmd    = &cobra.Command{
		Use:   "dcell",
		Short: "開発コンテキスト管理ツール",
		Long: `dcell は開発コンテキスト（Development Cell）を管理するツールです：
- Git/JJ worktree の管理
- Docker環境のポート自動割り当て
- AIアシスタントとのセッション管理`,
	}
)

// outputJSONHelp outputs command help information in JSON format
func outputJSONHelp(cmd *cobra.Command) {
	help := buildJSONHelp(cmd)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.Encode(help)
}

// buildJSONHelp recursively builds JSON help structure
func buildJSONHelp(cmd *cobra.Command) JSONHelp {
	help := JSONHelp{
		Name:        cmd.Name(),
		Description: cmd.Short,
		Usage:       cmd.UseLine(),
		Args:        parseArgs(cmd.Use),
		Flags:       []FlagInfo{},
		Subcommands: []JSONHelp{},
	}

	// Collect flags
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Skip json and help flags
		if f.Name == "json" || f.Name == "help" {
			return
		}
		flagInfo := FlagInfo{
			Name:        f.Name,
			Description: f.Usage,
		}
		if f.Shorthand != "" {
			flagInfo.Short = f.Shorthand
		}
		flagInfo.Type = getFlagType(f)
		if f.DefValue != "" {
			flagInfo.Default = f.DefValue
		}
		help.Flags = append(help.Flags, flagInfo)
	})

	// Collect persistent flags from parent if this is not root
	if cmd.Parent() != nil {
		cmd.Parent().PersistentFlags().VisitAll(func(f *pflag.Flag) {
			// Skip json and help flags
			if f.Name == "json" || f.Name == "help" {
				return
			}
			// Check if already added
			found := false
			for _, existing := range help.Flags {
				if existing.Name == f.Name {
					found = true
					break
				}
			}
			if !found {
				flagInfo := FlagInfo{
					Name:        f.Name,
					Description: f.Usage,
				}
				if f.Shorthand != "" {
					flagInfo.Short = f.Shorthand
				}
				flagInfo.Type = getFlagType(f)
				if f.DefValue != "" {
					flagInfo.Default = f.DefValue
				}
				help.Flags = append(help.Flags, flagInfo)
			}
		})
	}

	// Collect subcommands
	for _, sub := range cmd.Commands() {
		if !sub.Hidden {
			help.Subcommands = append(help.Subcommands, buildJSONHelp(sub))
		}
	}

	return help
}

// getFlagType determines the type of a flag
func getFlagType(f *pflag.Flag) string {
	// Try to determine type from value type
	if f.Value != nil {
		typeStr := fmt.Sprintf("%T", f.Value)
		switch {
		case strings.Contains(typeStr, "bool"):
			return "bool"
		case strings.Contains(typeStr, "int") && !strings.Contains(typeStr, "string"):
			return "int"
		case strings.Contains(typeStr, "stringArray") || strings.Contains(typeStr, "stringSlice"):
			return "array"
		default:
			return "string"
		}
	}
	return "string"
}

// parseArgs parses arguments from Use string
func parseArgs(use string) []ArgInfo {
	args := []ArgInfo{}

	// Extract argument parts from Use string
	// Format: "command <arg1> [arg2]"
	parts := strings.Fields(use)
	if len(parts) <= 1 {
		return args
	}

	for i, part := range parts[1:] {
		if !strings.HasPrefix(part, "<") && !strings.HasPrefix(part, "[") {
			continue
		}

		// Clean up brackets
		name := part
		required := strings.HasPrefix(part, "<")

		name = strings.TrimPrefix(name, "<")
		name = strings.TrimPrefix(name, "[")
		name = strings.TrimSuffix(name, ">")
		name = strings.TrimSuffix(name, "]")
		name = strings.TrimSuffix(name, "...")

		// Extract just the argument name (remove type hints like :string)
		if idx := strings.Index(name, ":"); idx != -1 {
			name = name[:idx]
		}

		arg := ArgInfo{
			Name:     name,
			Required: required,
		}

		// Add description based on common argument names
		switch name {
		case "name":
			arg.Description = "コンテキスト名"
		case "prompt":
			arg.Description = "AIプロンプト"
		case "directory":
			arg.Description = "プロジェクトディレクトリ名"
		case "number":
			arg.Description = "PR番号"
		default:
			if i == 0 {
				arg.Description = "引数"
			} else {
				arg.Description = "オプション引数"
			}
		}

		args = append(args, arg)
	}

	return args
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "設定ファイル（デフォルト: $HOME/.config/dcell/config.toml）")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "ヘルプ情報をJSON形式で出力")

	// Set custom help function that checks for --json flag
	rootCmd.SetHelpFunc(customHelpFunc)

	addAllCommands()
}

// customHelpFunc outputs help in JSON format if --json flag is set
func customHelpFunc(cmd *cobra.Command, args []string) {
	// Check if --json flag is set
	if jsonOutput {
		outputJSONHelp(cmd)
		return
	}

	// Default help output
	cmd.Usage()
}

func addAllCommands() {
	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(migrateCmd())
	rootCmd.AddCommand(createCmd())
	rootCmd.AddCommand(workCmd())
	rootCmd.AddCommand(attachCmd())
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(removeCmd())
	rootCmd.AddCommand(aiCmd())
	rootCmd.AddCommand(prCmd())
	rootCmd.AddCommand(submitCmd())
	rootCmd.AddCommand(contextCmd())
	rootCmd.AddCommand(devcontainerCmd())
	rootCmd.AddCommand(snapshotCmd())
	rootCmd.AddCommand(composeCmd())
}

func createCmd() *cobra.Command {
	var (
		from             string
		vcsType          string
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

			// Run post-create hooks
			if len(cfg.Hooks.PostCreate) > 0 {
				hookCtx := &hooks.Context{
					ProjectRoot:  projectRoot,
					WorktreePath: ctx.Path,
					ContextName:  ctxName,
					BaseBranch:   from,
					VCS:          ctx.VCS,
				}
				runner := hooks.NewRunner(hookCtx)
				if err := runner.ExecutePostCreate(cfg.Hooks.PostCreate); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Post-create hooks failed: %v\n", err)
				}
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

			// Get tmux sessions
			tmuxSessions := make(map[string]tmux.Session)
			if tmux.HasTmux() {
				sessions, _ := tmux.ListSessions()
				for _, s := range sessions {
					if strings.HasPrefix(s.Name, "dcell-") {
						ctxName := strings.TrimPrefix(s.Name, "dcell-")
						tmuxSessions[ctxName] = s
					}
				}
			}

			// Get PR info if gh is available
			var prs map[string]*gh.PR
			if gh.HasGH() {
				client, err := gh.New()
				if err == nil {
					prList, _ := client.ListPRs("open")
					prs = make(map[string]*gh.PR)
					for i := range prList {
						prs[prList[i].HeadRef] = &prList[i]
					}
				}
			}

			fmt.Printf("開発コンテキスト一覧 (%s):\n\n", v.Name())
			fmt.Printf("%-18s %-10s %-8s %-30s\n", "CONTEXT", "TMUX", "PR", "TITLE")
			fmt.Println(strings.Repeat("-", 70))

			for _, ctx := range contexts {
				prefix := "  "
				if current != nil && ctx.Name == current.Name {
					prefix = "* "
				}

				session, hasTmux := tmuxSessions[ctx.Name]
				var tmuxStatus string
				if hasTmux {
					if session.Attached {
						tmuxStatus = "attached"
					} else {
						tmuxStatus = "detached"
					}
				} else {
					tmuxStatus = "-"
				}

				var prNum, prTitle string
				if pr, ok := prs[ctx.Name]; ok {
					prNum = fmt.Sprintf("#%d", pr.Number)
					prTitle = pr.Title
					if len(prTitle) > 28 {
						prTitle = prTitle[:25] + "..."
					}
				} else {
					prNum = "-"
					prTitle = ""
				}

				fmt.Printf("%s%-16s %-10s %-8s %s\n", prefix, ctx.Name, tmuxStatus, prNum, prTitle)
			}
			fmt.Println()

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

			// Load config for hooks
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			v, err := vcs.NewAuto(repoPath)
			if err != nil {
				return err
			}

			// Get project root and worktree path for hooks
			projectRoot := getProjectRoot(v)
			if projectRoot == "" {
				projectRoot = repoPath
			}
			ctxPath := filepath.Join(projectRoot, ctxName)

			fmt.Printf("コンテキスト '%s' を削除中...\n", ctxName)

			// Run pre-remove hooks
			if len(cfg.Hooks.PreRemove) > 0 {
				hookCtx := &hooks.Context{
					ProjectRoot:  projectRoot,
					WorktreePath: ctxPath,
					ContextName:  ctxName,
					VCS:          v.Name(),
				}
				runner := hooks.NewRunner(hookCtx)
				if err := runner.ExecutePreRemove(cfg.Hooks.PreRemove); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Pre-remove hooks failed: %v\n", err)
				}
			}

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

			// Create AGENTS.md for AI assistant
			if err := store.CreateAGENTSMD(ctxPath, ctxName); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to create AGENTS.md: %v\n", err)
			}

			// Create context loader for layered context
			globalDir := filepath.Dir(config.GlobalConfigPath())
			loader := session.NewContextLoader(
				globalDir,                      // Global: ~/.config/dcell/
				projectRoot,                    // Project: dcell/
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

func attachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach <name>",
		Short: "既存のコンテキストにtmuxで接続",
		Long: `既存の開発コンテキストにtmuxセッションで接続します。
コンテキストが存在しない場合はエラーになります。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxName := args[0]

			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			// Check tmux
			if !tmux.HasTmux() {
				return fmt.Errorf("tmuxがインストールされていません")
			}

			// Detect repository
			v, err := vcs.NewAuto(repoPath)
			if err != nil {
				return err
			}

			// Get project root
			projectRoot := getProjectRoot(v)
			if projectRoot == "" {
				projectRoot = repoPath
			}

			// Check if worktree exists
			ctxPath := filepath.Join(projectRoot, ctxName)
			if _, err := os.Stat(ctxPath); os.IsNotExist(err) {
				return fmt.Errorf("コンテキスト '%s' が見つかりません: %s", ctxName, ctxPath)
			}

			sessionName := tmux.GetSessionForContext(ctxName)

			// Create session if not exists
			if !tmux.SessionExists(sessionName) {
				fmt.Printf("セッション '%s' を新規作成します...\n", sessionName)
				if err := tmux.CreateSession(sessionName, ctxPath); err != nil {
					return fmt.Errorf("tmuxセッションの作成に失敗: %w", err)
				}
			}

			fmt.Printf("🚀 コンテキスト '%s' に接続します...\n", ctxName)

			// Attach to tmux session
			if tmux.InTmux() {
				return tmux.SwitchSession(sessionName)
			}
			return tmux.AttachSession(sessionName)
		},
	}
}

func prCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "GitHub PR operations",
	}

	cmd.AddCommand(prCheckoutCmd())
	cmd.AddCommand(prListCmd())
	cmd.AddCommand(prViewCmd())

	return cmd
}

func prCheckoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "checkout <number>",
		Short: "PRをチェックアウトしてworktree作成",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !gh.HasGH() {
				return fmt.Errorf("gh CLI is not installed")
			}

			var prNum int
			fmt.Sscanf(args[0], "%d", &prNum)
			if prNum == 0 {
				return fmt.Errorf("invalid PR number: %s", args[0])
			}

			client, err := gh.New()
			if err != nil {
				return err
			}

			// Get PR info
			pr, err := client.GetPR(prNum)
			if err != nil {
				return fmt.Errorf("failed to get PR #%d: %w", prNum, err)
			}

			ctxName := pr.HeadRef
			fmt.Printf("PR #%d (%s) をチェックアウトします...\n", prNum, ctxName)

			// Use existing create flow but with PR branch
			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			v, err := vcs.NewAuto(repoPath)
			if err != nil {
				return err
			}

			// Check if already exists
			contexts, _ := v.ListContexts()
			for _, ctx := range contexts {
				if ctx.Name == ctxName {
					fmt.Printf("コンテキスト '%s' は既に存在します。attachします...\n", ctxName)
					// Attach to it
					projectRoot := getProjectRoot(v)
					if projectRoot == "" {
						projectRoot = repoPath
					}
					sessionName := tmux.GetSessionForContext(ctxName)
					if tmux.InTmux() {
						return tmux.SwitchSession(sessionName)
					}
					return tmux.AttachSession(sessionName)
				}
			}

			// Fetch PR branch
			projectRoot := getProjectRoot(v)
			if projectRoot == "" {
				projectRoot = repoPath
			}
			ctxPath := filepath.Join(projectRoot, ctxName)

			// Use gh pr checkout to fetch the branch
			fetchCmd := exec.Command("gh", "pr", "checkout", args[0], "--branch", ctxName)
			fetchCmd.Dir = v.(*vcs.Git).RepoPath
			if out, err := fetchCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to checkout PR: %w\n%s", err, out)
			}

			// Add worktree for the branch
			wtCmd := exec.Command("git", "worktree", "add", ctxPath, ctxName)
			wtCmd.Dir = v.(*vcs.Git).RepoPath
			if out, err := wtCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to create worktree: %w\n%s", err, out)
			}

			fmt.Printf("✅ PR #%d をコンテキスト '%s' として作成しました\n", prNum, ctxName)
			fmt.Printf("   パス: %s\n", ctxPath)

			// Create tmux session and attach
			sessionName := tmux.GetSessionForContext(ctxName)
			if !tmux.SessionExists(sessionName) {
				tmux.CreateSession(sessionName, ctxPath)
			}

			fmt.Printf("🚀 tmuxに接続します...\n")
			if tmux.InTmux() {
				return tmux.SwitchSession(sessionName)
			}
			return tmux.AttachSession(sessionName)
		},
	}
}

func prListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List open PRs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !gh.HasGH() {
				return fmt.Errorf("gh CLI is not installed")
			}

			client, err := gh.New()
			if err != nil {
				return err
			}

			prs, err := client.ListPRs("open")
			if err != nil {
				return err
			}

			fmt.Println("Open Pull Requests:")
			fmt.Println(strings.Repeat("-", 70))
			for _, pr := range prs {
				draft := ""
				if pr.IsDraft {
					draft = " [DRAFT]"
				}
				fmt.Printf("#%-5d %-20s %s%s\n", pr.Number, pr.HeadRef, pr.Title, draft)
			}

			return nil
		},
	}
}

func prViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view [number]",
		Short: "View PR in browser",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !gh.HasGH() {
				return fmt.Errorf("gh CLI is not installed")
			}

			client, err := gh.New()
			if err != nil {
				return err
			}

			var prNum int
			if len(args) > 0 {
				fmt.Sscanf(args[0], "%d", &prNum)
			}

			return client.ViewPR(prNum)
		},
	}
}

func submitCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "submit",
		Short: "現在のコンテキストをPRとして提出",
		Long: `現在のコンテキストの変更をPRとして作成します。
未プッシュの変更は自動的にプッシュされます。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !gh.HasGH() {
				return fmt.Errorf("gh CLI is not installed")
			}

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
				return fmt.Errorf("not in a dcell context: %w", err)
			}

			client, err := gh.New()
			if err != nil {
				return err
			}

			// Check if PR already exists
			existingPR, _ := client.GetPRForBranch(current.Name)
			if existingPR != nil {
				fmt.Printf("PR #%d が既に存在します: %s\n", existingPR.Number, existingPR.URL)
				fmt.Println("ブラウザで開きますか？ [Y/n]")
				var resp string
				fmt.Scanln(&resp)
				if resp == "" || strings.ToLower(resp) == "y" {
					return client.ViewPR(existingPR.Number)
				}
				return nil
			}

			// Generate PR title from branch name
			title := generatePRTitle(current.Name)

			if !yes {
				fmt.Printf("以下の内容でPRを作成します:\n")
				fmt.Printf("  ブランチ: %s\n", current.Name)
				fmt.Printf("  タイトル: %s\n", title)
				fmt.Println()
				fmt.Print("作成しますか？ [Y/n/d(ドラフト)] ")
				var resp string
				fmt.Scanln(&resp)

				resp = strings.ToLower(resp)
				if resp == "n" {
					fmt.Println("キャンセルしました")
					return nil
				}

				draft := resp == "d"

				fmt.Println("Pushing to origin...")
				pushCmd := exec.Command("git", "push", "-u", "origin", current.Name)
				pushCmd.Dir = current.Path
				if out, err := pushCmd.CombinedOutput(); err != nil {
					return fmt.Errorf("failed to push: %w\n%s", err, out)
				}

				fmt.Println("Creating PR...")
				if err := client.CreatePR(title, "", draft); err != nil {
					return fmt.Errorf("failed to create PR: %w", err)
				}

				fmt.Println("✅ PRを作成しました")

				// Get created PR URL
				newPR, _ := client.GetPRForBranch(current.Name)
				if newPR != nil {
					fmt.Printf("   %s\n", newPR.URL)
				}
			} else {
				// Auto mode
				fmt.Println("Pushing to origin...")
				pushCmd := exec.Command("git", "push", "-u", "origin", current.Name)
				pushCmd.Dir = current.Path
				if out, err := pushCmd.CombinedOutput(); err != nil {
					return fmt.Errorf("failed to push: %w\n%s", err, out)
				}

				fmt.Println("Creating PR...")
				if err := client.CreatePR(title, "", false); err != nil {
					return fmt.Errorf("failed to create PR: %w", err)
				}
				fmt.Println("✅ PRを作成しました")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "確認をスキップ")

	return cmd
}

func generatePRTitle(branch string) string {
	// Convert branch name to PR title
	// feat/user-auth -> feat: user auth
	// fix/login-bug -> fix: login bug

	parts := strings.SplitN(branch, "/", 2)
	if len(parts) == 2 {
		prefix := parts[0]
		desc := strings.ReplaceAll(parts[1], "-", " ")
		desc = strings.ReplaceAll(desc, "_", " ")
		return fmt.Sprintf("%s: %s", prefix, desc)
	}

	return strings.ReplaceAll(branch, "-", " ")
}


