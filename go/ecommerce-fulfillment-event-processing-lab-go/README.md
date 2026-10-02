# Ecommerce Fulfillment Event Processing Lab

Independent Go reference demo using synthetic data. This is newly built demonstration code, not employer source code.

## Purpose

Accept unique order events into a bounded queue. Drain with two workers, retain failed events for retry and reject duplicate IDs with changed content.

## Run

See the family-level README one directory above for the installed runtime and command. Use this folder's `project.json` to select this application.

## Workflow

Domain: **events**. Available commands: enqueue, drain.

The configuration includes request examples. Inspect state and the audit log after a successful command, then retry it or change its version to observe duplicate and concurrency behavior.

## Boundaries

Core workflow demo only. Local fixture authentication and local development storage. External cloud services, live AI model calls and production database/broker integrations from the broader CV descriptions are not configured here. No production deployment, benchmark result or client delivery is implied. Read the family-level README for the actual implemented stack and tests.

## Review the code

[Architecture and failure boundaries](ARCHITECTURE.md). Shared workflow modules and tests are in the parent stack folder. This repository is intended for source review; no hosted application is required.
