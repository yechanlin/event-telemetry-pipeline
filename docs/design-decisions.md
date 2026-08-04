# Design decisions (why we chose X over Y)

The "why" behind our choices. Great for interviews — when someone asks "why did you do
it this way?", the answer is here. Newest at the bottom.

---

## Echo server language: Go (not C)

**Choice:** Build the throwaway echo server in Go.

**Why:** Go is the language we'll use for the real connection pool later, so the
practice carries over directly — same tools, same style. C would show the lowest-level
details more, but it'd be a detour into a language we won't use for the actual project.
Go keeps us focused.

**Trade-off we accept:** Go hides some of the rawest low-level detail. We'll deliberately
peek under the hood at least once so it doesn't stay abstract.

---

## Already-decided project choices (from the plan)

These were set in the project plan, noted here so the reasoning is in one place:

- **Redis Streams, not Kafka** — Redis Streams is simpler and enough for this project's
  scale. Kafka would be heavier than we need.
- **KITTI replay, not CARLA** — replaying a real recorded dataset (KITTI) is simpler and
  more honest than running a full driving simulator (CARLA).
- **No GPU, no Terraform, no custom load balancer, one chaos test** — deliberately kept
  out of scope to stay focused. These live in the README's "Future Work" section.

---

## Simulator → ingestion protocol: newline-delimited JSON over TCP

**Choice:** Each telemetry event is one line of JSON, sent over a raw TCP connection.

**Why:** Reuses everything learned from the echo server (TCP, connections, reading lines).
JSON is human-readable, so the data flow is easy to see while learning. Simple and honest.

**Trade-off we accept (good interview point):** a real high-throughput system would likely
use a compact **binary format like Protobuf** to cut bytes-on-the-wire and parsing cost.
JSON is chosen deliberately for clarity here; the binary option is noted as future work.

## Replay speed: real-world cadence (~10 Hz)

**Choice:** Replay events at KITTI's real recording rate (~10 readings/second) — send an
event, wait ~100ms, repeat.

**Why:** Makes the simulator behave like a real car streaming live telemetry, which is the
whole point of a "simulator." (Week 3's load test will separately blast events as fast as
possible to measure throughput — a different goal.)
 
## KITTI data: real GPS/IMU (OXTS), not synthetic or full sensors

**Choice:** Replay a real KITTI "raw" drive's **OXTS** (GPS/IMU) data — lat, lon, speed,
acceleration, yaw rate, etc.

**Why:** Genuinely real data (keeps the project's story honest), small and clean, and
streams naturally as telemetry. Rejected synthetic data (not real) and full LiDAR/camera
sensors (multi-GB, complex binary parsing that distracts from the pipeline — against the
project's depth-over-breadth principle).

## Simulator in Python, ingestion service in Go: right tool for each job

**Choice:** Write the KITTI simulator in **Python**, the ingestion service and connection
pool in **Go**.

**Why — match the language to the job:**
- The **simulator** is data-wrangling (read files, parse numbers, format JSON) and emits
  only ~10 events/sec — *not* performance-critical. That's Python's sweet spot; quick and
  readable. Using Go here would be over-engineering — spending effort where it doesn't
  matter. It also mirrors real AV/ML pipelines, where replay/glue tooling is usually
  Python and the heavy services are Go/C++/Rust.
- The **ingestion service + pool** is the performance-critical, high-concurrency
  centerpiece — many connections at high throughput. That's exactly what Go
  (goroutines, scheduler) is built for, and where the depth should go.

**Bonus (good interview point):** a Python program talking to a Go program over a wire
protocol (JSON-over-TCP) shows the services are **decoupled by a protocol, not shared
code** — real systems are "polyglot," and each component can be written in whatever
language fits its job.

## Core services in Go, not Rust

**Choice:** Build the ingestion service, pool, and worker in **Go** (not Rust).

**Why Go fits this project:**
- **Concurrency model maps onto the problem.** Goroutines + channels are purpose-built for
  a concurrent network service with a connection pool — the pool *is* a buffered channel.
  In Rust you'd fight `Arc<Mutex<…>>`, lifetimes, and an async runtime (Tokio) for the same
  result.
- **Native language of the ecosystem.** Docker, Kubernetes, Prometheus, Grafana are all
  written in Go — the whole cloud-infra stack this project runs on. Directly valuable for a
  backend/infra career.
- **Keeps focus on systems concepts, not the compiler.** The project's goal is understanding
  pooling/backpressure/concurrency deeply; Go removes language friction so the ideas stay
  front and center. Rust's borrow checker would shift energy to fighting the compiler.
- **Performance isn't the bottleneck.** This is an IO/network-bound pipeline; Rust's raw
  speed edge wouldn't show up at this scale.

**Where Rust would win (and when to pick it):** no garbage collector → predictable, low
latency (hard real-time, HFT, embedded — e.g. Tesla real-time control); compile-time memory
safety without GC (kernels, hot paths — Cloudflare, Discord, parts of AWS, the Linux kernel);
CPU-bound or memory-constrained work. For a *low-latency embedded/real-time* showcase, Rust
or C++ would be more on-target; for a *distributed data pipeline*, Go is ideal.

**Interview one-liner:** "Go's goroutine/channel model maps naturally onto a connection pool,
and it's the native language of the cloud-infra ecosystem the project runs on. Rust would give
stronger latency guarantees by avoiding GC, but at a complexity cost not justified for an
IO-bound pipeline at this scale."

## Talk to Redis over our own pool + hand-written RESP (not go-redis)

**Choice:** The ingestion service sends events to Redis by writing `XADD` commands, encoded
in Redis's wire protocol (RESP) **by hand**, over connections from our **hand-built pool**.
We do *not* use the `go-redis` library on the ingestion side.

**Why:** `go-redis` ships with its **own connection pool**, which would replace our hand-built
one — gutting the centerpiece of the whole project. Keeping our pool and speaking RESP
ourselves turns the story from "a pool talking to a toy server" into **"a hand-built pool
managing real Redis connections and speaking the Redis wire protocol directly."** Much stronger
for interviews, and squarely in the "build the primitives ourselves" spirit.

**Trade-off we accept:** more code — we encode `XADD` in RESP by hand and read Redis's reply —
but that *is* the point of the exercise.

**Note:** the *worker* (consumer side) may still use `go-redis` for consumer-group reads
(`XREADGROUP`), where hand-rolling consumer-group semantics would be pointless complexity
(breadth, not depth). The hand-built pool lives on the *write* path, which stays pure.

## Object storage: MinIO (self-hosted S3), not real AWS S3

**Choice:** Store the raw JSON payloads in **MinIO**, an S3-compatible object store running
as a local container — not real AWS S3.

**Why:** MinIO speaks the **exact same API as S3**, so the code (`minio-go` SDK, `PutObject`,
buckets/keys) is identical to what you'd write against real S3 — but it runs locally, free,
with no AWS account or credentials to manage. Swapping to real S3 later is a config change
(endpoint + creds), not a code change. This keeps the project **fully runnable from `docker
compose up`** with nothing external, which is a core goal. Real S3 would add cloud setup,
cost, and network dependence for zero learning gain here. (Terraform/cloud deploy is
explicitly out of scope — Future Work.)

**Trade-off we accept:** not exercising real-cloud concerns (IAM, regions, egress cost). Noted
as Future Work; the S3-compatible API means the leap is small.

## Save each event to BOTH Postgres and object storage (the data-lake pattern)

**Choice:** The worker writes every event to **Postgres** (a structured row) **and** to
**object storage** (the raw JSON blob) — deliberately keeping two copies.

**Why:** They serve different needs. Postgres gives **queryable structured data** (you chose
the columns up front). Object storage keeps the **untouched raw bytes** so you can
**reprocess** later if you need a field you didn't model — the classic **data-lake** idea
(structured warehouse + raw lake). This mirrors how real AV/telemetry pipelines retain raw
sensor data. (See [concepts.md](concepts.md).)

**Trade-off we accept:** storing the data twice. Cheap and worth it — object storage is
inexpensive, and the raw copy is the safety net against "we should have kept that field."

## Run the whole stack with Docker Compose; gate startup with a healthcheck

**Choice:** Wire all five services (redis, postgres, minio, ingestion, worker) in one
`docker-compose.yml`, and make the worker wait for Postgres via a **healthcheck** +
`depends_on: condition: service_healthy` — not just `service_started`.

**Why:** One `docker compose up` is the "clone → run" promise, and service-name networking
(`redis:6379`, `postgres:5432`) keeps the services decoupled. `depends_on` alone only waits
for a container to *start*, not to be *ready* — which caused a real `connection refused` race
(worker beat Postgres's init). A healthcheck (`pg_isready`) gates the worker until Postgres
truly accepts connections. This is the **liveness-vs-readiness** distinction Kubernetes
formalizes with probes — a direct bridge to the K8s work later. (See [concepts.md](concepts.md).)

**Trade-off we accept:** the simulator stays *outside* Compose (runs on the host, connects to
`localhost:9000`) — deliberate, since real sensors live outside the backend. Also, redis/minio
use only `service_started` (they come up near-instantly); a fully rigorous setup would give
them healthchecks too — noted, not needed to work.

## Load testing: a custom Go load generator, not k6

**Choice:** Measure pool performance with a small hand-written **Go** load generator
(`ingestion/loadtest/`), not **k6** (which the original plan named).

**Why:** k6 speaks **HTTP**, but the ingestion service (and the pool's forward path) speaks
**raw TCP + RESP** — k6 can't drive that without a plugin. A tiny Go client is the right tool
for a raw-TCP service and gives precise control over the one comparison that matters:
**pooled connection reuse vs. dialing a fresh connection per event**, identical otherwise.
(Interview line: "I picked the tool that matched the protocol.")

**What we measured (localhost, Redis in Docker, 20k events, 50 concurrent senders):** pooled
sustained **~55K events/sec at p99 <2ms**, **~7×** the throughput of per-request dialing.
Per-dial also **collapsed under sustained load from ephemeral-port (TIME_WAIT) exhaustion** —
a live demonstration of why connection pools exist. (See [progress-log.md](progress-log.md).)

**Trade-off we accept:** a localhost benchmark overstates *absolute* throughput vs. a real
network; the honest, defensible headline is the **pooled-vs-perdial ratio**, since both ran
under identical conditions.
