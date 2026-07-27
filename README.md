# Event Telemetry Pipeline

A high-throughput ingestion pipeline for autonomous-driving sensor telemetry, built
around a **hand-written TCP connection pool**. It replays real KITTI sensor data,
ingests it through a Go service, queues it with Redis Streams, and persists it to
PostgreSQL and object storage — with full observability, load testing, and a chaos
recovery scenario.

---

## The problem

Autonomous-driving systems emit sensor telemetry continuously and at high frequency.
Ingesting that stream naively — opening a fresh network connection for every event —
does not scale:

- **Connection setup is expensive.** Every new TCP connection costs a full handshake
  round-trip before a single byte of data moves. At high event rates, this setup cost
  dominates and throughput collapses.
- **Connections are a finite resource.** Each open connection consumes a file
  descriptor and kernel memory on both ends. Unbounded connection growth exhausts
  those resources and takes the service down.

The result is a system that is both slow and fragile under exactly the load it was
built to handle.

## The solution

A **connection pool implemented from scratch** — no ORM, no `database/sql`-style
abstraction, just the primitives. The pool maintains a bounded set of established
connections and hands them out on demand:

- **Amortized setup cost** — the handshake is paid once per pooled connection, then
  reused across many events instead of once per event.
- **Bounded resource usage** — the pool caps the number of live connections by design,
  so file-descriptor and memory usage can never run away.
- **Explicit backpressure** — when every connection is in use, the pool applies a
  defined policy (wait, time out, or reject) rather than failing unpredictably.

This pool is the core of the system; the surrounding components are intentionally kept
straightforward so the pool's behavior is easy to reason about.

---

## Architecture

```
KITTI replay (Python)                                                  Monitoring
      │                                                          Prometheus + Grafana
      │  telemetry events                                                 ▲
      ▼                                                                   │
Ingestion service (Go)  ──►  Redis Streams  ──►  Processing worker (Go)  ─┤
   hand-built                  (queue)             │                      │
 connection pool                                   ├──►  PostgreSQL
                                                   └──►  Object storage
```

- **KITTI replay simulator (Python)** — replays real recorded autonomous-driving
  sensor data as a live telemetry stream.
- **Ingestion service (Go)** — receives the stream through the hand-built connection
  pool and publishes events onto the queue.
- **Redis Streams** — durable queue decoupling ingestion from processing.
- **Processing worker (Go)** — consumes the queue and persists events to PostgreSQL
  and object storage.
- **Prometheus + Grafana** — metrics and dashboards, with a meaningful alert.

## Tech stack

Go · Python · Redis Streams · PostgreSQL · Prometheus · Grafana · k6 · Docker · Kubernetes

---

## Validation

- **Load test (k6)** — quantified throughput and latency under sustained load.
- **Chaos scenario** — the processing worker is killed mid-processing to verify the
  pipeline recovers without data loss.

## Status

In active development. See [`docs/`](docs/) for detailed progress, design decisions,
and rationale.

## Future work

Scope intentionally bounded for this iteration. Candidate extensions: Kafka for the
queue layer, GPU-accelerated processing, CARLA-based simulation, Terraform-managed
cloud deployment, a custom load balancer, and additional failure scenarios.
