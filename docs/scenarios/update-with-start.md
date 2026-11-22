# Update-With-Start

**Introduced:** server GA in June 2025; public SDK support in Go v1.35 and later.

Open `/scenario/update-with-start`.

## What was harder

An application previously started a workflow and then needed separate signal/update handling, retries, and request de-duplication. If the API also needed to wait for an async head operation, teams commonly added polling or an outbox.

## What is simpler

The HTTP handler sends one Update-With-Start request. If no workflow exists, Temporal starts it and applies the update to the first workflow task. If it exists, Temporal applies the update to the existing execution. The caller waits only until the update reaches the completed stage.

## Run

Start the standalone server and worker, then press **Apply +1 update**.

## Code boundary

The workflow stores authoritative state. This is a valid replacement for the outbox trigger pattern. For independent database mutations, keep a SQLite/outbox transaction or make the workflow the authority; Temporal cannot make another system’s transaction atomic with Workflow History.
