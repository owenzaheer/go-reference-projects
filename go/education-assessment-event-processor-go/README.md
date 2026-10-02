# Education Assessment Event Processor

Score a fixed three-answer rubric before a deadline. Reject duplicate submissions. An instructor reviews feedback and publishes it using the expected record version; feedback does not change the score.

## Purpose

Score a fixed three-answer rubric before a deadline. Reject duplicate submissions. An instructor reviews feedback and publishes it using the expected record version; feedback does not change the score.

## Run

See the family-level README one directory above for the installed runtime and command. Use this folder's `project.json` to select this application.

## Workflow

Domain: **education**. Available commands: submit, feedback, publish.

The configuration includes request examples. Inspect state and the audit log after a successful command, then retry it or change its version to observe duplicate and concurrency behavior.

## Configuration

Core workflow demo only. Local fixture authentication and local development storage. External cloud services, live AI model calls and production database/broker integrations from the broader CV descriptions are not configured here. Read the family-level README for the actual implemented stack and tests.

## Architecture

[Architecture and failure boundaries](ARCHITECTURE.md). Shared workflow modules and tests are in the parent stack folder.
