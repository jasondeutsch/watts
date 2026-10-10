package cli

import (
	"fmt"
	"strings"

	"github.com/jasondeutsch/watts/cli/internal/agents"
	"github.com/jasondeutsch/watts/cli/internal/devtools"
	"github.com/jasondeutsch/watts/cli/internal/settings"
	"github.com/jasondeutsch/watts/cli/internal/setup"
	"github.com/jasondeutsch/watts/cli/internal/tasks"
	"github.com/jasondeutsch/watts/cli/internal/templates"
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

// tree is the command structure shown by `watts help`.
var tree = []*node{
	{name: "template", summary: "Render a YAML template package (-f values file, -o output directory)", run: templates.Template},
	{name: "init", summary: "Set up Watts in this repository (run once): kit, watts.yaml, agent directories, extension", needsRepo: true, run: setup.Init},
	{name: "doctor", summary: "Check prerequisites, the kit, agents and credentials (--live also calls each gateway)", needsRepo: true, run: setup.Doctor},
	{name: "install", summary: "Install Pi and the loop extension", needsRepo: false, runOnEmpty: true, run: setup.InstallAll, children: []*node{
		{name: "all", summary: "Install Pi, apply the config, install the extension and run doctor", needsRepo: true, run: setup.InstallAll},
		{name: "pi", summary: "Install Pi 1.x globally with npm (skipped if 1.x is present)", run: setup.InstallPi},
		{name: "ralph", summary: "Install the pinned pi-ralph-loop into every agent directory", needsRepo: true, run: setup.InstallRalph},
	}},
	{name: "config", summary: "Show and change watts.yaml", needsRepo: true, children: []*node{
		{name: "show", summary: "Print the config (--resolved fills in every agent's defaults)", needsRepo: true, run: settings.Show},
		{name: "set", summary: "Change a setting: watts config set <key> <value>", needsRepo: true, run: settings.Set},
		{name: "apply", summary: "Create or refresh the agent directories from the config (--force regenerates models.json)", needsRepo: true, run: settings.Apply},
		{name: "env", summary: "List, add or remove the environment variables passed into agents: watts config env add NAME", needsRepo: true, run: settings.Env},
	}},
	{name: "agent", summary: "Work with the agents (build, review and your own)", needsRepo: true, children: []*node{
		{name: "pi", summary: "Open Pi interactively as one agent, in its isolated environment (--agent build|review|name)", needsRepo: true, run: agents.Pi},
		{name: "list", summary: "List the agents with their model, identity and directory", needsRepo: true, run: agents.List},
		{name: "env", summary: "Show the environment variable names an agent receives (never values)", needsRepo: true, run: agents.Env},
		{name: "new", summary: "Create a custom agent from the documented example: watts agent new <name>", needsRepo: true, run: agents.New},
		{name: "follow", summary: "Print a readable live feed of an agent's session", needsRepo: true, run: agents.Follow},
		{name: "clean", summary: "Delete the private agent directories (logins, packages, sessions). Needs --force", needsRepo: true, run: agents.Clean},
	}},
	{name: "start", summary: "Start worker and web UI; -d runs detached", needsRepo: true, run: setup.Start},
	{name: "stop", summary: "Stop this project's worker and web UI", needsRepo: true, run: setup.Stop},
	{name: "task", summary: "Create and run tasks", needsRepo: true, children: []*node{
		{name: "new", summary: "Create a task: watts task new <slug>", needsRepo: true, run: tasks.New},
		{name: "decide", summary: "Approve or reject a pending Temporal human stage: task decide [task] [stage] [--reject] [--feedback text]", needsRepo: true, run: tasks.Decide},
		{name: "run", summary: "Start or continue a task; retry its failed stage", needsRepo: true, run: tasks.Run},
		{name: "status", summary: "Temporal workflow progress and attempt history (--json)", needsRepo: true, run: tasks.Status},
		{name: "cancel", summary: "Cancel the workflow and stop its active subprocesses", needsRepo: true, run: tasks.Cancel},
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
		return resolved{code: 2, errText: fmt.Sprintf("watts: unknown command %q. Run `watts help` for the list.\n", args[0])}
	}
	path, rest := "watts "+n.name, args[1:]
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
		return resolved{code: 2, errText: fmt.Sprintf("watts: %s has no subcommand %q. Run `%s` for the list.\n", path, first, path)}
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
	b.WriteString("watts " + version + ": governed AI coding on Pi 1.0 with pi-ralph-loop\n\n")
	b.WriteString("Usage: watts <command> [subcommand] [flags] [arguments]\n\n")
	b.WriteString("Run `watts init` once in a project folder. Task arguments are folders such as tasks/2026-10-03-http-retry,\nor just the folder name.\n")
	sections := []struct {
		title string
		names []string
	}{
		{"Setup", []string{"init", "doctor", "install", "config"}},
		{"Agents", []string{"agent"}},
		{"Tasks", []string{"task", "start", "stop"}},
		{"Other", []string{"template", "dev", "version"}},
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
	b.WriteString("\nRun `watts <command> -h` for a command's flags, or `watts <group>` for its subcommands.\n")
	return b.String()
}
