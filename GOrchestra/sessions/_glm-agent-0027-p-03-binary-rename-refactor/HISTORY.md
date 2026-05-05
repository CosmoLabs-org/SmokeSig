---
branch: _glm-agent-0027-p-03-binary-rename-refactor
base: master
status: killed
created: 2026-05-05
archived: 2026-05-05
commits: 2
files_changed: 107
lines_added: 307
lines_removed: 879
review_status: passed
---

# _glm-agent-0027-p-03-binary-rename-refactor

## Summary

Branch killed via `ccs kill` on 2026-05-05.
2 commits, 107 files changed (+307/-879).

## Commits

- `9bae9cb` chore: add quality review results
- `5dc38b2` refactor(cmd): rename binary from smoke to smokesig

## Files Changed

```
.glm-agent-counter                                 |   2 +-
 .gorchestra/fingerprint-cache.json                 |   7 +-
 .review.json                                       |  15 +-
 .version-registry.json                             |   6 +-
 .../files/cmd/schema_extra_test.go                 |   2 +-
 .../files/internal/runner/runner_extended_test.go  |   2 +-
 .../files/cmd/run_extra_test.go                    |   6 +-
 .../files/cmd/init_extra_test.go                   |   2 +-
 .../files/internal/runner/runner_parallel_test.go  |   2 +-
 .../files/internal/runner/runner_watch_test.go     |   2 +-
 GOrchestra/intel/architecture.json                 |  29 ++-
 GOrchestra/intel/status.json                       |   6 +-
 cmd/init_cmd.go                                    |  22 +--
 cmd/init_extra_test.go                             |  34 ++--
 cmd/mcp.go                                         |   2 +-
 cmd/migrate.go                                     |  18 +-
 cmd/root.go                                        |  12 +-
 cmd/run.go                                         |  20 +-
 cmd/run_extra_test.go                              |  22 +--
 cmd/schema.go                                      |   2 +-
 cmd/schema_extra_test.go                           |   2 +-
 cmd/serve.go                                       |  12 +-
 cmd/serve_test.go                                  |   4 +-
 cmd/stress.go                                      |   6 +-
 cmd/validate.go                                    |   6 +-
 cmd/validate_extra_test.go                         |  10 +-
 cmd/validate_test.go                               |   8 +-
 cmd/version.go                                     |   2 +-
 .../brainstorming/2026-05-05-rename-to-smokesig.md | 143 --------------
 .../planning-mode/2026-05-02-backstage-reporter.md | 216 ---------------------
 .../planning-mode/2026-05-05-rename-to-smokesig.md | 196 -------------------
 go.mod                                             |   2 +-
 internal/detector/container.go                     |   2 +-
 internal/detector/templates.go                     |   2 +-
 internal/mcp/handlers.go                           |  28 +--
 internal/mcp/handlers_test.go                      |  20 +-
 internal/mcp/helpers_test.go                       |  16 +-
 internal/mcp/server.go                             |  24 +--
 internal/mcp/server_test.go                        |  12 +-
 internal/mcp/suggestions.go                        |   2 +-
 internal/mcp/types.go                              |   2 +-
 internal/migrate/goss/emitter.go                   |   4 +-
 internal/migrate/goss/translator.go                |   2 +-
 internal/migrate/goss/translator_test.go           |   6 +-
 internal/monorepo/discover.go                      |   8 +-
 internal/monorepo/discover_test.go                 |  12 +-
 internal/monorepo/monorepo_extra_test.go           |  26 +--
 internal/runner/assertion_credential.go            |   2 +-
 internal/runner/assertion_credential_test.go       |   2 +-
 internal/runner/assertion_db.go                    |   2 +-
 internal/runner/assertion_deeplink.go              |   2 +-
 internal/runner/assertion_deeplink_test.go         |   2 +-
 internal/runner/assertion_dns.go                   |   2 +-
 internal/runner/assertion_dns_smtp_test.go         |   2 +-
 internal/runner/assertion_docker.go                |   2 +-
 internal/runner/assertion_file.go                  |   2 +-
 internal/runner/assertion_file_size_test.go        |   2 +-
 internal/runner/assertion_graphql.go               |   2 +-
 internal/runner/assertion_graphql_test.go          |   2 +-
 internal/runner/assertion_grpc.go                  |   2 +-
 internal/runner/assertion_grpc_stub.go             |   2 +-
 internal/runner/assertion_grpc_stub_test.go        |   2 +-
 internal/runner/assertion_grpc_test.go             |   2 +-
 internal/runner/assertion_k8s.go                   |   2 +-
 internal/runner/assertion_kafka.go                 |   2 +-
 internal/runner/assertion_ldap.go                  |   2 +-
 internal/runner/assertion_mongo.go                 |   2 +-
 internal/runner/assertion_mqtt.go                  |   2 +-
 internal/runner/assertion_network.go               |   2 +-
 internal/runner/assertion_ntp.go                   |   2 +-
 internal/runner/assertion_otel.go                  |   2 +-
 internal/runner/assertion_otel_test.go             |   2 +-
 internal/runner/assertion_ping.go                  |   2 +-
 internal/runner/assertion_ping_test.go             |   2 +-
 internal/runner/assertion_reachable.go             |   2 +-
 internal/runner/assertion_reachable_test.go        |   2 +-
 internal/runner/assertion_smtp.go                  |   2 +-
 internal/runner/assertion_test.go                  |   2 +-
 internal/runner/assertion_validation_test.go       |   2 +-
 internal/runner/assertion_version.go               |   2 +-
 internal/runner/assertion_version_test.go          |   2 +-
 internal/runner/assertion_wire_test.go             |   2 +-
 internal/runner/assertion_ws.go                    |   2 +-
 internal/runner/assertion_ws_test.go               |   2 +-
 internal/runner/chain_test.go                      |   2 +-
 internal/runner/lifecycle.go                       |   2 +-
 internal/runner/lifecycle_integration_test.go      |   2 +-
 internal/runner/lifecycle_test.go                  |   2 +-
 internal/runner/prereq.go                          |   2 +-
 internal/runner/prereq_test.go                     |   2 +-
 internal/runner/runner.go                          |   6 +-
 internal/runner/runner_extended_test.go            |   2 +-
 internal/runner/runner_extra_test.go               |  20 +-
 internal/runner/runner_parallel_test.go            |   2 +-
 internal/runner/runner_test.go                     |   4 +-
 internal/runner/runner_watch_test.go               |   2 +-
 internal/runner/skip_test.go                       |   2 +-
 internal/runner/stress.go                          |   2 +-
 internal/runner/stress_test.go                     |   4 +-
 internal/runner/varstore.go                        |   2 +-
 internal/runner/varstore_test.go                   |   2 +-
 internal/schema/merge_test.go                      |  14 +-
 internal/schema/remote_test.go                     |   4 +-
 internal/schema/schema.go                          |  16 +-
 internal/schema/schema_extra_test.go               |  12 +-
 internal/schema/schema_test.go                     |  18 +-
 main.go                                            |   2 +-
 107 files changed, 307 insertions(+), 879 deletions(-)
```
