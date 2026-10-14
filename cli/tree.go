package cli

import (
	"fmt"
	"strings"

	"github.com/jasondeutsch/watts/cli/internal/devtools"
	"github.com/jasondeutsch/watts/cli/internal/language"
	"github.com/jasondeutsch/watts/cli/internal/settings"
	"github.com/jasondeutsch/watts/cli/internal/setup"
	"github.com/jasondeutsch/watts/cli/internal/tasks"
	"github.com/jasondeutsch/watts/cli/internal/terminal"
)

// node is one command or group of commands. A group has children; a leaf has run.
type node struct {
	name      string
	summary   string
	needsRepo bool
	run       func(a *terminal.Context, args []string) error
	children  []*node
	// runOnEmpty makes a group run its own handler when no subcommand is given, instead of
	// printing its help.
	runOnEmpty bool
}

func (n *node) child(name string) *node {
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

// tree is the command structure shown by `wctl help`.
var tree = []*node{
	{name: "compile", summary: "Compile ACL packages and modules into a frozen JSON artifact", run: language.Compile},
	{name: "c", summary: "Alias for compile", run: language.Compile},
	{name: "exec", summary: "Execute a compiled artifact or continue an existing task", needsRepo: true, run: language.Exec},
	{name: "doctor", summary: "Validate selected manifests, Docker images, and required secret sources", needsRepo: true, run: setup.Doctor},
	{name: "config", summary: "Manage shared home-directory worker preferences", children: []*node{
		{name: "show", summary: "Print shared worker preferences", run: settings.Show},
		{name: "set", summary: "Change a shared setting: wctl config set <key> <value>", run: settings.Set},
	}},
	{name: "start", summary: "Start worker and web UI; -d runs detached", needsRepo: true, run: setup.Start},
	{name: "stop", summary: "Stop this project's worker and web UI", needsRepo: true, run: setup.Stop},
	{name: "task", summary: "Create and run tasks", needsRepo: true, children: []*node{
		{name: "new", summary: "Scaffold a task: wctl task new <slug> --compiled <artifact.json>", needsRepo: true, run: tasks.New},
		{name: "decide", summary: "Approve or reject a pending Temporal human stage: task decide [task] [stage] [--reject] [--feedback text]; delegation: --actor, --authorized-by, --reviewed-status", needsRepo: true, run: tasks.Decide},
		{name: "run", summary: "Start or continue a task; retry its failed stage", needsRepo: true, run: tasks.Run},
		{name: "status", summary: "Temporal workflow progress and attempt history (--json)", needsRepo: true, run: tasks.Status},
		{name: "cancel", summary: "Cancel the workflow and stop its active containers", needsRepo: true, run: tasks.Cancel},
	}},
	{name: "dev", summary: "Tools for developing and testing Watts itself", children: []*node{
		{name: "mock-server", summary: "Run a fake OpenAI-style gateway that returns canned replies (--status 401 to test failures)", run: devtools.MockServer},
	}},
	{name: "version", summary: "Print the version", run: cmdVersion},
}

func findTop(name string) (*node, bool) {
	for _, n := range tree {
		if n.name == name {
			return n, true
		}
	}
	return nil, false
}

// resolved is what a command line turns into: a handler to run, or help text to print.
type resolved struct {
	run       func(a *terminal.Context, args []string) error
	args      []string
	needsRepo bool
	helpText  string
	code      int
	errText   string
}

func isHelpArg(s string) bool { return s == "help" || s == "-h" || s == "--help" }

// resolve walks the command tree.
func resolve(args []string) resolved {
	if len(args) == 0 || isHelpArg(args[0]) {
		return resolved{helpText: usage()}
	}
	if args[0] == "--version" {
		args = []string{"version"}
	}
	n, ok := findTop(args[0])
	if !ok {
		return resolved{code: 2, errText: fmt.Sprintf("wctl: unknown command %q. Run `wctl help` for the list.\n", args[0])}
	}
	path, rest := "wctl "+n.name, args[1:]
	for len(n.children) > 0 {
		if len(rest) == 0 {
			if n.runOnEmpty {
				return resolved{run: n.run, args: rest, needsRepo: n.needsRepo}
			}
			return resolved{helpText: groupUsage(path, n)}
		}
		first := rest[0]
		if isHelpArg(first) {
			return resolved{helpText: groupUsage(path, n)}
		}
		if c := n.child(first); c != nil {
			n, path, rest = c, path+" "+c.name, rest[1:]
			continue
		}
		if strings.HasPrefix(first, "-") && n.runOnEmpty && n.run != nil {
			return resolved{run: n.run, args: rest, needsRepo: n.needsRepo}
		}
		return resolved{code: 2, errText: fmt.Sprintf("wctl: %s has no subcommand %q. Run `%s` for the list.\n", path, first, path)}
	}
	return resolved{run: n.run, args: rest, needsRepo: n.needsRepo}
}

func groupUsage(path string, n *node) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n\nUsage: %s <subcommand> [flags] [arguments]\n\nSubcommands:\n", path, n.summary, path)
	for _, c := range n.children {
		fmt.Fprintf(&b, "  %-14s %s\n", c.name, c.summary)
	}
	fmt.Fprintf(&b, "\nRun `%s <subcommand> -h` for a subcommand's flags.\n", path)
	return b.String()
}

func usage() string {
	var b strings.Builder
	b.WriteString("wctl " + version + ": declarative agent workflows\n\n")
	b.WriteString("Usage: wctl <command> [subcommand] [flags] [arguments]\n\n")
	b.WriteString("Compile a watts.acl package with `wctl compile` (or `wctl c`), then execute its JSON artifact with `wctl exec`.\nTask arguments are folders under tasks/, or just the folder name.\n")
	sections := []struct {
		title string
		names []string
	}{
		{"Authoring", []string{"compile", "exec"}},
		{"Setup", []string{"doctor", "config"}},
		{"Tasks", []string{"task", "start", "stop"}},
		{"Other", []string{"dev", "version"}},
	}
	for _, sec := range sections {
		fmt.Fprintf(&b, "\n%s:\n", sec.title)
		for _, name := range sec.names {
			n, _ := findTop(name)
			fmt.Fprintf(&b, "  %-12s %s\n", n.name, n.summary)
			if len(n.children) > 0 {
				var subs []string
				for _, c := range n.children {
					subs = append(subs, c.name)
				}
				fmt.Fprintf(&b, "  %-12s subcommands: %s\n", "", strings.Join(subs, ", "))
			}
		}
	}
	b.WriteString("\nRun `wctl <command> -h` for a command's flags, or `wctl <group>` for its subcommands.\n")
	return b.String()
}
