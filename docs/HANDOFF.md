# Session handoff — continuing on a new machine

This file orients a fresh Claude session (e.g. after switching from Mac to Windows).
Read this first, then the linked docs.

## Read these, in order
1. **[../AGENTS.md](../AGENTS.md)** — the operating rules. **Follow them exactly.** Summary:
   this is a *learning project*; do NOT write code, run commands, or create files without
   Ye Chan's explicit approval of that specific step. Teach first (plain language, analogies,
   one idea at a time), propose, wait for go-ahead, then explain what happened. The hand-built
   TCP connection pool is the centerpiece and must be defensible line-by-line.
   *(Note: `CLAUDE.md` is git-ignored and won't be on this machine — `AGENTS.md` carries the
   same rules.)*
2. **[vision.md](vision.md)** — north-star architecture + the progress map (what's done / next).
3. **[progress-log.md](progress-log.md)** — dated journal; **read the last 2–3 entries** to see
   exactly what was just finished.
4. **[concepts.md](concepts.md)** and **[design-decisions.md](design-decisions.md)** — the
   "why" behind everything (great for interview prep).

## Status as of 2026-08-14 (commit f14e678)
Done and committed:
- ✅ Python (KITTI) simulator → Go ingestion → **hand-built TCP connection pool** → Redis
  Streams → Go worker → PostgreSQL + MinIO (S3-compatible) object storage.
- ✅ Full stack runs via `docker compose up` (5 services, health-check-gated startup).
- ✅ Load test (Go load generator, `ingestion/loadtest/`): **~55K ops/sec, p99 <2ms, ~7×**
  vs. opening a new TCP connection per request. (Localhost benchmark — the 7× ratio is the
  defensible headline.)
- ✅ Consumer groups + acknowledgment (`XReadGroup` + `XAck`), proven with a **chaos test**:
  killed the worker mid-processing, unacked events replayed from the pending list, **zero loss**.

## Immediate next task: Prometheus + Grafana observability (Week 3)
The Microsoft "Cloud & Distributed Backend" role Ye Chan applied to names "observability,
monitoring at scale" — this is the highest-value remaining piece. Plan:
- Add a `/metrics` endpoint to `ingestion/main.go` (Prometheus Go client).
- **Instrument the pool** (`ingestion/pool/pool.go`): connections in-use (gauge), acquire
  waits, timeouts (counters). Go SLOW here — it's the centerpiece; every metric must be
  explainable.
- Add **prometheus** + **grafana** services to `docker-compose.yml` + a `prometheus.yml`
  scrape config.
- Build one Grafana dashboard (pool utilization, p99, throughput) + **one real alert**
  (e.g. pool saturated for N seconds).
Teach the concepts first (what a metric is, what Prometheus scraping does, why services
"expose" numbers) before any code.

## After that
- Minimal **Kubernetes** deployment (ties to Ye Chan's Meta PE fellowship).
- **README** (architecture diagram, design decisions, Future Work), demo video.

## Explicitly OUT of scope (README "Future Work" only)
Kafka, GPU, CARLA, Terraform/cloud, custom load balancer, a second chaos scenario, custom
frontend. (Redis Streams over Kafka is a deliberate decision — see design-decisions.md.)

## Running it on Windows — practical notes
- Use **Docker Desktop for Windows** (WSL2 backend). `docker compose up -d --build` is the same.
- The **simulator runs on the host** (outside Docker) and connects to `localhost:9000`.
  Run it with `python simulator/simulator.py` (Windows uses `python`, not `python3`).
- Host ports the stack uses: **9000** ingestion, **6379** redis, **5434** postgres (mapped to
  container 5432), **9100/9101** MinIO API/console. Free these if something else grabs them.
- Handy checks:
  - `docker compose ps`
  - `docker compose logs worker --tail 20`
  - `docker compose exec redis redis-cli XLEN events`
  - `docker compose exec redis redis-cli XPENDING events workers`
  - Postgres count: `docker exec -it event-telemetry-pipeline-postgres-1 psql -U postgres -d telemetry -c "SELECT COUNT(*) FROM telemetry_events;"`
- `docker compose exec <service>` uses the **service** name (`postgres`); raw `docker exec`
  needs the **full container** name (`event-telemetry-pipeline-postgres-1`).
- Git line endings: if Git warns about CRLF, it's harmless for this project.

## Resume state (context, not a task)
Two strong bullets already shipped: the pool metric (~55K ops/s, p99 <2ms, 7×) and the
zero-data-loss chaos test. Resume already submitted (incl. Microsoft). No fabricated metrics —
every number was measured firsthand.
