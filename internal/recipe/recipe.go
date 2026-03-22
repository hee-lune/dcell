package recipe

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Recipe represents a recipe definition
type Recipe struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	Files       []RecipeFile `yaml:"files"`
}

// RecipeFile represents a file to be generated
type RecipeFile struct {
	Path     string `yaml:"path"`     // Template path (e.g., {{name}}.tsx)
	Template string `yaml:"template"` // File content template
}

// LoadRecipe loads a recipe from a YAML file
func LoadRecipe(path string) (*Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read recipe file: %w", err)
	}

	var recipe Recipe
	if err := yaml.Unmarshal(data, &recipe); err != nil {
		return nil, fmt.Errorf("failed to parse recipe file: %w", err)
	}

	return &recipe, nil
}

// Execute executes a recipe by generating files with variable substitution
func Execute(recipe *Recipe, vars map[string]string, targetDir string) error {
	// Validate required variables by checking all placeholders in paths and templates
	requiredVars := extractRequiredVars(recipe)
	for _, v := range requiredVars {
		if _, ok := vars[v]; !ok {
			return fmt.Errorf("required variable not provided: %s", v)
		}
	}

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Generate each file
	for _, file := range recipe.Files {
		// Substitute variables in path
		filePath := SubstituteVars(file.Path, vars)
		
		// Substitute variables in template
		content := SubstituteVars(file.Template, vars)

		// Create full path
		fullPath := filepath.Join(targetDir, filePath)

		// Create parent directories if needed
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", fullPath, err)
		}
	}

	return nil
}

// SubstituteVars replaces {{variable}} placeholders with actual values
func SubstituteVars(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// extractRequiredVars extracts all variable names used in a recipe
func extractRequiredVars(recipe *Recipe) []string {
	varSet := make(map[string]bool)
	varRegex := regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)

	// Check paths
	for _, file := range recipe.Files {
		matches := varRegex.FindAllStringSubmatch(file.Path, -1)
		for _, match := range matches {
			if len(match) > 1 {
				varSet[match[1]] = true
			}
		}

		// Check templates
		matches = varRegex.FindAllStringSubmatch(file.Template, -1)
		for _, match := range matches {
			if len(match) > 1 {
				varSet[match[1]] = true
			}
		}
	}

	// Convert to slice
	var vars []string
	for v := range varSet {
		vars = append(vars, v)
	}
	return vars
}

// ListRecipes returns a list of recipe files in the given directory
func ListRecipes(recipesDir string) ([]string, error) {
	entries, err := os.ReadDir(recipesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var recipes []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			recipes = append(recipes, name)
		}
	}

	return recipes, nil
}
