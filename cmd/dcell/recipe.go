package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/heelune/dcell/internal/recipe"
)

func recipeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recipe",
		Short: "レシピ関連のコマンド",
	}

	cmd.AddCommand(recipeListCmd())

	return cmd
}

func recipeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "レシピ一覧を表示",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := os.Getwd()
			if err != nil {
				return err
			}

			recipesDir := filepath.Join(repoPath, ".dcell", "recipes")
			recipes, err := recipe.ListRecipes(recipesDir)
			if err != nil {
				return fmt.Errorf("failed to list recipes: %w", err)
			}

			if len(recipes) == 0 {
				fmt.Println("レシピが見つかりません")
				fmt.Printf("レシピディレクトリ: %s\n", recipesDir)
				return nil
			}

			fmt.Println("利用可能なレシピ:")
			fmt.Println(strings.Repeat("-", 50))

			for _, r := range recipes {
				recipePath := filepath.Join(recipesDir, r)
				rec, err := recipe.LoadRecipe(recipePath)
				if err != nil {
					fmt.Printf("  %-20s (読み込みエラー: %v)\n", r, err)
					continue
				}
				fmt.Printf("  %-20s %s\n", rec.Name, rec.Description)
			}

			return nil
		},
	}
}
