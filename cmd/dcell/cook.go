package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/recipe"
)

// cookCommandHelpJSON represents the JSON structure for --help --json
type cookCommandHelpJSON struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Args        []string     `json:"args"`
	Flags       []flagDefJSON `json:"flags"`
}

type flagDefJSON struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

func cookCmd() *cobra.Command {
	var (
		vars   []string
		target string
		jsonHelp bool
	)

	cmd := &cobra.Command{
		Use:   "cook <recipe-name>",
		Short: "レシピを適用してファイルを生成",
		Long: `レシピを適用してファイルを生成します。

変数は --var フラグで指定できます（複数指定可）：
  dcell cook component --var name=Button --var path=components

出力先は --target で指定できます（デフォルトはカレントディレクトリ）：
  dcell cook component --var name=Button --target ./src/components`,
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Handle --json flag (for --help --json support)
			if jsonHelp {
				return printCookHelpJSON()
			}

			if len(args) == 0 {
				return fmt.Errorf("レシピ名を指定してください")
			}

			recipeName := args[0]

			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			// Find recipe file
			recipesDir := filepath.Join(repoPath, ".dcell", "recipes")
			recipePath := filepath.Join(recipesDir, recipeName+".yml")
			
			// Try .yaml extension if .yml doesn't exist
			if _, err := os.Stat(recipePath); os.IsNotExist(err) {
				recipePath = filepath.Join(recipesDir, recipeName+".yaml")
				if _, err := os.Stat(recipePath); os.IsNotExist(err) {
					return fmt.Errorf("レシピ '%s' が見つかりません", recipeName)
				}
			}

			// Load recipe
			rec, err := recipe.LoadRecipe(recipePath)
			if err != nil {
				return fmt.Errorf("レシピの読み込みに失敗: %w", err)
			}

			// Parse variables
			varsMap := make(map[string]string)
			for _, v := range vars {
				parts := strings.SplitN(v, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("無効な変数形式: %s（期待: key=value）", v)
				}
				varsMap[parts[0]] = parts[1]
			}

			// Determine target directory
			targetDir := target
			if targetDir == "" {
				targetDir = repoPath
			}

			// Execute recipe
			if err := recipe.Execute(rec, varsMap, targetDir); err != nil {
				return fmt.Errorf("レシピの実行に失敗: %w", err)
			}

			fmt.Printf("✅ レシピ '%s' を適用しました\n", rec.Name)
			if rec.Description != "" {
				fmt.Printf("   %s\n", rec.Description)
			}
			fmt.Printf("   出力先: %s\n", targetDir)

			return nil
		},
	}

	cmd.Flags().StringArrayVar(&vars, "var", nil, "変数指定（key=value）")
	cmd.Flags().StringVarP(&target, "target", "t", "", "出力先ディレクトリ")
	cmd.Flags().BoolVar(&jsonHelp, "json", false, "JSON形式でヘルプを出力")

	// Custom help function to support --help --json
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if jsonHelp {
			printCookHelpJSON()
			return
		}
		c.Parent().HelpFunc()(c, args)
	})

	return cmd
}

func printCookHelpJSON() error {
	help := cookCommandHelpJSON{
		Name:        "cook",
		Description: "レシピを適用してファイルを生成",
		Args:        []string{"recipe-name"},
		Flags: []flagDefJSON{
			{
				Name:        "var",
				Type:        "string",
				Description: "変数指定（key=value）",
			},
			{
				Name:        "target",
				Type:        "string",
				Description: "出力先ディレクトリ",
			},
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(help)
}
