// Package resourcetemplates contains the starter workflow package shipped with Watts.
package resourcetemplates

import "embed"

// Files is the shared source for scaffolding and the documented base example.
//
//go:embed watts.yaml values.yaml values.schema.json templates/*
var Files embed.FS
