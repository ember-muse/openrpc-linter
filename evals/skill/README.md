# Local skill eval

Requires Go and an authenticated Codex CLI. From the repo root:

```sh
go run ./evals/skill
```

The runner sends the skill and task to Codex on stdin. To use another agent,
pass its command and arguments after `go run ./evals/skill`. It builds
this checkout's linter, puts it on PATH, and runs the agent in a temporary workspace.
The agent call uses your configured account and may incur model usage charges.

Checks: the description mentions `pong`, unrelated document fields and the rules
stay unchanged, and lint passes. Review `agent.log` for whether the agent actually
uses the CLI and whether the description is accurate; those checks are manual.
Artifacts remain in the printed temporary directory, including on failure.
