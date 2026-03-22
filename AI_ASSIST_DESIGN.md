# AI向け最強補助機能設計

## 目標
AIが `dcell` を使う際に、ミスをゼロにするための補助機能群

## 機能一覧

### 1. 事前バリデーション機能 (--validate)
```bash
dcell work feature-x --validate
# → 実行せずに「このコマンドは成功するか」を検証
# → エラー時は具体的な修正提案
```

**検証項目:**
- ブランチ名の衝突
- ディスク容量
- 必要ツールの存在確認
- 権限チェック

### 2. ドライラン機能 (--dry-run)
```bash
dcell work feature-x --dry-run
# → 実際には実行せず、何をするかをJSONで出力
```

**出力例:**
```json
{
  "action": "create_worktree",
  "target_path": "/Users/.../feature-x",
  "base_branch": "main",
  "estimated_size": "50MB",
  "commands": [
    "git worktree add -b feature-x ...",
    "tmux new-session -d -s dcell-feature-x"
  ],
  "risks": []
}
```

### 3. セマンティック解析 (--explain)
```bash
dcell work feature-x --explain
# → 人間/AI向けに「このコマンドは何をするか」を自然言語で説明
```

**出力例:**
```
このコマンドは:
1. 'feature-x' ブランチを 'main' から作成
2. 新しいworktreeを /Users/.../feature-x に配置
3. tmuxセッション 'dcell-feature-x' を作成
4. 自動的にtmuxにアタッチ

推定所要時間: 3秒
必要ディスク容量: 50MB
```

### 4. 対話モード (--interactive)
```bash
dcell work --interactive
# → AIが段階的に入力
# → 不足パラメータを対話で補完
```

**対話例:**
```
$ dcell work --interactive
コンテキスト名: feature-x
ベースブランチ [main]: develop
AIアシスタントを起動しますか？ [y/N]: y
→ 実行しますか？ [Y/n/dry-run]: dry-run
```

### 5. スキーマ拡張 (--help --json-schema)
```bash
dcell work --help --json-schema
# → JSON Schema形式で厳密な型定義を出力
```

**出力例:**
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "pattern": "^[a-zA-Z][a-zA-Z0-9_-]*$",
      "minLength": 1,
      "maxLength": 100
    },
    "from": {
      "type": "string",
      "description": "ベースブランチ名"
    }
  },
  "required": ["name"]
}
```

### 6. 自動修正提案 (--suggest-fix)
```bash
dcell work feature-x
# → エラー時に自動で修正案を提示

# 出力例:
エラー: ブランチ 'feature-x' は既に存在します
修正案:
  1. dcell attach feature-x           # 既存に接続
  2. dcell work feature-x-v2 --from feature-x  # 別名で作成
  3. dcell remove feature-x -f && dcell work feature-x  # 削除して再作成
```

### 7. コンテキスト認識強化 (--context)
```bash
dcell --context
# → 現在の状態をJSONで出力
```

**出力例:**
```json
{
  "current_worktree": "main",
  "current_tmux_session": "dcell-main",
  "available_worktrees": ["main", "feature-a", "hotfix-b"],
  "git_status": {
    "branch": "main",
    "uncommitted_changes": false
  },
  "system_resources": {
    "disk_free_gb": 45.2,
    "memory_available_mb": 2048
  }
}
```

## 実装優先度

1. **--dry-run** (最高) - 実行前確認の基本
2. **--validate** - 事前検証
3. **--explain** - 可視化
4. **--suggest-fix** - エラー時の補助
5. **--json-schema** - 厳密な型定義
6. **--interactive** - 対話モード
7. **--context** - 環境認識

## 実装計画

### Phase 1: 基盤 (--dry-run, --validate)
- 各コマンドにdry-run対応を追加
- バリデーションロジックを共通化

### Phase 2: 補助 (--explain, --suggest-fix)
- エラーハンドリング強化
- 自然言語生成

### Phase 3: 高度 (--json-schema, --interactive)
- JSON Schema生成
- 対話インターフェース

### Phase 4: 環境 (--context)
- システム状態収集
