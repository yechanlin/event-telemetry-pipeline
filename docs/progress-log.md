# Progress log

A dated journal of what we did and understood. Newest entries at the bottom.
Each entry = one thing we finished. Coming back after a break? Read the last entry.

---

## 2026-07-23 — Project kickoff & design conversation
- Read the project plan and set the ground rules: go slow, explain everything in plain
  language, and don't write code until I approve each step.
- Learned the core idea in simple terms: a **connection** is like a phone line; opening
  one is slow; a **connection pool** keeps a few lines open and reuses them.
  (See [concepts.md](concepts.md).)
- Decided the first thing we build is a throwaway **echo server** (a toy program that
  sends back whatever you send it) just to see a connection work with our own hands.
- Chose **Go** for the echo server. (See [design-decisions.md](design-decisions.md).)
- Set up the documentation structure: a showcase README + this `docs/` notebook.

**Next up:** build the toy echo server — simple version first (send a word, get it back).

---

## 2026-07-24 — Built and ran the echo server (simple version)
- Wrote `scratch/echo-server/main.go`: a from-scratch TCP echo server (~30 lines).
  Opens a listener on port 9000, accepts one connection, reads each line, echoes it back.
- Confirmed Go 1.26.5 installed and working.
- Ran it and tested successfully — sent text via `nc`, got the same text back.
- Understood the code deeply, line by line:
  - Go functions can return **multiple values**; `listener, err := ...` fills two
    separate variables. `err` is `nil` on success, or an error value on failure.
  - `defer` schedules cleanup for when a function ends *normally*; Ctrl+C skips it, and
    the OS reclaims resources anyway.
  - The four steps every server follows: **listen → accept a connection → read → respond.**
  - Cleared up "server" = a software *role* (waits and responds), running on my laptop,
    not the cloud. (See [concepts.md](concepts.md).)

**Next up:** see the "one client at a time" limit — connect two clients and watch the
second one wait — then fix it so the server handles many clients at once.

---

## 2026-07-24 — Saw the "one client at a time" limit, then fixed it with a goroutine
- Tested two clients at once on the simple version: client 1 echoed fine, but client 2
  connected and got **total silence** until client 1 disconnected. Saw the limit live.
- Understood *why*: the single worker got stuck inside `handle(client 1)` and never
  looped back to `Accept()`. The OS accepted client 2 into its waiting line, but the
  program never serviced it.
- Fixed it by changing `handle(conn)` → `go handle(conn)`. Now each client gets its own
  **goroutine**, so the loop is instantly free to accept the next client.
- Re-tested two clients at once — **both echo simultaneously.** Fixed.
- Learned how Go spreads work across CPU cores: the built-in **scheduler** juggles many
  cheap goroutines onto a small pool of OS threads (~one per core). Cheap workers, but
  expensive connections → the reason we'll pool connections later. (See [concepts.md](concepts.md).)

**Next up:** decide what (if anything) else to do with the echo server, then move toward
the real Week 1 goals (KITTI simulator, bare-bones Go ingestion pipe).

---

## 2026-07-24 — Made file descriptors visible (a connection IS an fd)
- Learned what a **file descriptor (fd)** is: a numbered "ticket" the OS gives your
  program for anything it opened (a file, a connection, the screen). A connection = an
  fd. Tickets are finite, which is the core reason we pool connections. (See [concepts.md](concepts.md).)
- Added code to `handle` that prints the real fd for each connection, using
  `SyscallConn().Control(...)` to reach under Go's `conn` wrapper.
- Ran it and connected two clients — saw **fd 5** and **fd 6**.
  - Why 5/6 and not 4/5: fds 0/1/2 = keyboard/screen/errors; fd 3 = the listener;
    fd 4 = Go's own internal machinery (the `kqueue` netpoller on macOS). So the first
    real client lands on fd 5.
- Takeaway: "a connection is a file descriptor" is now something seen on screen, not a
  slogan. Freed fds get recycled by later connections — exactly what a pool exploits.

**Next up:** the echo server has done its job (throwaway learning code). Move toward the
real Week 1 goals — the Python KITTI simulator and a bare-bones Go ingestion pipe.

---

## 2026-07-25 — Design decisions + downloaded real KITTI data
- Made the Week 1 design decisions (logged in [design-decisions.md](design-decisions.md)):
  newline-delimited JSON over TCP, replay at ~10 Hz, real KITTI GPS/IMU (OXTS) data.
- Downloaded one short real KITTI drive (`2011_09_26_drive_0001`, 437 MB) from the
  official KITTI dataset (Amazon S3), extracted **only** the tiny `oxts/` GPS/IMU files
  (108 readings, 440 KB), deleted the big zip. Added `data/` to `.gitignore`.
- Inspected the data: each reading is 30 numbers (see `oxts/dataformat.txt`). Frame 0 =
  real car in Karlsruhe, Germany going ~47 km/h with 11 GPS satellites. Timestamps
  confirm ~10 Hz cadence.
- Useful fields (0-based index into each line): lat 0, lon 1, alt 2, forward velocity 8,
  forward accel 14, yaw rate 22, num satellites 26.

**Next up:** build the Python simulator (read OXTS → JSON events), then the Go ingestion
service (receive + count). Test data flowing end to end.

---

## 2026-07-25 — Week 1 pipeline works end to end (Python → TCP → Go)
- Built the Python simulator (`simulator/simulator.py`): reads the 108 OXTS files in order,
  turns each into a JSON telemetry event, and streams them over a TCP connection at ~10 Hz.
  Verified parsing first in print-only mode (frame 0 speed 13.17, 11 sats), then switched
  to sending over a socket.
- Built the Go ingestion service (`ingestion/main.go`): the echo server "grown up" —
  listen / accept / `go handle` / `ReadString('\n')`, but instead of echoing it parses each
  JSON line into a `TelemetryEvent` struct and prints a running count. Set it up as a
  proper Go module (`go mod init ingestion`).
- New concepts learned:
  - **JSON / serialize-deserialize** — a dict/struct lives in memory; the wire only carries
    text, so we serialize to JSON to send and deserialize on the other side. (See [concepts.md](concepts.md).)
  - **Go structs + static typing** — Go needs the data's shape declared up front; struct
    tags (`json:"speed_mps"`) bridge JSON keys to Go fields; `json.Unmarshal` fills the
    struct; `&event` passes a pointer so it can be written into.
  - **Python socket client** — `socket.create_connection` dials the server (client role),
    `with` auto-closes (like Go's `defer`), `.encode()` turns text into bytes for the wire.
- Ran both together: all 108 events flowed Python → Go, ~10/sec, ending with
  `simulator disconnected after 108 events`. **Week 1 goals done.**

**Next up (Week 2 — the centerpiece):** the hand-built TCP connection pool, integrated into
the ingestion service. This is the core of the whole project.

---

## 2026-07-30 — Built the connection pool (centerpiece) and integrated it — data flows through it
- Learned **channels** (Go's thread-safe pipe between goroutines) and the "buffered channel
  = bowl of keys" model: receiving blocks when empty (backpressure), safe under concurrency,
  capacity = the pool's max size. (See [concepts.md](concepts.md).)
- Built the pool step by step, each mechanic understood and demo'd:
  - **Make the bowl + fill it** with real dialed connections (`New`).
  - **Take & return** (`Acquire` / `Release`).
  - **Wait when empty** = backpressure (channel receive blocks). Saw a race between
    goroutines live, and learned `len()` of a shared channel is a fleeting snapshot.
  - **Timeouts** via `select` + `time.After` — a saturated pool sheds load instead of
    hanging forever; `Acquire` now returns an error.
  - **Broken lines** (`Discard`) — close a dead connection and dial a fresh replacement so
    the pool stays full and never hands out a poisoned line.
  - **Many workers at once** — safe by the channel's design.
- Turned the pool into a proper **package** (`ingestion/pool/`, `package pool`, exported
  API: `New`/`Acquire`/`Release`/`Discard`/`Available`). Deleted the throwaway `pool/` lab.
- Built a tiny **downstream sink** (`scratch/sink/`, port 9001) as a stand-in for Redis.
- **Integrated the pool into the ingestion service**: each received event is forwarded
  downstream via `forward()` = borrow a pooled conn → `Write` → `Release` (or `Discard` on
  error). Ran sink + ingestion + simulator together — 108 real KITTI events flowed through
  just **5 pooled connections**, end to end.

**Next up:** replace the sink with real **Redis Streams**, then build the Go **processing
worker** (reads from Redis → PostgreSQL + object storage), then wire it all with Docker Compose.

---

## 2026-08-02 — Wired ingestion to real Redis (over the hand-built pool, hand-written RESP)
- Ran **Redis in Docker** (`docker run -d --name redis -p 6379:6379 redis`) — first real
  container. Learned Docker Hub / images / port mapping, and Redis Streams by hand
  (`XADD` / `XLEN` / `XRANGE` in `redis-cli`).
- Learned the **RESP wire protocol** (length-prefixed array of bulk strings) and the
  cache-vs-queue distinction. (See [concepts.md](concepts.md).)
- **Decision:** keep the hand-built pool and speak RESP ourselves rather than use `go-redis`
  (which has its own pool) — preserves the centerpiece. (See [design-decisions.md](design-decisions.md).)
- Pointed the pool at Redis (`localhost:6379`) and rewrote `forward()` to send
  `XADD events * data <json>` in hand-encoded RESP over a pooled connection, then read
  Redis's reply. New file `ingestion/redis.go`: `buildXADD` (RESP encoder) + `readReply`
  (RESP reply parser).
- Verified end to end: ran ingestion + simulator → **108 real KITTI events stored in the
  `events` stream** (`XLEN` = 108; `XRANGE` shows frame 0 with the exact original values).
- Retired the toy sink — Redis replaces it.

**Next up:** the Go **processing worker** — read events from the Redis stream and save them
to PostgreSQL + object storage, with consumer-group acknowledgment for crash recovery.

---

## 2026-08-02 — Worker reads Redis → saves to PostgreSQL (full pipeline works end to end)
- Built the Go **worker** (`worker/`, its own module): connects to Redis with **go-redis**
  (first outside library — used on the read side per the design decision), reads the `events`
  stream with `XRead`, parses each entry's JSON, and inserts a row into Postgres.
- Ran **Postgres in Docker** (`etp-postgres`, db `telemetry`). Worker creates the
  `telemetry_events` table on startup (`CREATE TABLE IF NOT EXISTS`) and inserts with
  **parameterized queries** (`$1..$9` — safe against SQL injection) via **pgx**.
- Debugged a real **port conflict**: a native Mac Postgres owned `localhost:5432` and shadowed
  our container (error `role "postgres" does not exist` = reached the wrong Postgres). Fixed by
  remapping our container to host port **5434**.
- Verified end to end: **216 rows in `telemetry_events`** — the full path works:
  simulator → ingestion → pool → Redis → worker → Postgres.

**Next up:** worker also writes raw payloads to **object storage**; then **consumer groups +
acknowledgment** (so re-running/crash doesn't duplicate or lose events) for the chaos test;
then **Docker Compose** to run the whole stack with one command.
