// Package snapshot manages context snapshots.
package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Metadata represents snapshot metadata.
type Metadata struct {
	Name       string    `json:"name"`
	Context    string    `json:"context"`
	Branch     string    `json:"branch"`
	CommitHash string    `json:"commit_hash"`
	Timestamp  time.Time `json:"timestamp"`
	HasDB      bool      `json:"has_db"`
	HasFiles   bool      `json:"has_files"`
}

// Store manages snapshot storage.
type Store struct {
	ProjectPath string
}

// NewStore creates a new snapshot store.
func NewStore(projectPath string) *Store {
	return &Store{
		ProjectPath: projectPath,
	}
}

// SnapshotDir returns the snapshots directory path.
func (s *Store) SnapshotDir() string {
	return filepath.Join(s.ProjectPath, ".dcell", "snapshots")
}

// GetSnapshotDir returns the directory for a specific snapshot.
func (s *Store) GetSnapshotDir(name string) string {
	return filepath.Join(s.SnapshotDir(), name)
}

// Save saves a snapshot.
func (s *Store) Save(ctxName string, name string, opts SaveOptions) error {
	snapshotDir := s.GetSnapshotDir(name)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Get context path
	ctxPath := filepath.Join(s.ProjectPath, "..", ctxName)

	// Collect metadata
	meta := &Metadata{
		Name:      name,
		Context:   ctxName,
		Timestamp: time.Now(),
	}

	// Get git info
	if branch, err := s.getBranch(ctxPath); err == nil {
		meta.Branch = branch
	}
	if commit, err := s.getCommitHash(ctxPath); err == nil {
		meta.CommitHash = commit
	}

	// Save DB if requested
	if !opts.FilesOnly {
		if err := s.saveDB(ctxPath, snapshotDir, opts.DBServices); err != nil {
			if !opts.DBOnly {
				// Don't fail if we're not in DB-only mode
				fmt.Fprintf(os.Stderr, "Warning: DB save failed: %v\n", err)
			} else {
				return err
			}
		} else {
			meta.HasDB = true
		}
	}

	// Save files if requested
	if !opts.DBOnly {
		if err := s.saveFiles(ctxPath, snapshotDir, opts); err != nil {
			if !opts.FilesOnly {
				fmt.Fprintf(os.Stderr, "Warning: Files save failed: %v\n", err)
			} else {
				return err
			}
		} else {
			meta.HasFiles = true
		}
	}

	// Save metadata
	if err := s.saveMetadata(snapshotDir, meta); err != nil {
		return err
	}

	return nil
}

// Load restores a snapshot.
func (s *Store) Load(ctxName string, name string, opts RestoreOptions) error {
	snapshotDir := s.GetSnapshotDir(name)

	// Load metadata
	meta, err := s.loadMetadata(snapshotDir)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Verify context matches
	if meta.Context != "" && meta.Context != ctxName {
		return fmt.Errorf("snapshot was created for context '%s', not '%s'", meta.Context, ctxName)
	}

	ctxPath := filepath.Join(s.ProjectPath, "..", ctxName)

	// Restore DB if present and requested
	if meta.HasDB && !opts.FilesOnly {
		if err := s.restoreDB(ctxPath, snapshotDir, opts.DBServices); err != nil {
			if !opts.DBOnly {
				fmt.Fprintf(os.Stderr, "Warning: DB restore failed: %v\n", err)
			} else {
				return err
			}
		}
	}

	// Restore files if present and requested
	if meta.HasFiles && !opts.DBOnly {
		if err := s.restoreFiles(ctxPath, snapshotDir); err != nil {
			if !opts.FilesOnly {
				fmt.Fprintf(os.Stderr, "Warning: Files restore failed: %v\n", err)
			} else {
				return err
			}
		}
	}

	return nil
}

// List returns all snapshots.
func (s *Store) List() ([]*Metadata, error) {
	entries, err := os.ReadDir(s.SnapshotDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []*Metadata{}, nil
		}
		return nil, err
	}

	var snapshots []*Metadata
	for _, entry := range entries {
		if entry.IsDir() {
			meta, err := s.loadMetadata(filepath.Join(s.SnapshotDir(), entry.Name()))
			if err == nil {
				snapshots = append(snapshots, meta)
			}
		}
	}

	return snapshots, nil
}

// Remove removes a snapshot.
func (s *Store) Remove(name string) error {
	snapshotDir := s.GetSnapshotDir(name)
	return os.RemoveAll(snapshotDir)
}

// Clean removes old snapshots, keeping the most recent n.
func (s *Store) Clean(keep int) error {
	snapshots, err := s.List()
	if err != nil {
		return err
	}

	if len(snapshots) <= keep {
		return nil
	}

	// Sort by timestamp (newest first)
	for i := 0; i < len(snapshots)-1; i++ {
		for j := i + 1; j < len(snapshots); j++ {
			if snapshots[j].Timestamp.After(snapshots[i].Timestamp) {
				snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
			}
		}
	}

	// Remove old snapshots
	for i := keep; i < len(snapshots); i++ {
		if err := s.Remove(snapshots[i].Name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to remove snapshot '%s': %v\n", snapshots[i].Name, err)
		}
	}

	return nil
}

// SaveOptions contains options for saving snapshots.
type SaveOptions struct {
	DBOnly     bool
	FilesOnly  bool
	DBServices []string // List of DB service names
}

// RestoreOptions contains options for restoring snapshots.
type RestoreOptions struct {
	DBOnly     bool
	FilesOnly  bool
	DBServices []string
}

func (s *Store) saveDB(ctxPath, snapshotDir string, services []string) error {
	if len(services) == 0 {
		services = []string{"db"}
	}

	for _, service := range services {
		dbFile := filepath.Join(snapshotDir, fmt.Sprintf("%s.sql", service))
		
		// Try pg_dump for PostgreSQL
		cmd := exec.Command("docker", "compose", "exec", "-T", service, "pg_dump", "-U", "postgres", "-d", "postgres")
		cmd.Dir = ctxPath
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to dump %s: %w", service, err)
		}

		if err := os.WriteFile(dbFile, output, 0644); err != nil {
			return fmt.Errorf("failed to write DB dump: %w", err)
		}
	}

	return nil
}

func (s *Store) restoreDB(ctxPath, snapshotDir string, services []string) error {
	if len(services) == 0 {
		services = []string{"db"}
	}

	for _, service := range services {
		dbFile := filepath.Join(snapshotDir, fmt.Sprintf("%s.sql", service))
		if _, err := os.Stat(dbFile); os.IsNotExist(err) {
			continue
		}

		data, err := os.ReadFile(dbFile)
		if err != nil {
			return fmt.Errorf("failed to read DB dump: %w", err)
		}

		// Restore using psql
		cmd := exec.Command("docker", "compose", "exec", "-T", service, "psql", "-U", "postgres")
		cmd.Dir = ctxPath
		cmd.Stdin = stringReader(data)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to restore %s: %w", service, err)
		}
	}

	return nil
}

func (s *Store) saveFiles(ctxPath, snapshotDir string, opts SaveOptions) error {
	filesPath := filepath.Join(snapshotDir, "files.tar.gz")

	// Create tar.gz archive
	file, err := os.Create(filesPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Walk the directory and add files
	return filepath.Walk(ctxPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip certain directories
		rel, _ := filepath.Rel(ctxPath, path)
		if rel == ".git" || rel == ".jj" || rel == ".dcell" || rel == ".dcell-session" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip the snapshot itself if it's inside the context
		if rel == ".dcell" {
			if info.IsDir() {
				return filepath.SkipDir
			}
		}

		if info.IsDir() {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}

func (s *Store) restoreFiles(ctxPath, snapshotDir string) error {
	filesPath := filepath.Join(snapshotDir, "files.tar.gz")

	file, err := os.Open(filesPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(ctxPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

func (s *Store) saveMetadata(snapshotDir string, meta *Metadata) error {
	metaPath := filepath.Join(snapshotDir, "meta.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}

func (s *Store) loadMetadata(snapshotDir string) (*Metadata, error) {
	metaPath := filepath.Join(snapshotDir, "meta.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

func (s *Store) getBranch(ctxPath string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = ctxPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (s *Store) getCommitHash(ctxPath string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = ctxPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func stringReader(data []byte) *stringReaderImpl {
	return &stringReaderImpl{data: data, pos: 0}
}

type stringReaderImpl struct {
	data []byte
	pos  int
}

func (r *stringReaderImpl) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
