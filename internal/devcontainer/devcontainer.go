// Package devcontainer manages Dev Container configurations.
package devcontainer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents a devcontainer.json configuration.
type Config struct {
	Name              string                 `json:"name"`
	DockerComposeFile []string               `json:"dockerComposeFile"`
	Service           string                 `json:"service"`
	WorkspaceFolder   string                 `json:"workspaceFolder"`
	Features          map[string]interface{} `json:"features,omitempty"`
	PostCreateCommand string                 `json:"postCreateCommand,omitempty"`
	Customizations    *Customizations        `json:"customizations,omitempty"`
}

// Customizations represents editor customizations.
type Customizations struct {
	VSCode *VSCodeConfig `json:"vscode,omitempty"`
}

// VSCodeConfig represents VS Code specific settings.
type VSCodeConfig struct {
	Extensions []string          `json:"extensions,omitempty"`
	Settings   map[string]interface{} `json:"settings,omitempty"`
}

// Generator generates devcontainer configurations.
type Generator struct {
	ProjectName string
	ProjectPath string
}

// NewGenerator creates a new devcontainer generator.
func NewGenerator(projectName, projectPath string) *Generator {
	return &Generator{
		ProjectName: projectName,
		ProjectPath: projectPath,
	}
}

// Generate creates a devcontainer.json file for a context.
func (g *Generator) Generate(ctxName string, serviceName string) error {
	devcontainerDir := filepath.Join(g.ProjectPath, "..", ctxName, ".devcontainer")
	
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		return fmt.Errorf("failed to create .devcontainer directory: %w", err)
	}

	config := g.createConfig(ctxName, serviceName)
	
	configPath := filepath.Join(devcontainerDir, "devcontainer.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal devcontainer config: %w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write devcontainer.json: %w", err)
	}
	
	return nil
}

// GenerateForWorktree creates a devcontainer.json for a worktree context with port adjustments.
func (g *Generator) GenerateForWorktree(ctxName string, serviceName string, ports map[string]int) error {
	devcontainerDir := filepath.Join(g.ProjectPath, "..", ctxName, ".devcontainer")
	
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		return fmt.Errorf("failed to create .devcontainer directory: %w", err)
	}

	config := g.createConfig(ctxName, serviceName)
	
	// Add port forwarding if available
	var forwardPorts []int
	for _, port := range ports {
		forwardPorts = append(forwardPorts, port)
	}
	
	configPath := filepath.Join(devcontainerDir, "devcontainer.json")
	
	// Marshal with custom handling for port forwarding
	data, err := g.marshalWithForwardPorts(config, forwardPorts)
	if err != nil {
		return fmt.Errorf("failed to marshal devcontainer config: %w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write devcontainer.json: %w", err)
	}
	
	return nil
}

// InitProject initializes devcontainer configuration for the main project.
func (g *Generator) InitProject(serviceName string) error {
	devcontainerDir := filepath.Join(g.ProjectPath, ".devcontainer")
	
	if err := os.MkdirAll(devcontainerDir, 0755); err != nil {
		return fmt.Errorf("failed to create .devcontainer directory: %w", err)
	}

	config := g.createConfigForMain(serviceName)
	
	configPath := filepath.Join(devcontainerDir, "devcontainer.json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal devcontainer config: %w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write devcontainer.json: %w", err)
	}
	
	return nil
}

// Cleanup removes devcontainer configuration for a context.
func (g *Generator) Cleanup(ctxName string) error {
	devcontainerDir := filepath.Join(g.ProjectPath, "..", ctxName, ".devcontainer")
	return os.RemoveAll(devcontainerDir)
}

func (g *Generator) createConfig(ctxName string, serviceName string) *Config {
	if serviceName == "" {
		serviceName = "app"
	}
	
	return &Config{
		Name: fmt.Sprintf("%s-%s", g.ProjectName, ctxName),
		DockerComposeFile: []string{
			"../../docker-compose.yml",
			"../../docker-compose.dcell.yml",
		},
		Service:         serviceName,
		WorkspaceFolder: "/workspace",
		Features: map[string]interface{}{
			"ghcr.io/devcontainers/features/git:1":        map[string]interface{}{},
			"ghcr.io/devcontainers/features/github-cli:1": map[string]interface{}{},
		},
		PostCreateCommand: "git config --global --add safe.directory /workspace",
		Customizations: &Customizations{
			VSCode: &VSCodeConfig{
				Extensions: []string{
					"golang.go",
				},
			},
		},
	}
}

// createConfigForMain creates a devcontainer config for the main project (not a worktree).
func (g *Generator) createConfigForMain(serviceName string) *Config {
	if serviceName == "" {
		serviceName = "app"
	}
	
	return &Config{
		Name: g.ProjectName,
		DockerComposeFile: []string{
			"../docker-compose.yml",
			"../docker-compose.dcell.yml",
		},
		Service:         serviceName,
		WorkspaceFolder: "/workspace",
		Features: map[string]interface{}{
			"ghcr.io/devcontainers/features/git:1":        map[string]interface{}{},
			"ghcr.io/devcontainers/features/github-cli:1": map[string]interface{}{},
		},
		PostCreateCommand: "git config --global --add safe.directory /workspace",
		Customizations: &Customizations{
			VSCode: &VSCodeConfig{
				Extensions: []string{
					"golang.go",
				},
			},
		},
	}
}

func (g *Generator) marshalWithForwardPorts(config *Config, forwardPorts []int) ([]byte, error) {
	// Create a map to have more control over JSON output
	m := map[string]interface{}{
		"name":              config.Name,
		"dockerComposeFile": config.DockerComposeFile,
		"service":           config.Service,
		"workspaceFolder":   config.WorkspaceFolder,
		"features":          config.Features,
		"postCreateCommand": config.PostCreateCommand,
	}
	
	if len(forwardPorts) > 0 {
		m["forwardPorts"] = forwardPorts
	}
	
	if config.Customizations != nil {
		m["customizations"] = config.Customizations
	}
	
	return json.MarshalIndent(m, "", "  ")
}

// DetectService attempts to detect the main service name from docker-compose.yml.
func DetectService(projectPath string) string {
	// Default service name
	return "app"
}
