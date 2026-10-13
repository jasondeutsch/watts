package workflowlang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

func (pkg Package) fingerprint() (string, error) {
	pkg.Digest = ""
	data, err := json.Marshal(pkg)
	if err != nil {
		return "", err
	}
	return digest(data), nil
}

// ReadCompiled verifies the captured artifact and never resolves source modules.
func ReadCompiled(source string) (Package, error) {
	file, err := os.Open(source)
	if err != nil {
		return Package{}, err
	}
	defer file.Close()
	const limit = 128 << 20
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return Package{}, err
	}
	if len(data) > limit {
		return Package{}, fmt.Errorf("compiled artifact exceeds 128 MiB")
	}
	var pkg Package
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&pkg); err != nil {
		return pkg, fmt.Errorf("invalid compiled artifact: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return pkg, fmt.Errorf("compiled artifact must contain one JSON document")
	}
	if err := pkg.Validate(); err != nil {
		return pkg, err
	}
	return pkg, nil
}
func (pkg Package) Validate() error {
	if pkg.SchemaVersion != 1 {
		return fmt.Errorf("unsupported compiled schema version %d", pkg.SchemaVersion)
	}
	if !validResourceName(pkg.Name) || pkg.Version == "" {
		return fmt.Errorf("compiled artifact requires package name and version")
	}
	expected, err := pkg.fingerprint()
	if err != nil {
		return err
	}
	if pkg.Digest != expected {
		return fmt.Errorf("compiled artifact fingerprint does not match; compile the source again")
	}
	seen := map[string]bool{}
	for _, asset := range pkg.Assets {
		if !fs.ValidPath(asset.Path) || asset.Path == "." || !fs.ValidPath(asset.Root) || !strings.HasPrefix(asset.Root, ".watts/packages/") {
			return fmt.Errorf("invalid compiled asset path")
		}
		if asset.Mode & ^fs.FileMode(0777) != 0 {
			return fmt.Errorf("invalid compiled asset mode")
		}
		target := asset.Root + "/" + asset.Path
		if seen[target] {
			return fmt.Errorf("duplicate compiled asset %s", target)
		}
		seen[target] = true
	}
	return pkg.Definition.Validate()
}
