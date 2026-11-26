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

`mise run dev` starts Temporal, creates the `team-a`/`team-b` namespaces and the Nexus
endpoint if they are missing, then starts the worker and UI. The Nexus scenario is
wired from the first run — no extra setup step. The script is idempotent, so re-running
it reuses an already-running Temporal server and existing namespaces.

Every service binds to `0.0.0.0` by default (`WORKSHOP_HTTP_ADDR=0.0.0.0:8080`, Temporal `--ip 0.0.0.0`), so the UI and dev server are also reachable over Tailscale or your LAN. Replace `127.0.0.1` with your machine's Tailscale IP.

Environment values are supplied by `mise.toml` and can be overridden by the shell.

## Mise tasks

| Task | Purpose |
|---|---|
| `mise run setup` | Create local data and download modules |
| `mise run setup-nexus` | Create `team-a`/`team-b` namespaces and register the `workshop-user-directory` Nexus endpoint. Run automatically by `mise run dev`; use it alone to repair wiring without restarting the stack |
| `mise run dev` | Start Temporal, wire Nexus namespaces, then start worker and HTTP UI |
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

## Running the scenarios

Start the stack once, then work through the pages in a browser:

```bash
mise run dev
```

`mise run dev` runs Overmind, which starts three processes from the `Procfile`: the Temporal dev server (`:7233`, Web UI `:8233`), the worker, and the HTTP UI (`:8080`). Stop everything with `Ctrl-C`; Overmind shuts all three down together.

Open <http://127.0.0.1:8080> and use the left navigation.

### Which scenarios are interactive

Three pages drive real Temporal APIs. The rest are reading material that explains
the concept, because they cannot be demonstrated on a local single-node server.

| Page | Interactive | What you do | What proves it worked |
|---|---|---|---|
| `/scenario/update-with-start` | Yes | Press **Apply +1 update** | Counter increments; pressing again updates the *same* workflow, not a new one |
| `/scenario/standalone` | Yes | Pick a quote, **Forge it**, then Pause / Unpause / Cancel / Terminate | Live progress over SSE; unpause resumes at the same step on a higher attempt |
| `/scenario/nexus` | Yes | Press **Charge user over Nexus** | Receipt trace crosses `team-a` → `team-b` |
| `/scenario/dynamic` | No | Read the page | — |
| `/scenario/external-storage` | No | Read the page | — |
| `/scenario/worker-versioning` | No | Read the page | — |
| `/scenario/cloud` | No | Read the page | — |

### 1. Update-With-Start

1. Open <http://127.0.0.1:8080/scenario/update-with-start>.
2. Press **Apply +1 update**. The result shows the workflow state, e.g. `counter: 1`.
3. Press it again. The counter goes to 2 and the notes list grows — one workflow, many updates.

The first press *starts* `demo-counter` and applies the update in the same round trip.
Later presses attach to the existing execution. That is the whole point: no
start-then-poll, no outbox.

Cross-check in the CLI:

```bash
mise exec -- temporal workflow query --workflow-id demo-counter --type Snapshot
```

### 2. Standalone Activities

1. Open <http://127.0.0.1:8080/scenario/standalone>.
2. Choose one of the ten quotes and press **Forge it**.
3. Watch the progress bar and heartbeat counter climb. The activity runs 16 steps.
4. Press **Pause**. Status turns `PAUSED` within ~1.6 s.
5. Press **Unpause**. Note the **Attempt** counter goes to 2 and the step *continues*
   from where it stopped — it does not restart at zero.
6. Start another forge and try **Cancel** or **Terminate**.

Pause is only visible because the activity heartbeats. The SDK batches heartbeats at
80% of `HeartbeatTimeout`; the demo sets a 2 s timeout so a pause is noticed quickly.
With a 10 s timeout the activity would finish before the pause was ever delivered.

Copy the activity ID from the page to inspect it directly:

```bash
mise exec -- temporal activity describe --activity-id quote-forge-3-...
mise exec -- temporal activity pause    --activity-id quote-forge-3-...
mise exec -- temporal activity unpause  --activity-id quote-forge-3-...
```

### 3. Nexus

Two teams, two namespaces: TeamA (`team-a`) owns `PaymentWorkflow`, TeamB (`team-b`)
owns `UserWorkflow` and serves the `user-directory` Nexus service.

The namespaces and endpoint are created by `mise run dev` before the worker starts, so
this scenario works on a fresh checkout with no extra step. The worker logs
`Nexus team workers started` and `workers 3` when it has joined both teams.

1. Open <http://127.0.0.1:8080/scenario/nexus>. The wiring banner is green.
   If it stays orange, it names exactly what is missing.
2. Press **Charge user over Nexus**, then switch to the **TeamB** role to watch the
   inbound runs appear (the page polls every 3 s).

If you started Temporal separately and the banner is orange, repair the wiring:

```bash
mise run setup-nexus
```

The task is idempotent. It creates `team-a`, `team-b`, and the
`workshop-user-directory` endpoint, retrying endpoint creation briefly because the
dev server's namespace registry lags a moment behind creation. After running it
manually, restart the worker (`overmind restart worker`) so it joins the teams.

The receipt's trace is the proof — each line names the namespace that did the work:

```
1. PAYMENT payment-u-101-... started in namespace "team-a" (TeamA)
2. TeamA calls user-directory/get-user through endpoint "workshop-user-directory"
3. USER lookup for u-101 answered by namespace "team-b" (TeamB)
4. TeamA charged 42 credits and kept the receipt
```

Inspect the wiring from the CLI:

```bash
mise exec -- temporal operator namespace list
mise exec -- temporal operator nexus endpoint list
```

### 4. Reading-only scenarios

`/scenario/dynamic`, `/scenario/external-storage`, `/scenario/worker-versioning`, and
`/scenario/cloud` explain capabilities that either need extra server configuration or
are Cloud-only. They are deliberately not faked as local demos. The docs go deeper:

- [Dynamic Workflows and Activities](docs/scenarios/dynamic-registration.md)
- [External Storage](docs/scenarios/external-storage.md)
- [Worker Versioning](docs/scenarios/worker-versioning.md)
- [Cloud-only roadmap](docs/scenarios/cloud-roadmap.md)

### Troubleshooting

| Symptom | Cause and fix |
|---|---|
| Page says `Temporal server is not running` | The worker or server process died. Check the Overmind output, then `mise run dev` again. |
| Pause does nothing, activity completes | You are running an older build; the heartbeat timeout must be 2 s. Rebuild with `mise run build`. |
| Nexus banner stays orange | Wiring is missing or the worker predates it. Run `mise run setup-nexus`, then `overmind restart worker`. |
| Nexus payment errors | The worker is running but has not joined `team-a`/`team-b`. Restart it (`overmind restart worker`). |
| Port 7233/8080 already in use | An older dev server is still up. `overmind quit`, or `pkill -f 'temporal server start-dev'`. |
| Worker exits immediately | Temporal is not reachable yet. Start the stack with `mise run dev` so Temporal comes up before the worker. |

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
