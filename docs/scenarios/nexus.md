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
| TeamB (handler) | `team-b` | `workshop-team-b` | `UserWorkflow` (USER), `CreditReviewWorkflow` (CREDIT REVIEW), Nexus service `user-directory` |

TeamA's `PaymentWorkflow` charges a user it does not own. Through the endpoint
`workshop-user-directory` it makes two typed calls:

- `get-user` — a quick lookup backed by `UserWorkflow`.
- `review-credit` — a **durable promise** backed by `CreditReviewWorkflow`: a
  multi-stage review that takes several seconds, exposes a `Stage` query, and returns
  approved/declined. TeamA attaches through Nexus and awaits the verdict; the promise
  survives TeamB worker restarts. The operation's workflow ID is deterministic
  (`credit-review-<user>-<amount>`), so repeated calls are idempotent.

TeamB's workshop credit limit is 250 — charge more to watch the promise come back
*declined* and the receipt flip.

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
2. In the TeamA tab, charge a user. The receipt lists each step: the lookup, the
   durable promise, and the verdict.
3. While the review runs (a few seconds), the TeamB tab's polling list shows the
   `CreditReviewWorkflow` executing — the promise being kept in real time.

The page checks the wiring on load and refuses to run the payment when a namespace
or the endpoint is missing, naming what is absent. The worker logs the same reminder
at startup when the team namespaces are absent.

### Go SDK boundary

- Handler: `temporalnexus.NewWorkflowRunOperation` maps each operation to its
  workflow; `worker.RegisterNexusService` serves both from the `team-b` worker.
- Caller: `workflow.NewNexusClient(endpoint, service)` inside `PaymentWorkflow`
  executes the typed operations and awaits them like futures.

## Nexus is not granular RPC

The demo deliberately makes `review-credit` substantive because Nexus operations are
durable promises, not cheap remote calls:

- A Nexus call is a server-mediated pipeline (caller workflow task → schedule → Nexus
  dispatch → handler workflow → completion callback), not an RPC hop. Latency is
  tens-to-hundreds of milliseconds minimum — an activity or query is far cheaper for
  sub-second, frequent reads.
- A workflow may have **30 in-flight Nexus operations**; the next one stalls the
  workflow task. Every operation also adds history events, so chatty use pushes
  toward the 51,200-event history limit and forces Continue-As-New.
- Sync operations must answer the start request within **10 seconds**; anything
  longer must be a workflow-backed async operation — the wrong shape for RPC-style
  chatter.
- On Temporal Cloud, each scheduled operation is a billed Action on the caller
  namespace, and the handler's workflow/activities bill normally on the handler
  namespace — one granular call costs several times an activity's Actions, on two
  namespaces, and counts toward both namespaces' rate limits.

Use Nexus for coarse-grained, long-lived, cross-team operations. Use activities,
queries, or replicated local state for frequent fine-grained reads.
