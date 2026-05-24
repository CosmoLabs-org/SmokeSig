# Stability Guarantees

SmokeSig follows [semantic versioning](https://semver.org/). After v1.0, the version contract is:

## Stable (breaking changes require major version bump)

- **YAML config schema** — field names, types, and nesting structure in `.smokesig.yaml`
- **CLI flags and commands** — `smokesig run`, `smokesig init`, `smokesig validate`, `smokesig schema`, `smokesig serve`, `smokesig stress`, `smokesig version` and their flags
- **Exit codes** — 0 (pass), 1 (test failure), 2 (config/argument error)
- **Output formats** — `--format terminal|json|junit|tap|prometheus|gha|backstage` structure

## Additive (new fields are backward compatible)

- **New assertion types** — added as new optional fields under `expect:`, always `omitempty`
- **New CLI flags** — optional flags with sensible defaults
- **New output format fields** — additional JSON keys in structured output

## Experimental (may change between minor versions)

- **Wasm plugins** — plugin ABI and `plugins:` config section
- **`-tags` build variants** — `grpc`, `tui` build tags

## Deprecation policy

1. A field or flag is marked deprecated in the next minor release
2. It continues to work for one additional minor release
3. It is removed in the following major release

Deprecated items emit a stderr warning: `"warning: <field> is deprecated, use <alternative>"`.
