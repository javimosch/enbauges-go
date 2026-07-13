// Package plugin is the stable API mini-apps build against, plus the
// overlay filesystem that makes UI files editable without reshipping
// the binary.
package plugin

import (
	"io/fs"
	"os"
	"path/filepath"
)

// OverlayFS serves files from diskDir when they exist there, falling back
// to the embedded copy otherwise. diskDir may not exist at all — then the
// overlay is a pure pass-through to embedded. This is what lets a
// `rsync web/` deploy a UI change with no rebuild and no restart.
type OverlayFS struct {
	diskDir  string
	embedded fs.FS
}

func NewOverlayFS(diskDir string, embedded fs.FS) *OverlayFS {
	return &OverlayFS{diskDir: diskDir, embedded: embedded}
}

// OnDisk reports whether name currently resolves to a disk file.
func (o *OverlayFS) OnDisk(name string) bool {
	if o.diskDir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(o.diskDir, filepath.FromSlash(name)))
	return err == nil && !info.IsDir()
}

func (o *OverlayFS) Open(name string) (fs.File, error) {
	if o.OnDisk(name) {
		return os.Open(filepath.Join(o.diskDir, filepath.FromSlash(name)))
	}
	return o.embedded.Open(name)
}

func (o *OverlayFS) ReadFile(name string) ([]byte, error) {
	if o.OnDisk(name) {
		return os.ReadFile(filepath.Join(o.diskDir, filepath.FromSlash(name)))
	}
	return fs.ReadFile(o.embedded, name)
}
