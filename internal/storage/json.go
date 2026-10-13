package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// WriteJSONAtomically replaces a complete record without exposing a partially written file.
func WriteJSONAtomically(artifactPath string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomically(artifactPath, append(data, '\n'))
}

// WriteFileAtomically replaces a complete file in the same directory.
func WriteFileAtomically(artifactPath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(artifactPath), ".watts-*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, artifactPath)
}

func WriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func Digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
