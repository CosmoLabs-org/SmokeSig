# SmokeSig vs Alternatives

How SmokeSig compares to other tools commonly used for smoke testing and infrastructure validation.

## Feature Matrix

| Feature | SmokeSig | Goss | Bats | Terratest | curl + scripts | Testinfra |
|---------|----------|------|------|-----------|----------------|-----------|
| **Config-driven (no code)** | YAML | YAML | Bash scripts | Go code | Shell scripts | Python code |
| **Single binary** | Yes | Yes | Bash + helpers | Go toolchain | System tools | Python runtime |
| **Runtime dependency** | None | None | Bash 4+ | Go 1.18+ | curl, jq, etc. | Python 3, pip |
| **Assertion types** | 40 | ~10 | Unlimited (custom) | Unlimited (custom) | Manual | ~15 |
| **Wire protocol checks** | 9 native (Redis, Postgres, MySQL, Mongo, Kafka, LDAP, MQTT, NTP, Memcached) | None | Manual | Manual | Manual | Some via plugins |
| **HTTP assertions** | Built-in (status, body, headers, timing) | Built-in | Manual | Manual | curl flags | Built-in |
| **SSL/TLS validation** | Built-in (expiry, self-signed) | None | Manual | Manual | openssl commands | None |
| **Docker assertions** | Container, image, compose health | None | Manual | Built-in | docker commands | Built-in |
| **Kubernetes checks** | Built-in (any resource + conditions) | None | Manual | Built-in | kubectl commands | None |
| **DNS resolution** | Built-in | None | dig commands | Manual | dig/nslookup | None |
| **Auto-detection** | 31 project types | None | None | None | None | None |
| **Config generation** | `smokesig init` | `goss autoadd` | None | None | None | None |
| **Output formats** | 7 (terminal, JSON, JUnit, TAP, Prometheus, GHA, Backstage) | 5 (text, JSON, JUnit, TAP, nagios) | TAP | Go test output | Manual | JUnit, JSON |
| **CI annotations** | GitHub Actions native | None | TAP adapters | None | Manual | pytest plugins |
| **Backstage integration** | Built-in | None | None | None | None | None |
| **Retry with backoff** | Built-in | None | Manual | Manual | Manual loops | pytest-retry |
| **Watch mode** | Built-in (fsnotify) | None | entr/watchman | None | Manual | pytest-watch |
| **Monorepo support** | `--monorepo` auto-discovery | None | Manual | Manual | Manual | conftest.py |
| **Performance baselines** | Built-in (`--baseline`) | None | None | Manual | Manual | None |
| **OpenTelemetry** | Trace propagation + verification | None | None | None | None | None |
| **MCP server (AI)** | 7 tools via `smokesig mcp` | None | None | None | None | None |
| **Goss migration** | `smokesig migrate goss` | N/A | None | None | None | None |
| **Conditional skip** | `skip_if` (env, file) | None | `skip` function | `t.Skip()` | `if` statements | `pytest.mark.skip` |
| **Allow failure** | `allow_failure: true` | None | Manual | Manual | Manual | `xfail` |
| **Go templates in config** | `{{ .Env.FOO }}` | Gossfile templating | Variable expansion | Go code | Shell variables | Jinja2 |
| **Config includes** | `includes:` directive | `gossfile:` | `load` | Go imports | `source` | conftest.py |
| **Deep link verification** | Built-in (iOS + Android) | None | None | None | Manual | None |
| **GraphQL introspection** | Built-in | None | Manual | Manual | curl + jq | None |
| **WebSocket testing** | Built-in (stdlib, no deps) | None | websocat | Manual | websocat | None |
| **Credential verification** | Built-in (no value leak) | None | Manual | Manual | Manual | None |

## Detailed Comparisons

### vs Goss

[Goss](https://github.com/goss-org/goss) is the closest tool in philosophy: YAML-based, single binary, designed for server validation. SmokeSig was inspired by Goss and provides a migration path.

**Where SmokeSig goes further:**
- **Application-level assertions.** Goss focuses on OS-level validation (packages, services, files, users, groups, kernels). SmokeSig adds HTTP endpoint checks, database wire protocols, SSL certificates, WebSocket, GraphQL, OTel traces, and mobile deep links. If your smoke test needs to verify "does my API respond?" rather than "is nginx installed?", SmokeSig is purpose-built for that.
- **Project auto-detection.** `smokesig init` scans your project and generates a config tailored to your stack. Goss has `autoadd` for system resources but nothing for application-level scaffolding.
- **Output format breadth.** Prometheus metrics exposition, GitHub Actions annotations, and Backstage integration are native in SmokeSig.
- **Observability integration.** OTel trace propagation through smoke tests is unique to SmokeSig.
- **AI integration.** The MCP server lets AI agents generate and run smoke tests programmatically.
- **Migration.** `smokesig migrate goss goss.yaml` converts your existing Goss config, mapping core resource types (process, port, command, file, HTTP, package, service) to native SmokeSig assertions.

**Where Goss is stronger:**
- OS-level resource validation (packages, users, groups, kernels, mounts) is Goss's core domain. SmokeSig focuses on application and infrastructure smoke tests rather than system configuration auditing.
- `goss autoadd` can discover running system resources (processes, ports, users) and generate tests from the live state.

### vs Bats

[Bats](https://github.com/bats-core/bats-core) (Bash Automated Testing System) is a TAP-compliant testing framework for Bash. It is a general-purpose shell test runner, not a smoke testing tool specifically.

**When to choose SmokeSig:**
- You want declarative config, not imperative scripts. SmokeSig tests are data (`expect: http: {url: ..., status_code: 200}`), not code (`curl -sf $URL; [ $? -eq 0 ]`).
- You need protocol-level assertions. Testing Redis, Postgres, Kafka, or MongoDB connectivity in Bats means installing client tools and writing shell commands. SmokeSig does it natively.
- You need structured output. SmokeSig outputs JSON, JUnit, Prometheus, and more. Bats outputs TAP, and structured reports require additional tooling.
- You need retry, baselines, or watch mode built in rather than scripted around the test runner.

**When to choose Bats:**
- Your tests are fundamentally shell scripts that need full Bash expressiveness (loops, conditionals, complex pipelines).
- You already have a large Bats test suite and the migration cost is not justified.

### vs Terratest

[Terratest](https://github.com/gruntwork-io/terratest) is a Go library for writing automated tests for infrastructure code. It requires writing Go test functions.

**When to choose SmokeSig:**
- You want zero-code smoke tests. SmokeSig is YAML config, Terratest is Go code. The skill floor is dramatically different.
- Your team includes non-Go developers who need to author and maintain smoke tests.
- You need fast iteration. Editing YAML and running `smokesig run` is seconds. Compiling Go test files adds friction.
- You want built-in output formats, watch mode, retry, and baselines without writing that plumbing yourself.

**When to choose Terratest:**
- You need deep infrastructure testing (provisioning Terraform, waiting for resources, running complex validation logic). Terratest's strength is end-to-end infrastructure lifecycle tests, not smoke checks.
- Your tests require complex setup/teardown orchestration that goes beyond what declarative config can express.
- You are already a Go shop and prefer tests as code.

### vs curl + scripts

Ad-hoc shell scripts with curl, jq, and conditional logic are the most common "smoke test" approach. They work until they don't.

**SmokeSig replaces the script approach because:**
- **Consistency.** Every project uses the same config format. New team members read `.smokesig.yaml`, not `scripts/smoke-test.sh` with its unique conventions.
- **Protocol breadth.** Checking Redis in a shell script means installing `redis-cli`. Checking Postgres means `psql`. SmokeSig handles 9 database/messaging protocols with zero client tools.
- **Structured output.** Shell scripts produce text. SmokeSig produces JUnit for CI, JSON for dashboards, Prometheus for monitoring, GitHub Actions annotations for PR feedback.
- **Retry and resilience.** Exponential backoff with configurable retry counts is a YAML field, not a hand-rolled `while` loop.
- **Maintenance.** YAML configs are readable, diffable, and reviewable. Shell scripts accumulate special cases and silent failures.

### vs Testinfra

[Testinfra](https://github.com/pytest-dev/pytest-testinfra) is a pytest plugin for testing the state of servers configured by Ansible, Salt, Puppet, and other configuration management tools.

**When to choose SmokeSig:**
- You do not want to install or maintain a Python runtime on your CI runners or target machines.
- You want a single binary with no dependencies. Testinfra requires Python 3, pip, and pytest.
- You need application-level assertions (HTTP endpoints, databases, SSL, WebSocket, GraphQL, OTel) rather than server state validation (packages, services, files, sockets).
- You want config-driven tests that non-developers can author.

**When to choose Testinfra:**
- You are in a Python-centric infrastructure team and want tests as pytest functions.
- You use Ansible/Salt/Puppet and want to test the resulting server state with Python assertions.
- You need SSH-based remote execution to test machines you cannot install tools on.

## Summary

SmokeSig occupies a unique position: **config-driven like Goss, but application-aware rather than OS-aware.** It is the only tool that combines zero-code YAML configuration with 40 assertion types spanning HTTP, databases, messaging, observability, mobile, and Kubernetes — all in a single dependency-free binary.

If your question is "does my application turn on and connect to its dependencies?", SmokeSig is the answer.
