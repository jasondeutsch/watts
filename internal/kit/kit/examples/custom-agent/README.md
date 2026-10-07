# Example custom agent

This directory is a complete, working definition of a custom Watts agent. Copy it with
`watts agent new <name>`, which also registers the agent in `watts.json`, or copy it by hand.

An agent is a Pi agent directory (Pi's `PI_CODING_AGENT_DIR`) plus an entry under `agents` in
`watts.json`. Watts runs it with a private HOME and an otherwise empty environment, the same as
the built-in `build` and `review` agents.

## What is in this directory

| File | Who owns it | Purpose |
| --- | --- | --- |
| `README.md` | you | Notes for people. Watts never reads it. |
| `skills/` | you | One folder per skill, each with a `SKILL.md`. Watts copies these into the agent directory every time you run `watts config apply`. |
| `watts.example.json` | you | The `watts.json` entries that point at this directory. It is a reference and is not read by Watts. |

## What Watts writes into the agent directory

These are generated and rebuilt from `watts.json`. Do not edit them by hand; change the
configuration instead.

| File | Source |
| --- | --- |
| `settings.json` | Provider and model selected by the workflow stage, plus thinking level and `pi_settings` from `watts.json`. Anything Pi added, such as installed packages, is kept. |
| `models.json` | Generated from `providers` in `watts.json`, with the Ollama endpoint in `providers.ollama.base_url`. A `models.json` you wrote yourself is never replaced unless you pass `--force`. |
| `.gitignore` | Keeps Pi's local state (logins, sessions, packages) out of git. |

## The three ways to add to an agent

1. Skills. Put a folder with a `SKILL.md` in `skills/` here, or list a directory in `skills_dirs`
   (project wide at the top level of `watts.json`, or for this agent only under its entry).
2. Tool servers (MCP) and other Pi settings. Put them under `pi_settings`, at the top level for
   every agent or under this agent's entry. They are merged into `settings.json`; the keys are the
   ones Pi documents for its settings file.
3. Credentials. Never put a key in any file. Declare the provider under `providers` with
   `api_key_env` (the name of a variable in your shell) or `api_key_command` (a command that prints
   a key, for example to mint a short-lived one). Watts passes the variable through for you.

## Using the agent in a workflow

Add a step to `workflow.steps` in `watts.json` (see `watts.example.json`) and run
`watts task run <task>`. A step with `agent` runs Pi under this agent with the given prompt,
where `{task}` is the task folder. A step with `command` runs a shell command, and a step with
`check` runs one of the kit scripts.
