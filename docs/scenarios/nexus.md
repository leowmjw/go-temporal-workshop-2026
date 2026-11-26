# Nexus

**Introduced:** Nexus GA March 2025.

Open `/scenario/nexus`.

## What was harder

Cross-team or cross-namespace integration depended on custom RPC, duplicated routing, or ad-hoc callbacks.

## What is simpler

Nexus provides service contracts and operation routing while retaining Durable Execution across application boundaries.

## Local demo

Two teams, two namespaces, one typed contract:

| Role | Namespace | Task queue | Owns |
|---|---|---|---|
| TeamA (caller) | `team-a` | `workshop-team-a` | `PaymentWorkflow` (PAYMENT) |
| TeamB (handler) | `team-b` | `workshop-team-b` | `UserWorkflow` (USER), Nexus service `user-directory` |

TeamA's `PaymentWorkflow` charges a user it does not own. Instead of RPC or a shared
database, it calls the operation `user-directory/get-user` through the endpoint
`workshop-user-directory`. Temporal routes the call to TeamB's worker, which starts
`UserWorkflow` and returns the profile. The receipt shows the trace crossing the
namespace boundary.

### Wire it up

`mise run dev` does this for you: it starts Temporal, waits for it to accept
connections, creates the namespaces and endpoint, and only then starts the worker.
That ordering is required — the worker checks for `team-a`/`team-b` once at boot and
skips the Nexus team workers if they are missing.

If you started Temporal separately, create the wiring yourself:

```bash
mise run setup-nexus
```

The task is idempotent: it creates the `team-a` and `team-b` namespaces and registers
the `workshop-user-directory` endpoint targeting `team-b/workshop-team-b`. If the
worker was already running, restart it (`overmind restart worker`) so it joins the
team namespaces.

Confirm the worker joined by looking for this line in its output:

```
"Nexus team workers started","caller":"team-a/workshop-team-a","handler":"team-b/workshop-team-b","service":"user-directory"
"worker started",...,"workers":3
```

### Try it

1. Open `/scenario/nexus` in two browser tabs — TeamA in one, TeamB in the other.
2. In the TeamA tab, charge a user. The receipt lists each step, including the Nexus
   call and the namespace that answered it.
3. The TeamB tab polls recent `UserWorkflow` runs and shows each inbound call a few
   seconds later.

The page checks the wiring on load and refuses to run the payment when a namespace
or the endpoint is missing, naming what is absent. The worker logs the same reminder
at startup when the team namespaces are absent.

### Go SDK boundary

- Handler: `temporalnexus.NewWorkflowRunOperation` maps `get-user` to `UserWorkflow`;
  `worker.RegisterNexusService` serves it from the `team-b` worker.
- Caller: `workflow.NewNexusClient(endpoint, service)` inside `PaymentWorkflow`
  executes the typed operation and awaits its result like a future.
