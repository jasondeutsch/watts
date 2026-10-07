package kit

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Files contains the scripts, templates and skills needed to initialize a project.
//
//go:embed all:kit
var Files embed.FS

func Digest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Initialize creates missing project assets. Existing files are left untouched.
func Initialize(directory string) error {
	return initializeFrom(Files, "kit", directory)
}

func initializeFrom(source fs.FS, root, directory string) error {
	return fs.WalkDir(source, root, func(sourcePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative := strings.TrimPrefix(sourcePath, root+"/")
		target := filepath.Join(directory, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if strings.HasSuffix(relative, ".sh") {
			mode = 0755
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		if err != nil {
			return err
		}
		contents, readErr := fs.ReadFile(source, sourcePath)
		if readErr != nil {
			file.Close()
			return readErr
		}
		_, writeErr := file.Write(contents)
		return errors.Join(writeErr, file.Close())
	})
}

// copyDir copies a directory tree, replacing dst.
func CopyDir(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		taskPath, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, taskPath)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

// embeddedSkillNames lists the skills shipped in the binary, for tests and messages.
func EmbeddedSkillNames() []string {
	entries, err := fs.ReadDir(Files, path.Join("kit", "skills"))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}
