// Package fileops implements sandboxed file operations for the agent's
// file manager feature. Every path from the panel is relative to a
// configured root directory and resolved defensively so a path like
// "../../etc/passwd" can never escape it.
package fileops

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrPathEscapesRoot = errors.New("path escapes the configured root directory")

// MaxTransferSize caps how large a file this agent will read into memory
// for a download or accept for an upload — the whole file is base64'd
// through a single JSON message, so this is deliberately conservative for
// the MVP rather than doing chunked transfer.
const MaxTransferSize = 50 * 1024 * 1024 // 50MB

type Root struct {
	base string
}

func NewRoot(base string) (*Root, error) {
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &Root{base: abs}, nil
}

// resolve turns a panel-supplied relative path into an absolute path
// guaranteed to be inside the root, or errors.
//
// filepath.Clean("/"+relPath) alone already can't produce a ".." that
// escapes above the synthetic root (Clean collapses a leading ".." on a
// rooted path), so joining it onto r.base can't escape either — the
// isWithin check below is deliberate defense in depth in case that
// assumption ever changes.
func (r *Root) resolve(relPath string) (string, error) {
	cleaned := filepath.Clean("/" + relPath)
	full := filepath.Join(r.base, cleaned)
	if !isWithin(r.base, full) {
		return "", ErrPathEscapesRoot
	}
	return full, nil
}

func isWithin(base, path string) bool {
	if path == base {
		return true
	}
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

type Entry struct {
	Name      string    `json:"name"`
	IsDir     bool      `json:"isDir"`
	IsSymlink bool      `json:"isSymlink"`
	Size      int64     `json:"size"`
	Mode      string    `json:"mode"`
	ModTime   time.Time `json:"modTime"`
}

func (r *Root) List(relPath string) ([]Entry, error) {
	full, err := r.resolve(relPath)
	if err != nil {
		return nil, err
	}
	dirEntries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(dirEntries))
	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			continue
		}
		entries = append(entries, Entry{
			Name:      de.Name(),
			IsDir:     de.IsDir(),
			IsSymlink: de.Type()&os.ModeSymlink != 0,
			Size:      info.Size(),
			Mode:      info.Mode().String(),
			ModTime:   info.ModTime(),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func (r *Root) ReadFile(relPath string) ([]byte, error) {
	full, err := r.resolve(relPath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("cannot download a directory")
	}
	if info.Size() > MaxTransferSize {
		return nil, errors.New("file too large to transfer")
	}
	return os.ReadFile(full)
}

func (r *Root) WriteFile(relPath string, data []byte) error {
	if len(data) > MaxTransferSize {
		return errors.New("file too large to transfer")
	}
	full, err := r.resolve(relPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

// Mkdir creates a directory and its missing parents inside the configured
// root. Existing directories are treated as success, matching mkdir -p.
func (r *Root) Mkdir(relPath string) error {
	full, err := r.resolve(relPath)
	if err != nil {
		return err
	}
	if full == r.base {
		return errors.New("cannot create the root directory")
	}
	return os.MkdirAll(full, 0o755)
}

// Rename moves a file or directory inside the configured root. Both sides
// are resolved independently so neither can escape the file manager root.
func (r *Root) Rename(fromPath, toPath string) error {
	from, err := r.resolve(fromPath)
	if err != nil {
		return err
	}
	to, err := r.resolve(toPath)
	if err != nil {
		return err
	}
	if from == r.base || to == r.base {
		return errors.New("cannot rename the root directory")
	}
	return os.Rename(from, to)
}

// Delete removes a single file or an empty directory. Deliberately not
// recursive (no RemoveAll) — a wrong path here shouldn't be able to wipe
// out a whole tree from a single API call.
func (r *Root) Delete(relPath string) error {
	full, err := r.resolve(relPath)
	if err != nil {
		return err
	}
	if full == r.base {
		return errors.New("cannot delete the root directory")
	}
	return os.Remove(full)
}
