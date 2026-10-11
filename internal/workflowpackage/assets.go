package workflowpackage

import (
	"bytes"
	"fmt"
	"os"
	"path"
)

// Materialize installs the captured assets in content-addressed local directories.
// Existing content must still match; packages are inputs, never writable workspace.
func (pkg Package) Materialize(projectRoot string) error {
	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, layer := range pkg.Layers {
		for _, asset := range layer.Assets {
			target := path.Join(layer.Root, asset.Path)
			if existing, err := root.ReadFile(target); err == nil {
				if !bytes.Equal(existing, asset.Data) {
					return fmt.Errorf("package asset was modified: %s", target)
				}
				continue
			} else if !os.IsNotExist(err) {
				return err
			}
			if err := root.MkdirAll(path.Dir(target), 0755); err != nil {
				return err
			}
			file, err := root.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, asset.Mode)
			if err != nil {
				return err
			}
			_, writeErr := file.Write(asset.Data)
			closeErr := file.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	return nil
}
