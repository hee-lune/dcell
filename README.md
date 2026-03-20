# dcell

開発コンテキスト（Development Cell）管理ツール

- **Git/JJ worktree** - 独立した作業コピーの管理
- **Docker環境** - ポート自動割り当て、分離されたサービス
- **AIセッション** - コンテキスト認識型AIアシスタント連携
- **Dev Container** - VS Code Dev Container対応
- **スナップショット** - DBとファイルの状態保存・復元

## インストール

```bash
# ソースからビルド
git clone https://github.com/heelune/dcell
cd dcell
go build -o dcell ./cmd/dcell
mv dcell ~/.local/bin/
```

## クイックスタート

### 新規プロジェクトの作成

```bash
# 新規プロジェクトを初期化（bareリポジトリ）
dcell init my-project
cd my-project/main

# 既存リポジトリをクローン
dcell init my-project --clone https://github.com/user/repo.git
```

### コンテキストの管理

```bash
# 新しい開発コンテキストを作成
dcell create feature-x --from main

# コンテキスト一覧を表示
dcell list

# コンテキストに切り替え
dcell switch feature-x
cd ../feature-x  # フラット構造: main/ と同じ階層

# AIアシスタントを起動
dcell ai

# 使用後に削除
dcell remove feature-x
```

## ディレクトリ構造

```
my-project/
├── .bare/          # bareリポジトリ
├── main/           # メインworktree
├── feature-x/      # 追加worktree（フラット構造）
└── .dcell/         # dcell設定
```

## 機能

### VCS対応

- **Jujutsu (jj)** - ネイティブワークスペースサポート
- **Git** - Worktreeサポート、自動フォールバック

### Docker連携

- 自動ポート割り当て（競合防止）
- `docker-compose.dcell.yml` 自動生成
- コンテキストごとの `.env.dcell`（データベースURL等）

### AIセッション管理

- コンテキストごとのセッション保存
- `context.md`, `todo.md`, `decisions.md` 自動作成
- Claude Code / Kimi CLI 対応

### Dev Container連携

- `.devcontainer/devcontainer.json` 自動生成
- worktreeごとに独立したDev Container設定
- VS Codeでの「コンテナで再度開く」対応

### スナップショット機能

- DB状態の保存・復元（PostgreSQL対応）
- ファイル変更の保存・復元
- ブランチ・コミット情報の記録

## 設定

### グローバル設定: `~/.config/dcell/config.toml`

```toml
[vcs]
prefer = "jj"           # "jj" または "git"
default_branch = "main" # dcell create のデフォルトブランチ

[docker]
port_base = 3000
port_step = 10
services = ["app", "db", "redis"]

[ai]
default = "claude"  # "claude" または "kimi"
```

### プロジェクト設定: `.dcell/config.toml`

プロジェクト固有の設定を上書きできます。

## コマンド一覧

| コマンド | 説明 |
|---------|------|
| `init <dir>` | 新規プロジェクトを初期化 |
| `create <name>` | 新しい開発コンテキストを作成 |
| `switch <name>` | 開発コンテキストに切り替え |
| `list` | 開発コンテキストの一覧表示 |
| `remove <name>` | 開発コンテキストを削除 |
| `ai [name]` | AIアシスタントを起動 |
| `devcontainer` | Dev Container設定の管理 |
| `snapshot` | スナップショットの管理 |

### init コマンド

```bash
# 新規ローカルリポジトリを作成
dcell init my-project

# main worktreeが自動作成される
dcell init my-project

# 既存リポジトリをクローン
dcell init my-project --clone https://github.com/user/repo.git

# 特定ブランチをクローン
dcell init my-project --clone https://github.com/user/repo.git --branch develop

# Jujutsuを使用
dcell init my-project --vcs jj
```

## ライセンス

MIT
