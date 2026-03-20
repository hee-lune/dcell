package devcontainer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewGenerator(t *testing.T) {
	g := NewGenerator("test-project", "/tmp/test")
	if g.ProjectName != "test-project" {
		t.Errorf("ProjectName = %s, want test-project", g.ProjectName)
	}
	if g.ProjectPath != "/tmp/test" {
		t.Errorf("ProjectPath = %s, want /tmp/test", g.ProjectPath)
	}
}

func TestCreateConfig(t *testing.T) {
	g := NewGenerator("test-project", "/tmp/test")
	
	tests := []struct {
		name        string
		ctxName     string
		serviceName string
		wantService string
	}{
		{
			name:        "default service",
			ctxName:     "feature-x",
			serviceName: "",
			wantService: "app",
		},
		{
			name:        "custom service",
			ctxName:     "feature-y",
			serviceName: "web",
			wantService: "web",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := g.createConfig(tt.ctxName, tt.serviceName)
			
			wantName := "test-project-" + tt.ctxName
			if config.Name != wantName {
				t.Errorf("Name = %s, want %s", config.Name, wantName)
			}
			
			if config.Service != tt.wantService {
				t.Errorf("Service = %s, want %s", config.Service, tt.wantService)
			}
			
			if config.WorkspaceFolder != "/workspace" {
				t.Errorf("WorkspaceFolder = %s, want /workspace", config.WorkspaceFolder)
			}
			
			if len(config.DockerComposeFile) != 2 {
				t.Errorf("DockerComposeFile length = %d, want 2", len(config.DockerComposeFile))
			}
		})
	}
}

func TestInitProject(t *testing.T) {
	tmpDir := t.TempDir()
	g := NewGenerator("test-project", tmpDir)
	
	err := g.InitProject("app")
	if err != nil {
		t.Fatalf("InitProject failed: %v", err)
	}
	
	// 設定ファイルが作成されたか確認
	configPath := filepath.Join(tmpDir, ".devcontainer", "devcontainer.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("devcontainer.json was not created")
	}
	
	// 内容を確認
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}
	
	if config.Name != "test-project" {
		t.Errorf("Name = %s, want test-project", config.Name)
	}
	
	// メインプロジェクトの場合はパスが異なる
	if len(config.DockerComposeFile) != 2 {
		t.Errorf("DockerComposeFile length = %d, want 2", len(config.DockerComposeFile))
	}
}

func TestGenerateForWorktree(t *testing.T) {
	// 親ディレクトリとworktreeディレクトリを作成
	parentDir := t.TempDir()
	repoPath := filepath.Join(parentDir, "main")
	worktreePath := filepath.Join(parentDir, "feature-x")
	
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("Failed to create worktree dir: %v", err)
	}
	
	g := NewGenerator("test-project", repoPath)
	
	ports := map[string]int{
		"app": 3000,
		"db":  5432,
	}
	
	err := g.GenerateForWorktree("feature-x", "app", ports)
	if err != nil {
		t.Fatalf("GenerateForWorktree failed: %v", err)
	}
	
	// 設定ファイルが作成されたか確認
	configPath := filepath.Join(worktreePath, ".devcontainer", "devcontainer.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("devcontainer.json was not created in worktree")
	}
	
	// 内容を確認
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	
	// forwardPortsが含まれているか確認
	var configMap map[string]interface{}
	if err := json.Unmarshal(data, &configMap); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}
	
	if _, ok := configMap["forwardPorts"]; !ok {
		t.Errorf("forwardPorts not found in config")
	}
}

func TestCleanup(t *testing.T) {
	parentDir := t.TempDir()
	repoPath := filepath.Join(parentDir, "main")
	worktreePath := filepath.Join(parentDir, "feature-x")
	
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("Failed to create worktree dir: %v", err)
	}
	
	g := NewGenerator("test-project", repoPath)
	
	// まず設定を作成
	err := g.Generate("feature-x", "app")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	
	// クリーンアップ
	err = g.Cleanup("feature-x")
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
	
	// ディレクトリが削除されたか確認
	devcontainerDir := filepath.Join(worktreePath, ".devcontainer")
	if _, err := os.Stat(devcontainerDir); !os.IsNotExist(err) {
		t.Errorf(".devcontainer directory was not removed")
	}
}

func TestDetectService(t *testing.T) {
	tmpDir := t.TempDir()
	
	// 現在は常に "app" を返す
	service := DetectService(tmpDir)
	if service != "app" {
		t.Errorf("DetectService = %s, want app", service)
	}
}
