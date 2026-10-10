// Package workflowtemplates embeds the base workflow YAML files shipped with Watts.
package workflowtemplates

import "embed"

//go:embed *.yaml
var Files embed.FS
