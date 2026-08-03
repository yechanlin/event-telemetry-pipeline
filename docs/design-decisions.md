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
