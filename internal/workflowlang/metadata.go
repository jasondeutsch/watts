package workflowlang

// Metadata travels with a task without embedding its executable assets twice.
type Identity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
	Source  string `json:"source"`
}
type Metadata struct {
	SourcePaths []string   `json:"source_paths,omitempty"`
	Name        string     `json:"name"`
	Version     string     `json:"version"`
	Digest      string     `json:"digest"`
	Layers      []Identity `json:"layers"`
}
