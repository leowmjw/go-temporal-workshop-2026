# Standalone Activities

**Introduced:** Go SDK support GA September 2026.

Open `/scenario/standalone`.

## What was harder

One-off jobs were often wrapped in a workflow, a scheduler, or an application-owned queue.

## What is simpler

The client starts an Activity directly on a task queue and obtains an activity handle for lifecycle operations and results. Because the handle exists, you can pause, unpause, cancel, and terminate a running activity through an RPC — something a workflow-wrapped activity does not expose.

## The demo: quote forge

Pick one of ten quotes and press **Forge it**. The server starts a standalone `ForgeQuoteActivity` that simulates composing the quote in 16 steps and records a heartbeat at every step. The page streams live state over SSE.

Because the activity heartbeats, three things become observable:

| Action | What happens |
|---|---|
| **Pause** | The next heartbeat RPC returns `activity.ErrActivityPaused`; the context is canceled and the activity yields. |
| **Unpause** | A new attempt starts and resumes from `activity.GetHeartbeatDetails` — it does not restart at step 0. Watch the attempt counter. |
| **Cancel / Terminate** | The activity closes as `CANCELED` or `TERMINATED`; the stream stops on the terminal state. |

## Pacing matters

The SDK batches heartbeats at 80% of `HeartbeatTimeout`. The demo uses a 2s heartbeat timeout, so pause is noticed within roughly 1.6s. With a 10s timeout the 16-step activity could finish before a pause was ever observed.

## Run

```bash
mise run dev
```

Then open `/scenario/standalone`. The worker must be running; the page reports `Temporal server is not running` otherwise.

## Verify from the CLI

While an activity is running, copy its activity ID from the page:

```bash
temporal activity describe --activity-id quote-forge-3-...
temporal activity pause    --activity-id quote-forge-3-...
temporal activity unpause  --activity-id quote-forge-3-...
```
