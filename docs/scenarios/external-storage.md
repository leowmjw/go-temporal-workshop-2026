# External Storage / Claim-Check

**Introduced:** public preview 2026.

Open `/scenario/external-storage`.

## What was harder

Large payloads copied through Workflow and Activity code could inflate Event History and require application-specific blob handling.

## What is simpler

External Storage keeps large payload references small while SDK/converter integration transfers the real payload separately.
