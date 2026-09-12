# OpenRPC Linter

[![CI](https://github.com/open-rpc/openrpc-linter/workflows/CI/badge.svg)](https://github.com/open-rpc/openrpc-linter/actions)

Fast, extensible linter for OpenRPC documents.

## Getting started

```bash
go install github.com/open-rpc/openrpc-linter@latest
```

Or, if your project already lives in the npm world:

```bash
npm install --save-dev @open-rpc/openrpc-linter
```

Create a `rules.yml` — or run `openrpc-linter init` to scaffold one that extends `recommended`. A rules file must include **`extends` and/or `rules`** — at least one is required. Both keys are optional individually; omit `rules` to run the inherited set as-is, or omit `extends` for a fully custom ruleset (see [Rules](#rules)).

```yaml
# optional — inherit bundled rulesets (recommended is built in)
extends:
  - recommended

# optional — override severity or add custom rules (see Rules below)
rules:
  info-license:
    severity: ignore
```

Pass it to `lint` with `-r rules.yml` (no default rules path). `recommended` resolves to the bundled ruleset in `rules/defaults/recommended.yaml`.

## Usage

```bash
openrpc-linter --skill                           # print Markdown instructions for agents
openrpc-linter lint openrpc.json -r rules.yml
openrpc-linter init                              # create a basic rules.yml w/ recommmended rules
openrpc-linter lint -r rules.yml -f json          # default path: openrpc.json
openrpc-linter validate openrpc.json              # JSON Schema only
```

Example text output:

```text
openrpc.json

7 errors, 8 warnings found in 8 rules

  path                                     level    message                                                   rule

debug_getBadBlocks
  methods[0].description                   error    missing field 'description'                               method-description
  methods[0].result.schema.description     warning  missing field 'description'                               schema-description
    schema: "BlockObject"

info
  info.license                             warning  missing field 'license'                                   info-license
```

Text output groups violations by method, schema, or top-level section and colors rows on a TTY (`NO_COLOR` / `FORCE_COLOR`). JSON output (`-f json`) is a flat violation list with `path` and `pathLabels`.

## Rules

Define custom rules with `given` (JSONPath) and a built-in function in `then`:

```yaml
rules:
  method-description:
    description: "Methods must have descriptions"
    given: "$.methods[*].description"
    severity: "error"
    then:
      function: "truthy"
```

### Selecting with `given`

The selector turns `given` into one target per matched node. The path shape determines what each function receives:

- **Field mode** — paths ending in a field name (e.g. `$.info.description`, `$.methods[*].summary`): one target per parent object. Missing fields are reported as absent (`Exists=false`).
- **Value mode** — paths ending in a wildcard, index, filter, or collection (e.g. `$.methods`, `$.methods[*].name`): the selected node or value directly.
- **Descendant field mode** — paths with `..` before a terminal field (e.g. `$..schema.description`): like field mode, but uses the OpenRPC meta-schema index to find candidates even when the field is absent.

Point `given` at the thing you want to check: a field path for presence (`truthy`), a value path for shape, length, or uniqueness (`schema`, `unique`).

### `truthy`

Require the selected field to exist and be non-empty. `nil`, `""`, and `"null"` fail. No options.

In field or descendant-field mode, missing fields report `missing field '<name>'`.

```yaml
method-description:
  given: "$.methods[*].description"
  severity: "error"
  then:
    function: "truthy"
```

### `schema`

Validate each selected value against a JSON Schema fragment in `functionOptions`. Any valid JSON Schema keywords work (via [santhosh-tekuri/jsonschema](https://github.com/santhosh-tekuri/jsonschema)). `functionOptions` is the schema itself — no wrapper key.

Missing fields are skipped silently; pair with `truthy` on the same field if you need both presence and shape. Violations report `Value does not match schema: …`.

```yaml
# array length
methods-non-empty:
  given: "$.methods"
  then:
    function: "schema"
    functionOptions:
      type: "array"
      minItems: 1

# string length
method-summary-length:
  given: "$.methods[*].summary"
  then:
    function: "schema"
    functionOptions:
      type: "string"
      maxLength: 120

# array max size
param-count-limit:
  given: "$.methods[*].params"
  then:
    function: "schema"
    functionOptions:
      type: "array"
      maxItems: 4
```

### `unique`

Require primitive values selected by `given` to be distinct within a scope bucket for the rule run. Supported types: string, bool, number, null.

Options:

- `scope` (JSONPath, optional) — partition duplicate tracking by longest-matching scope path. Targets outside all scopes are skipped (not deduped globally). Default: one global bucket.
- `ignoreMissing` (bool, default `true`) — in field-mode paths, skip missing targets; set `false` to treat missing as duplicate `null`.

```yaml
# global uniqueness
unique-method-names:
  given: "$.methods[*].name"
  then:
    function: "unique"

# per-method uniqueness
unique-param-names-per-method:
  given: "$.methods[*].params[*].name"
  then:
    function: "unique"
    functionOptions:
      scope: "$.methods[*]"
```

`ignoreMissing` only matters for `given` paths whose terminal segment is a field name (so the selector can emit missing-field targets).
