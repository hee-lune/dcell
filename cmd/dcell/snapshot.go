package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/snapshot"
)

func snapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "スナップショットの管理",
		Long:  `開発コンテキストのスナップショットを保存・復元します。`,
	}

	cmd.AddCommand(snapshotSaveCmd())
	cmd.AddCommand(snapshotListCmd())
	cmd.AddCommand(snapshotRestoreCmd())
	cmd.AddCommand(snapshotRemoveCmd())
	cmd.AddCommand(snapshotCleanCmd())

	return cmd
}

func snapshotSaveCmd() *cobra.Command {
	var (
		dbOnly    bool
		filesOnly bool
		services  []string
	)

	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "現在の状態をスナップショットとして保存",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			// Determine context name from current directory
			ctxName := filepath.Base(repoPath)

			// Context name is derived from directory name in flat structure

			store := snapshot.NewStore(repoPath)

			opts := snapshot.SaveOptions{
				DBOnly:     dbOnly,
				FilesOnly:  filesOnly,
				DBServices: services,
			}

			fmt.Printf("スナップショット '%s' を保存中...\n", name)

			if err := store.Save(ctxName, name, opts); err != nil {
				return fmt.Errorf("スナップショットの保存に失敗しました: %w", err)
			}

			fmt.Printf("✓ スナップショット '%s' を保存しました\n", name)

			return nil
		},
	}

	cmd.Flags().BoolVar(&dbOnly, "db-only", false, "DBのみ保存")
	cmd.Flags().BoolVar(&filesOnly, "files-only", false, "ファイルのみ保存")
	cmd.Flags().StringArrayVarP(&services, "service", "s", []string{}, "DBサービス名（デフォルト: db）")

	return cmd
}

func snapshotListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "保存済みスナップショット一覧",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			store := snapshot.NewStore(repoPath)
			snapshots, err := store.List()
			if err != nil {
				return err
			}

			if len(snapshots) == 0 {
				fmt.Println("スナップショットはありません")
				return nil
			}

			fmt.Printf("スナップショット一覧:\n\n")
			for _, s := range snapshots {
				fmt.Printf("  %s\n", s.Name)
				fmt.Printf("    コンテキスト: %s\n", s.Context)
				if s.Branch != "" {
					fmt.Printf("    ブランチ: %s\n", s.Branch)
				}
				if s.CommitHash != "" {
					hash := s.CommitHash
					if len(hash) > 7 {
						hash = hash[:7]
					}
					fmt.Printf("    コミット: %s\n", hash)
				}
				fmt.Printf("    作成日: %s\n", s.Timestamp.Format(time.RFC3339))
				fmt.Printf("    内容: ")
				if s.HasDB {
					fmt.Print("DB ")
				}
				if s.HasFiles {
					fmt.Print("Files ")
				}
				fmt.Println()
				fmt.Println()
			}

			return nil
		},
	}
}

func snapshotRestoreCmd() *cobra.Command {
	var (
		dbOnly    bool
		filesOnly bool
		services  []string
	)

	cmd := &cobra.Command{
		Use:   "restore <name>",
		Short: "スナップショットを復元",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			ctxName := filepath.Base(repoPath)

			store := snapshot.NewStore(repoPath)

			opts := snapshot.RestoreOptions{
				DBOnly:     dbOnly,
				FilesOnly:  filesOnly,
				DBServices: services,
			}

			fmt.Printf("スナップショット '%s' を復元中...\n", name)

			if err := store.Load(ctxName, name, opts); err != nil {
				return fmt.Errorf("スナップショットの復元に失敗しました: %w", err)
			}

			fmt.Printf("✓ スナップショット '%s' を復元しました\n", name)

			return nil
		},
	}

	cmd.Flags().BoolVar(&dbOnly, "db-only", false, "DBのみ復元")
	cmd.Flags().BoolVar(&filesOnly, "files-only", false, "ファイルのみ復元")
	cmd.Flags().StringArrayVarP(&services, "service", "s", []string{}, "DBサービス名（デフォルト: db）")

	return cmd
}

func snapshotRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "スナップショットを削除",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			store := snapshot.NewStore(repoPath)

			fmt.Printf("スナップショット '%s' を削除中...\n", name)

			if err := store.Remove(name); err != nil {
				return fmt.Errorf("スナップショットの削除に失敗しました: %w", err)
			}

			fmt.Printf("✓ スナップショット '%s' を削除しました\n", name)

			return nil
		},
	}
}

func snapshotCleanCmd() *cobra.Command {
	var keep int

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "古いスナップショットを削除",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			store := snapshot.NewStore(repoPath)

			fmt.Printf("古いスナップショットを削除中（最新 %d 個を保持）...\n", keep)

			if err := store.Clean(keep); err != nil {
				return fmt.Errorf("スナップショットのクリーンアップに失敗しました: %w", err)
			}

			fmt.Println("✓ 古いスナップショットを削除しました")

			return nil
		},
	}

	cmd.Flags().IntVarP(&keep, "keep", "k", 5, "保持するスナップショット数")

	return cmd
}
