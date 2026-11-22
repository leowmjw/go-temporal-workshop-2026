# Dynamic Workflows and Activities

**Introduced:** Go SDK v1.35.0, July 2025.

Open `/scenario/dynamic`.

## What was harder

Generic routing and fallback handlers required custom reflection or a name-to-function switch maintained by the application.

## What is simpler

The worker registers a dynamic workflow and activity implementation once, and handlers receive the requested name and encoded arguments.
