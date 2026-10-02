# Go backend labs

Go 1.24+. From this directory: `go test ./...`, then `go run ./ecommerce-fulfillment-event-processing-lab-go`. Substitute the healthcare or education folder to select the other services. Open `http://127.0.0.1:8000` for the console; use `-addr 127.0.0.1:8001` to change ports.

Event lab: bounded four-event queue, replay-safe event IDs, two-worker drain batches, failure retry and graceful HTTP shutdown. Scheduling: tenant-specific fixture tokens, future appointment validation, overlap rejection, cancellation and audit. Assessment: deterministic scoring, deadline validation, duplicate event rejection and instructor feedback approval.

Implemented with Go's standard library and in-memory state guarded by a mutex. Kafka, PostgreSQL, gRPC/Protobuf and external identity are not configured in these local reference versions. No real patient records are used. Scheduling tokens are `Bearer local-clinic-a` and `local-clinic-b`; the audit endpoint uses `local-operator`. Token roles are development fixtures.

Run `go test -race ./...` on a machine with the Go race toolchain and C compiler. Ordinary tests include concurrent enqueue checks; local verification details are in the repository report.
