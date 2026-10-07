// Package workflowtemplates embeds the base workflow JSON files shipped with Watts.
package workflowtemplates

import "embed"

//go:embed *.json
var Files embed.FS
