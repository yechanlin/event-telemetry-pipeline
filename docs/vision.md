# North-star: what the finished project looks like

Read this whenever the work feels like a tutorial. The toy demos are scaffolding — this
is the real target.

---

## The finished architecture

```
                                          ┌──────────────► Prometheus ──► Grafana
                                          │  (metrics)        (dashboards + alert)
                                          │
  ┌──────────────┐   TCP    ┌─────────────────────┐   ┌──────────────┐   ┌──────────────┐
  │  Simulators   │ ───────► │  Ingestion service   │──►│ Redis Streams │──►│ Processing    │
  │  (Python ×N)  │  events  │  ┌────────────────┐  │   │   (queue)     │   │ worker (Go)   │
  │  KITTI replay │          │  │  THE POOL 🏆   │  │   └──────────────┘   └──────┬───────┘
  └──────────────┘          │  └────────────────┘  │                              │
                             └─────────────────────┘                    ┌─────────┴─────────┐
                                                                        ▼                   ▼
                                                                   PostgreSQL         Object storage
```

Runs with one command (`docker compose up`), and later on Kubernetes.

## End-to-end story

1. Several Python simulators replay real KITTI car telemetry, all at once.
2. The Go ingestion service receives them and uses the hand-built pool to forward events
   to Redis without dialing a fresh connection per event.
3. Redis Streams buffers the events (the queue).
4. The Go worker pulls events off the queue and saves them to PostgreSQL + object storage.
5. Prometheus + Grafana show it all live; one real alert fires if things back up.

A genuine distributed data pipeline — the shape Tesla/Meta/NVIDIA infra teams run.

## The 5 things that make it outstanding (not a tutorial)

1. **It runs for real, one command.** Clone → `docker compose up` → watch real data flow.
2. **Measured performance** (from the k6 load test): sustained throughput [X] events/sec,
   p99 latency [Y] ms, pool cut connection overhead by [Z]% vs. per-request dialing,
   saturates at N connections and sheds load via timeouts. *(Brackets = we measure them.)*
3. **Live observability:** Grafana dashboard of pool utilization, acquire-wait time, and
   timeout counts under load. Point at a graph, explain the pool's behavior.
4. **Proven resilience:** chaos test kills the worker mid-processing → zero data loss +
   auto-recovery (Redis Streams re-delivers unacknowledged events).
5. **Told well:** README with architecture diagram, design-decisions doc, demo video, and
   the `docs/` folder proving understanding, not just assembly.

## Resume bullets we're building toward (XYZ format)

Drafts; `[measured]` placeholders get filled with real numbers we produce.

- **Built a high-throughput telemetry ingestion pipeline in Go** processing **[X]
  events/sec**, centered on a **hand-written TCP connection pool** (acquire/release,
  timeout-based load shedding, health-checked reconnection) that reduced connection-setup
  overhead by **[Z]%** vs. per-request dialing.
- **Instrumented the system with Prometheus/Grafana** (pool utilization, p99 acquire
  latency) and validated resilience via a **chaos test**, achieving **zero-data-loss
  recovery** from mid-processing worker failure using Redis Streams consumer acknowledgment.
- **Load-tested with k6 to [X] events/sec at p99 [Y] ms**, characterizing pool saturation
  and tuning pool size / timeout policy under overload.

Every bracket is a number measured firsthand — which is why each bullet is defensible in
an interview.

## Progress map

```
✅ Real KITTI telemetry flowing Python → Go               (Week 1 — done)
✅ Hand-built connection pool — all mechanics built       (Week 2 — done)
✅ Plug pool into ingestion → real data through the pool  (done)
✅ Ingestion → Redis Streams (over the hand-built pool)   (done)
✅ Worker: Redis → PostgreSQL                             (done — full pipeline works!)
✅ Worker: also write raw payloads to object storage      (done — MinIO/S3, data-lake pattern)
✅ Docker Compose (one-command run)                       (Week 2 — done! whole stack, one command)
⬜ Consumer groups + acknowledgment (crash recovery)       (Week 3, with the chaos test)
⬜ k6 load test → the NUMBERS
⬜ Grafana dashboards + alert
⬜ Chaos test → resilience story
⬜ K8s + README + demo video + final resume bullets
```

The pool — the hardest, most impressive part — is essentially built. The "toy demo"
feeling ends the moment real telemetry flows through the pool.

**Week 2 complete:** the entire pipeline now runs from a single `docker compose up`, with
real KITTI telemetry flowing simulator → ingestion → pool → Redis → worker → Postgres + MinIO.
