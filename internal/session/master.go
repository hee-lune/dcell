package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/heelune/dcell/internal/config"
)

const (
	MasterContextName = "_master"
	MasterDirName     = ".dcell-master"
)

// MasterSession represents the master AI session for a project.
type MasterSession struct {
	ProjectPath   string
	ContextPath   string // path to context.md
	AGENTSPath    string // path to AGENTS.md
	ConfigPath    string // path to config.toml
}

// IsMasterAvailable checks if master AI context exists for the project.
func IsMasterAvailable(projectPath string) bool {
	masterDir := filepath.Join(projectPath, MasterDirName)
	_, err := os.Stat(masterDir)
	return err == nil
}

// GetMasterSession returns the master session for a project.
func GetMasterSession(projectPath string) *MasterSession {
	masterDir := filepath.Join(projectPath, MasterDirName)
	return &MasterSession{
		ProjectPath: projectPath,
		ContextPath: filepath.Join(masterDir, "context.md"),
		AGENTSPath:  filepath.Join(masterDir, "AGENTS.md"),
		ConfigPath:  filepath.Join(masterDir, "config.toml"),
	}
}

// InitMaster creates the master AI context for a project if it doesn't exist.
func InitMaster(projectPath string, vcsType string) error {
	masterDir := filepath.Join(projectPath, MasterDirName)
	if err := os.MkdirAll(masterDir, 0755); err != nil {
		return fmt.Errorf("failed to create master directory: %w", err)
	}

	session := GetMasterSession(projectPath)

	// Create context.md if it doesn't exist
	if _, err := os.Stat(session.ContextPath); os.IsNotExist(err) {
		if err := createMasterContextFile(session.ContextPath, vcsType); err != nil {
			return err
		}
	}

	// Create AGENTS.md if it doesn't exist
	if _, err := os.Stat(session.AGENTSPath); os.IsNotExist(err) {
		if err := createMasterAGENTSFile(session.AGENTSPath, projectPath); err != nil {
			return err
		}
	}

	return nil
}

// createMasterContextFile creates the default master context file.
func createMasterContextFile(path string, vcsType string) error {
	content := fmt.Sprintf(`# dcell Master AI Context

## 役割（Role）

あなたはこのプロジェクトの **Master AI** です。
プロジェクト全体の管理と指揮を行い、各ワークツリー（Session）で動作するワークAIを統率します。

### 主要な責務
1. **コンテキスト管理** - ワークツリーの作成、一覧表示、削除
2. **PR管理** - GitHub PRの作成、確認、チェックアウト
3. **ワークAIへの委譲** - 実際のコード編集はワークAIに任せる
4. **プロジェクト全体の把握** - 複数の作業が並行した際の調整

### 権限の範囲
| 操作 | Master AI | ワークAI |
|------|-----------|----------|
| コンテキスト作成/削除 | ✅ | ❌ |
| PR作成/管理 | ✅ | ❌ |
| コード編集 | ❌（委譲） | ✅ |
| テスト実行 | ✅ | ✅ |
| Git操作（commit/push） | ✅ | ✅ |

## dcell の使い方

### コンテキスト管理
` + "```bash" + `
# 新規コンテキスト作成
dcell create <name> --from <base-branch>
dcell work <name> "<prompt>"  # 作成+AI起動+tmux接続

# コンテキスト一覧
dcell list

# コンテキスト削除
dcell remove <name>
` + "```" + `

### PR管理
` + "```bash" + `
# PR一覧
dcell pr list

# PRをチェックアウト（レビュー用）
dcell pr checkout <number>

# 現在のコンテキストをPRとして提出
dcell submit
` + "```" + `

### AI連携
` + "```bash" + `
# ワークAI起動（特定コンテキスト）
dcell ai <context-name>

# 自分自身（Master AI）を再起動
dcell ai  # プロジェクトルートで実行
` + "```" + `

## ワークフロー指針

### 新規開発時
1. 適切な命名規則でコンテキストを作成（feature/*, fix/*, refactor/*）
2. 明確なプロンプトと共にワークAIを起動
3. 進捗は定期的に確認（tmuxでアタッチして確認）

### 既存PRレビュー時
1. ` + "`dcell pr checkout <number>`" + ` でPRをワークツリー化
2. コードレビュー用のワークAIを起動
3. レビュー結果をフィードバック

### 作業完了時
1. ワークAIの作業内容を確認
2. ` + "`dcell submit`" + ` でPR作成
3. 必要に応じてワークツリーを削除

## VCS情報
- タイプ: %s
- ワークツリー構造: フラット（main/, feature-x/ などが同階層）

## 連絡先・参考情報
<!-- プロジェクト固有の情報を追記 -->
`, vcsType)

	return os.WriteFile(path, []byte(content), 0644)
}

// createMasterAGENTSFile creates the AGENTS.md for master AI.
func createMasterAGENTSFile(path string, projectPath string) error {
	globalDir := filepath.Dir(config.GlobalConfigPath())
	globalCtxPath := filepath.Join(globalDir, "context.md")
	projectCtxPath := filepath.Join(projectPath, ".context.md")
	masterCtxPath := filepath.Join(projectPath, MasterDirName, "context.md")

	content := fmt.Sprintf(`# dcell Master AI Session

## 階層型コンテキストファイル

このセッションで作業を開始する前に、以下のファイルを**順番に**読み込んでください：

### 1. Global Context
**パス**: %s
- ユーザー全体の設定、個人の好み、グローバルな制約

### 2. Project Context  
**パス**: %s
- プロジェクト固有の技術スタック、コーディング規約
- 言語、フレームワーク、アーキテクチャの制約

### 3. Master Context（最重要）
**パス**: %s
- **あなたの役割と責務**
- dcell の使い方、ワークフロー指針
- ワークAIとの役割分担

## 指示

1. **必ず上記3つのコンテキストをすべて読み込んで**から作業を開始
2. **自分がMaster AIであることを常に意識**してください
3. **コード編集が必要な場合は、ワークAIに委譲**してください
4. **コンテキスト作成時は、適切な命名規則を遵守**してください

セッション作成時刻: %s
`, globalCtxPath, projectCtxPath, masterCtxPath, time.Now().Format("2006-01-02 15:04"))

	return os.WriteFile(path, []byte(content), 0644)
}

// IsProjectRoot checks if the given path is a dcell project root.
func IsProjectRoot(path string) bool {
	// Check for .bare directory (dcell project marker)
	barePath := filepath.Join(path, ".bare")
	if _, err := os.Stat(barePath); err == nil {
		return true
	}

	// Check for .dcell directory (alternative marker)
	dcellPath := filepath.Join(path, ".dcell")
	if _, err := os.Stat(dcellPath); err == nil {
		return true
	}

	return false
}

// ListContexts returns a formatted list of available contexts.
func ListContexts(projectPath string) (string, error) {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return "", err
	}

	var contexts []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip hidden directories and special directories
		if strings.HasPrefix(name, ".") || name == MasterDirName {
			continue
		}
		// Check if it's a worktree (has .git or is a git worktree)
		gitPath := filepath.Join(projectPath, name, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			contexts = append(contexts, name)
		}
	}

	if len(contexts) == 0 {
		return "No contexts found. Create one with: dcell create <name>", nil
	}

	result := "Available contexts:\n"
	for _, ctx := range contexts {
		result += fmt.Sprintf("  - %s\n", ctx)
	}
	result += "\nUse 'dcell ai <context-name>' to start AI in a specific context."

	return result, nil
}
