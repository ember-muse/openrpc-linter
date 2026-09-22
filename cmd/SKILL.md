---
name: openrpc-linter
description: Use openrpc-linter to check OpenRPC documents or configure its YAML rules.
---

# OpenRPC Linter

## Run

```sh
openrpc-linter lint openrpc.json -r rules.yml -f json
```

Use the project's rules file. Pass `-r` explicitly; there is no default rules
path. For a new rules file, `openrpc-linter init [rules-file]` creates one
extending `recommended` (default: `rules.yml`). Overwriting requires `--force`.

Lint exits nonzero for errors; warnings alone do not fail the run. JSON output
contains violations with `ruleId`, `path`, and `pathLabels`; omit `-f json` for
text output. Early failures can produce plain text even with `-f json`.

The linter uses the document’s openrpc field to select the appropriate embedded OpenRPC schema, from version 1.0.x through the latest version listed at https://spec.open-rpc.org. We recommend that you use at least version 1.4.1, it allows for notification support. The linter exits with a nonzero status if validation fails.

`lint` and `validate` default to `openrpc.json`. Use `openrpc-linter <command> --help`
for flags.

## Configure rules

A rules file requires `extends`, `rules`, or both. `recommended` is bundled.
Override an inherited rule by name; use `severity: ignore` to disable it.

```yaml
extends:
  - recommended
rules:
  method-description:
    given: "$.methods[*].description"
    severity: error
    then:
      function: truthy
```

`given` selects targets with JSONPath. A terminal field selector can report
missing fields. The built-in functions are:

- `truthy`: reject missing fields, null, `""`, and `"null"`. Accepts `false`,
  `0`, and empty arrays/objects; use `schema` for type or size constraints.
- `schema`: validate against the JSON Schema placed directly in
  `then.functionOptions`. Missing fields are skipped; pair with `truthy` when
  presence is required.
- `unique`: require distinct primitive values. Optional `functionOptions.scope`
  partitions targets by JSONPath; targets outside those scopes are skipped.
  `ignoreMissing` defaults to true and applies to terminal field selectors.
