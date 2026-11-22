# Nexus

**Introduced:** Nexus GA March 2025.

Open `/scenario/nexus`.

## What was harder

Cross-team or cross-namespace integration depended on custom RPC, duplicated routing, or ad-hoc callbacks.

## What is simpler

Nexus provides service contracts and operation routing while retaining Durable Execution across application boundaries.

## Local scope

The standalone server and workshop worker document and test the Go SDK boundary. Running a real cross-namespace Nexus demo requires configured endpoints and namespaces beyond the default local server setup.
