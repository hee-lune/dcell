package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	store := NewStore("/tmp/test")
	if store.ProjectPath != "/tmp/test" {
		t.Errorf("ProjectPath = %s, want /tmp/test", store.ProjectPath)
	}
}

func TestSnapshotDir(t *testing.T) {
	store := NewStore("/tmp/test")
	want := filepath.Join("/tmp/test", ".dcell", "snapshots")
	if got := store.SnapshotDir(); got != want {
		t.Errorf("SnapshotDir = %s, want %s", got, want)
	}
}

func TestGetSnapshotDir(t *testing.T) {
	store := NewStore("/tmp/test")
	want := filepath.Join("/tmp/test", ".dcell", "snapshots", "snapshot1")
	if got := store.GetSnapshotDir("snapshot1"); got != want {
		t.Errorf("GetSnapshotDir = %s, want %s", got, want)
	}
}

func TestSaveAndLoadMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	
	meta := &Metadata{
		Name:       "test-snapshot",
		Context:    "feature-x",
		Branch:     "main",
		CommitHash: "abc123",
		Timestamp:  time.Now(),
		HasDB:      true,
		HasFiles:   true,
	}
	
	snapshotDir := store.GetSnapshotDir("test-snapshot")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		t.Fatalf("Failed to create snapshot dir: %v", err)
	}
	
	// Save metadata
	if err := store.saveMetadata(snapshotDir, meta); err != nil {
		t.Fatalf("saveMetadata failed: %v", err)
	}
	
	// Load metadata
	loaded, err := store.loadMetadata(snapshotDir)
	if err != nil {
		t.Fatalf("loadMetadata failed: %v", err)
	}
	
	if loaded.Name != meta.Name {
		t.Errorf("Name = %s, want %s", loaded.Name, meta.Name)
	}
	if loaded.Context != meta.Context {
		t.Errorf("Context = %s, want %s", loaded.Context, meta.Context)
	}
	if loaded.Branch != meta.Branch {
		t.Errorf("Branch = %s, want %s", loaded.Branch, meta.Branch)
	}
	if loaded.CommitHash != meta.CommitHash {
		t.Errorf("CommitHash = %s, want %s", loaded.CommitHash, meta.CommitHash)
	}
	if loaded.HasDB != meta.HasDB {
		t.Errorf("HasDB = %v, want %v", loaded.HasDB, meta.HasDB)
	}
	if loaded.HasFiles != meta.HasFiles {
		t.Errorf("HasFiles = %v, want %v", loaded.HasFiles, meta.HasFiles)
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	
	// 空のリストをテスト
	snapshots, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(snapshots) != 0 {
		t.Errorf("len(snapshots) = %d, want 0", len(snapshots))
	}
	
	// スナップショットを作成
	for _, name := range []string{"snapshot1", "snapshot2"} {
		snapshotDir := store.GetSnapshotDir(name)
		if err := os.MkdirAll(snapshotDir, 0755); err != nil {
			t.Fatalf("Failed to create snapshot dir: %v", err)
		}
		
		meta := &Metadata{
			Name:      name,
			Context:   "feature-x",
			Timestamp: time.Now(),
		}
		if err := store.saveMetadata(snapshotDir, meta); err != nil {
			t.Fatalf("saveMetadata failed: %v", err)
		}
	}
	
	// リストを取得
	snapshots, err = store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(snapshots) != 2 {
		t.Errorf("len(snapshots) = %d, want 2", len(snapshots))
	}
}

func TestRemove(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	
	// スナップショットを作成
	snapshotDir := store.GetSnapshotDir("to-delete")
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		t.Fatalf("Failed to create snapshot dir: %v", err)
	}
	
	meta := &Metadata{
		Name:    "to-delete",
		Context: "feature-x",
	}
	if err := store.saveMetadata(snapshotDir, meta); err != nil {
		t.Fatalf("saveMetadata failed: %v", err)
	}
	
	// 削除
	if err := store.Remove("to-delete"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	
	// 削除されたか確認
	if _, err := os.Stat(snapshotDir); !os.IsNotExist(err) {
		t.Errorf("Snapshot directory was not removed")
	}
}

func TestClean(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewStore(tmpDir)
	
	// 複数のスナップショットを作成
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("snapshot%d", i)
		snapshotDir := store.GetSnapshotDir(name)
		if err := os.MkdirAll(snapshotDir, 0755); err != nil {
			t.Fatalf("Failed to create snapshot dir: %v", err)
		}
		
		meta := &Metadata{
			Name:      name,
			Context:   "feature-x",
			Timestamp: time.Now().Add(time.Duration(i) * time.Hour), // 時間差をつける
		}
		if err := store.saveMetadata(snapshotDir, meta); err != nil {
			t.Fatalf("saveMetadata failed: %v", err)
		}
	}
	
	// 古いものを削除（最新3つを保持）
	if err := store.Clean(3); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}
	
	// 残っているスナップショットを確認
	snapshots, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(snapshots) != 3 {
		t.Errorf("len(snapshots) = %d, want 3", len(snapshots))
	}
}
