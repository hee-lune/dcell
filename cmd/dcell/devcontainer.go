package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/devcontainer"
	"github.com/heelune/dcell/internal/docker"
)

func devcontainerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devcontainer",
		Short: "Dev Container設定の管理",
		Long:  `VS Code Dev Containerの設定を生成・管理します。`,
	}

	cmd.AddCommand(devcontainerInitCmd())
	cmd.AddCommand(devcontainerGenerateCmd())

	return cmd
}

func devcontainerInitCmd() *cobra.Command {
	var serviceName string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "既存プロジェクトにDev Container設定を追加",
		Long: `既存のプロジェクトにDev Container設定を追加します。

このコマンドは .devcontainer/devcontainer.json を作成します。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			// Detect project name from directory
			projectName := filepath.Base(repoPath)

			// Check if docker-compose.yml exists
			composePath := filepath.Join(repoPath, "docker-compose.yml")
			if _, err := os.Stat(composePath); os.IsNotExist(err) {
				fmt.Println("警告: docker-compose.yml が見つかりません。")
				fmt.Println("Dev Container設定を作成する前に docker-compose.yml を作成してください。")
			}

			// Detect or use default service name
			if serviceName == "" {
				serviceName = devcontainer.DetectService(repoPath)
			}

			// Generate devcontainer config
			generator := devcontainer.NewGenerator(projectName, repoPath)
			if err := generator.InitProject(serviceName); err != nil {
				return fmt.Errorf("Dev Container設定の作成に失敗しました: %w", err)
			}

			fmt.Println("✓ Dev Container設定を作成しました: .devcontainer/devcontainer.json")
			fmt.Println("\n次のステップ:")
			fmt.Println("  1. VS Codeで 'Dev Containers: Reopen in Container' を実行")
			fmt.Println("  2. または 'code .' でプロジェクトを開き、コンテナで再度開く")

			return nil
		},
	}

	cmd.Flags().StringVarP(&serviceName, "service", "s", "", "メインサービス名（デフォルト: app）")

	return cmd
}

func devcontainerGenerateCmd() *cobra.Command {
	var (
		ctxName     string
		serviceName string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "特定のコンテキスト用にDev Container設定を生成",
		Long: `特定の開発コンテキスト用にDev Container設定を生成します。

既存のworktreeに対してDev Container設定を追加する場合に使用します。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if ctxName == "" {
				return fmt.Errorf("コンテキスト名を --context フラグで指定してください")
			}

			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			projectName := filepath.Base(repoPath)

			// Check if context exists
			ctxPath := filepath.Join(repoPath, "..", ctxName)
			if _, err := os.Stat(ctxPath); os.IsNotExist(err) {
				return fmt.Errorf("コンテキスト '%s' が見つかりません: %s", ctxName, ctxPath)
			}

			// Load config for port allocation
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

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

			// Detect or use default service name
			if serviceName == "" {
				serviceName = devcontainer.DetectService(repoPath)
			}

			// Generate devcontainer config for worktree
			generator := devcontainer.NewGenerator(projectName, repoPath)
			if err := generator.GenerateForWorktree(ctxName, serviceName, ports); err != nil {
				return fmt.Errorf("Dev Container設定の生成に失敗しました: %w", err)
			}

			fmt.Printf("✓ コンテキスト '%s' のDev Container設定を作成しました\n", ctxName)
			fmt.Printf("  場所: %s/.devcontainer/devcontainer.json\n", ctxPath)
			
			if len(ports) > 0 {
				fmt.Println("\n転送ポート:")
				for svc, port := range ports {
					fmt.Printf("  %s: %d\n", svc, port)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&ctxName, "context", "c", "", "コンテキスト名（必須）")
	cmd.Flags().StringVarP(&serviceName, "service", "s", "", "メインサービス名（デフォルト: app）")
	cmd.MarkFlagRequired("context")

	return cmd
}
