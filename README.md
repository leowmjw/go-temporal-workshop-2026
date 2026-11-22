# Temporal Go Workshop 2025–2026

A local-first workshop for Go developers exploring major Temporal capabilities released or promoted in 2025 and 2026.

The workshop uses the **Temporal CLI development server**, Go SDK **v1.51.0**, Go **1.27**, structured `slog`, the standard-library HTTP router, DataStar v1.0.5, and the Temporal built-in test suite.

## What is demonstrated

| Capability | Status | Simplification demonstrated |
|---|---|---|
| Update-With-Start | Server GA, June 2025 | Start or update a workflow and wait for the result in one idempotent call instead of start-then-poll plus an application outbox |
| Standalone Activities | Go SDK GA, September 2026 | Start durable work directly instead of wrapping it in a short-lived workflow or a custom queue |
| Nexus | GA, March 2025 | Typed cross-namespace/cross-team operations |
| Dynamic Workflows and Activities | Go SDK, July 2025 | Name-based fallback handlers instead of large wrapper registries |
| External Storage / Claim-Check | Public preview, 2026 | Move large payloads outside Event History and pass a small reference |
| Worker Versioning | Preview in 2025; verify Cloud/status before demo | Safer deployment routing and pinned executions |
| Poller / resource autoscaling | GA/preview per rollout | Less fixed-poller tuning and capacity engineering |

Cloud-only capabilities are indexed in the UI for future advanced demos, but are not falsely represented as self-contained local demos:

- Multi-region and same-region replication
- Managed High Availability failover
- SCIM provisioning
- Cloud API-key authentication
- Serverless Workers

## Start

```bash
mise trust
mise install
mise run setup
mise run dev
```

Open <http://127.0.0.1:8080>. Temporal Web UI starts at <http://127.0.0.1:8233>.

Environment values are supplied by `mise.toml` and can be overridden by the shell.

## Mise tasks

| Task | Purpose |
|---|---|
| `mise run setup` | Create local data and download modules |
| `mise run dev` | Start Temporal, worker, and HTTP UI with Overmind |
| `mise run fmt` | Run `gofmt` and `go fix` |
| `mise run test` | Race-enabled unit/integration tests |
| `mise run test-timing` | Repeat timing-sensitive tests |
| `mise run build` | Build every package |
| `mise run e2e` | Start standalone Temporal and exercise the HTTP + worker stack |

Use the shell environment through mise:

```bash
mise exec -- go version
mise exec -- temporal --help
```

## Outbox demo boundary

Update-With-Start can remove an application outbox **for the sync-head/async-tail workflow-led pattern**: Temporal's Event History becomes the durable trigger log and Workflow retries become the durable relay. It does not make a separate database transaction and a Temporal Update atomic. If the command must update an existing relational database and publish the same commit atomically, retain the database outbox or move the authoritative mutation into the workflow.

The workshop therefore has separate architecture sections for workflow state and database state rather than an unsafe “both systems in one transaction” claim.

## Testing

```bash
mise run test
mise run test-timing
mise run e2e
```

- Unit tests use `test suite` anonymous callbacks and the SDK test activity environment.
- Race tests use `-race`.
- Timing tests repeat delayed callbacks.
- End-to-end uses the Temporal CLI standalone server and real worker/client processes.

## Scenarios

- [Update-With-Start](docs/scenarios/update-with-start.md)
- [Standalone Activities](docs/scenarios/standalone-activities.md)
- [Nexus](docs/scenarios/nexus.md)
- [Dynamic Workflows and Activities](docs/scenarios/dynamic-registration.md)
- [External Storage](docs/scenarios/external-storage.md)
- [Worker Versioning](docs/scenarios/worker-versioning.md)
- [Cloud-only roadmap](docs/scenarios/cloud-roadmap.md)
