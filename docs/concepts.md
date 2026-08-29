# Concepts (in plain words)

Simple explanations of ideas as I learn them. No jargon dumps — everyday language and
analogies. Newest concepts added at the bottom.

---

## Connection = a phone line

Two programs that want to talk over a network open a **connection**. Think of it like
a **phone line** between them: once it's open, they can send messages back and forth.

## Why opening a connection is slow

Opening a connection is like **dialing a phone**: you dial, it rings, the other side
picks up — *then* you can talk. That setup takes time. If you hung up and re-dialed for
every single sentence, you'd spend all your time dialing and barely any time talking.

## Connection pool = keep a few lines open and reuse them

My simulator sends *lots* of small messages very fast. Re-dialing for each one would be
painfully slow. A **connection pool** fixes this: keep a small handful of phone lines
already open, share them, and reuse them. You "dial" a few times at the start, then
reuse those lines over and over.

Building this pool by hand is the centerpiece of the project.

## Echo server = a practice toy

Before building the real pool, we build an **echo server**: the simplest program that
uses a connection. You send it a word, it sends the same word back — like shouting into
a canyon and hearing the echo. It's throwaway practice code, just to see a connection
work with our own hands before building the real thing.

## Where do connections physically live? (RAM, not disk)

When we talk about a "connection," a "listener," or a "port" — where do those actually
exist in the computer?

**Short version:** almost everything lives in **RAM**, managed by the **operating
system (OS)**. The **network card** is the only extra hardware involved — it moves the
actual bytes on and off the wire. The **disk is not involved** unless we deliberately
save data to a file.

Part by part:

- **The program's code** — starts on **disk** (it's a file), gets loaded into **RAM**
  when it runs. A running program lives in RAM.
- **The connection / listener** — lives in **RAM**, in a protected area the OS owns
  (kernel memory). A connection isn't a physical object; it's **bookkeeping the OS
  keeps in memory**: who's connected, what's been sent, what's waiting.
- **Buffers (waiting messages)** — when data arrives, the bytes sit in a small holding
  area in **RAM** until the program reads them.
- **A port (e.g. 9000)** — *not hardware at all.* It's just a **number**, a label the
  OS uses to know which program a message is for. Exists only as a value in RAM.
- **The network card (NIC)** — the one piece of real hardware in the story. Bytes
  physically enter and leave the computer through it.
- **The CPU** — does all the work: running the program and running the OS bookkeeping.

**Analogy:** the OS is a **hotel front desk**. A connection is a **guest's record in
the ledger** (RAM). The port is the **room number** on that record (a label). Waiting
messages sit in a **mail cubby** (buffer, RAM). The only physical door to the street is
the **network card**. The **CPU** is the staff doing the work. The storage room (disk)
is untouched unless someone files something permanently.

**Why this matters:** each connection is a real chunk of the OS's memory and resources.
That's exactly why the connection pool matters — we don't want to waste them.

## `defer` + Ctrl+C: when does cleanup actually run?

`defer someCleanup()` schedules cleanup to run when the surrounding **function ends
normally** (either reaches the end / a `return`, or panics). Think of it like setting a
reminder the moment you enter a building: "on the way out, turn off the lights."

**The twist:** a hard kill — like pressing **Ctrl+C** to stop a program — does *not*
run deferred cleanup. Ctrl+C sends a "kill" signal that stops the program immediately,
skipping `defer` entirely.

**So does the resource leak?** No. When *any* program dies, the **operating system
automatically reclaims everything it was using** — closes its connections, frees its
ports, releases its memory. The OS is the ultimate cleanup crew. (Connections/ports are
just OS bookkeeping in RAM, so when the program dies the OS throws that bookkeeping away.)

**Example from our echo server:**
- `defer listener.Close()` in `main` → **rarely runs**, because `main` loops forever and
  only stops via Ctrl+C. The OS cleans up the port instead.
- `defer conn.Close()` in `handle` → **runs every time**, because `handle` returns
  normally each time a client disconnects.

Same keyword, opposite real-world behavior — knowing *why* shows deep understanding.

## "Server" = a role (software), not necessarily a machine (hardware)

The word "server" has **two meanings**:

1. **Hardware server** — a physical computer (often in a data center, no monitor, has
   CPU/RAM/disk/OS). Built to run all the time.
2. **Software server** — a *program* whose job is to **wait for requests and respond**.

Our "echo server" is meaning #2: a **program**, running on my own laptop — not the cloud.

**Server and client are roles**, like in a restaurant:
- **Server** (waiter) = the one who waits for you to order and brings what you asked for.
- **Client** (customer) = the one who asks.

Our echo program plays the waiter role; the `nc` (netcat) tool plays the customer role.
For local testing, both run on the same laptop talking to itself — that's what
**`localhost`** means: "this very computer."

A software server runs *on* a computer, which could be a data-center machine *or* a
laptop. The program doesn't care. Later, deploying to Kubernetes just moves the same
server program onto a data-center machine instead of my laptop.

## Goroutines and how Go spreads work across CPU cores

`go handle(conn)` runs `handle` as a **goroutine** — a lightweight worker. It does
**not block**: the main loop fires it off and immediately moves on, loops back to
`Accept()`, and is ready for the next client. That's how one server handles many
clients at once. Goroutines are **concurrent**, and Go can run them in *genuine
parallel* across CPU cores (unlike languages such as JavaScript that only fake it by
switching fast on one worker).

**How it actually works — three layers:**

1. **Goroutines (many — could be thousands).** Every `go ...` makes one. Lightweight,
   cheap, tiny bit of memory.
2. **The Go scheduler (the manager).** Built into every Go program automatically.
   Takes all the goroutines and decides which run right now on the real workers.
3. **OS threads (few — roughly one per core).** The real workers the OS provides,
   usually about as many as there are CPU cores. The **cores** are the hardware.

Flow: *thousands of goroutines → Go scheduler shuffles them onto → a handful of OS
threads → which run on the CPU cores.* Go does **not** put one goroutine per core; the
scheduler juggles many cheap goroutines onto a small pool of real threads.

**Kitchen analogy:** CPU cores = the **chefs** (fixed number, the only ones who can
truly cook). Goroutines = the **orders** (could be hundreds). The Go scheduler = the
**kitchen manager** who keeps handing orders to whichever chef is free. You don't hire
one chef per order — a few chefs churn through many orders.

**Why it matters for this project:** because goroutines are cheap, "a goroutine per
connection" is a reasonable design. BUT the *connections* they hold are **not** cheap —
each is real OS bookkeeping, file descriptors, and memory. So even with plentiful
goroutines, we still want to **limit and reuse connections** — which is exactly what the
**connection pool** does. Cheap workers, expensive connections → pool the connections.

## File descriptor = a numbered "ticket" for something the OS opened

A **file descriptor (fd)** is just a **number** the OS hands your program to refer to
something it opened for you. The number isn't the thing — it's a *claim ticket* for it.

**Coat-check analogy:** you hand your coat to the coat check and get ticket **#7** back.
The ticket isn't the coat; it's a claim the coat check (the OS) holds for you. Later you
say "#7" and they fetch it. An fd works the same way.

**"Everything is a file":** Unix/Mac/Linux use the *same* ticket system for open files on
disk, network connections, even the keyboard and screen. That's why it's a *file*
descriptor even for a network connection. **So a connection = a file descriptor.**

**The table:** each program has a table of ticket# → what it points to. Three are always
pre-filled: **0** = keyboard (stdin), **1** = screen (stdout), **2** = errors (stderr).
New things get the next free number: 3, 4, 5, …

**In our echo server (seen live):** the listener took **fd 3**; Go's own internal
machinery (the `kqueue` netpoller on macOS) took **fd 4**; so the first real client
landed on **fd 5**, the second on **fd 6**. Closing a connection frees its number, and a
later connection reuses it.

**Why it matters:** the OS caps how many fds one program can hold at once (often ~1024).
Every open connection burns one. Run out → the server can't open more connections and
falls over. **That finite-ticket limit is the core reason we bound and reuse connections
with a pool** instead of opening endless new ones.

## JSON = the shared "language" two programs use to send data

Inside a program, an event is a live object in memory (a Python **dict**, later a Go
**struct**). You can't send that object across a network as-is, because:

1. **The network only moves text/bytes** — a TCP connection carries a stream of bytes; it
   has no concept of a "Python dict."
2. **Go doesn't understand Python objects** (and vice versa) — different languages, different
   internal formats.

So we need a format that's (a) plain text and (b) understood by both languages. **That's
JSON** — a neutral text format of `{"key": value}` pairs, e.g.
`{"frame": 0, "speed_mps": 13.17, "num_satellites": 11}`.

**The flow:**
```
Python dict  --json.dumps()-->  JSON text  --over TCP-->  JSON text  --parse-->  Go struct
(in memory)   "serialize"      (plain text)              (plain text)  "deserialize"  (in memory)
```
- **Serialize** = flatten an in-memory object into text (`json.dumps` in Python).
- **Deserialize** = rebuild the object from text (parse the JSON on the Go side).

**Flat-pack furniture analogy:** an assembled chair (live object) can't teleport to a
friend. You flat-pack it into a standard box (serialize to JSON), ship it (over the
network), and they reassemble it (deserialize). JSON is the standard box both languages
know how to pack and unpack.

**One object per line:** we end each JSON event with a newline (`\n`) so the receiver knows
where one event ends and the next begins. The Go receiver reads up to the newline
(`ReadString('\n')`, same trick as the echo server) to get exactly one complete event.

**Trade-off:** JSON is chosen for readability/clarity. A real high-throughput system might
use a compact binary format (e.g. Protobuf) to cut bytes and parsing cost. (See
[design-decisions.md](design-decisions.md).)

## Go structs + static typing (reading JSON in Go)

Python is loosely typed — JSON becomes a `dict` and you just write `event["speed_mps"]`.
Go is **statically typed**: it wants the exact shape of the data declared *up front* —
which fields exist and each field's type. You declare that shape with a **struct** (a named
blueprint, like a form with labeled, typed blanks):

```go
type TelemetryEvent struct {
    SpeedMPS      float64 `json:"speed_mps"`
    NumSatellites int     `json:"num_satellites"`
}
```

- **Types** (`float64` = decimal, `int` = whole number) — Go insists on knowing these; the
  compiler then catches mistakes for you. That's the static-typing trade-off: more upfront
  structure in exchange for compile-time safety.
- **Struct tags** (`` `json:"speed_mps"` ``) — bridge the naming styles: Go fields start with
  a capital (`SpeedMPS`, capital = visible to other packages), JSON uses `speed_mps`. The tag
  says "map this JSON key to this field."
- **`json.Unmarshal([]byte(line), &event)`** — deserialize: read JSON text and fill in the
  struct. "Unmarshal" = Go's word for deserialize (the reverse, "Marshal", = serialize).
- **`&event`** — the `&` means "address of." We pass a **pointer** (the location of `event`)
  so `Unmarshal` can write into our actual variable, not a copy.

## Sending over TCP: the client side (Python)

The server (Go) waits; the **client** (Python simulator) dials. In Python:
- **`socket.create_connection((HOST, PORT))`** — dials the server and opens a TCP connection
  (the client role, like `nc` did).
- **`with ... as sock:`** — auto-closes the socket when the block ends (Python's version of
  Go's `defer conn.Close()` — guaranteed cleanup).
- **`.encode()`** — turns a text string into **bytes**, because the wire carries bytes, not
  strings (Python's equivalent of Go's `[]byte(...)`).
- **`sock.sendall(...)`** — pushes all the bytes over the connection.

## How Go reads the data (stream → lines → struct)

The read path in the ingestion service, and the two big "aha"s in it.

**Aha #1: TCP is a stream of bytes, not messages.** Python calling `sendall` 108 times
feels like 108 messages, but TCP has **no message boundaries** — everything arrives as one
continuous ribbon of bytes:
```
{"frame":0,...}\n{"frame":1,...}\n{"frame":2,...}\n...
```
The OS might deliver half an event, or three glued together. So *something* must chop the
ribbon back into events. That's what the **newline `\n`** is for — the delimiter Python put
between events.

**The steps:**
1. **`bufio.NewReader(conn)`** — wraps the raw connection in a **buffered reader** that
   collects incoming bytes and lets us read them conveniently.
2. **`reader.ReadString('\n')`** — reads bytes **until it hits a `\n`**, returning exactly
   one event's JSON text. It handles the messy reality automatically: if only half an event
   arrived it waits for the rest; if three arrived it returns one and buffers the others. So
   you always get exactly one event per call. *This is why the newline delimiter matters.*
3. **`var event TelemetryEvent`** — an empty struct (all fields zero), the container to fill.
4. **`json.Unmarshal([]byte(line), &event)`** — for each JSON key, finds the struct field
   whose `json:` tag matches and stores the value there. `"speed_mps": 13.17` → the field
   tagged `json:"speed_mps"` (`SpeedMPS`) → `event.SpeedMPS = 13.17`.

**Aha #2: the struct tags are the map** from JSON keys to struct fields — without them
`Unmarshal` wouldn't know `speed_mps` belongs in `SpeedMPS`.

**One sentence:** Python sends events as a newline-separated byte stream → `bufio.Reader`
buffers it → `ReadString('\n')` chops off one event's text → `Unmarshal` fills the struct
via the tags → we read the fields like normal variables.

## Redis (and why we use a queue in the middle)

**Redis** is a super-fast data store that keeps everything in **memory (RAM)**, which makes
it blazingly fast. It's used for caching, sessions, real-time counters, and — what we use —
**queues**. The specific feature is **Redis Streams**: a **waiting line** programs drop
items into and pull items out of.

**Why our pipeline needs a queue in the middle:** two sides run at different speeds.
- **Ingestion** (receiving events) is **fast**.
- **The worker** (saving each event to a database + storage) is **slower**.

Connect them directly and ingestion has to wait for the worker on every event → the whole
system crawls at the speed of the slowest part, and bursts overwhelm the worker. A queue in
the middle **decouples** them: ingestion drops events in quickly and moves on; the worker
pulls them out at its own pace.

**Restaurant analogy:** waiters take orders fast (ingestion), the kitchen cooks at its own
pace (worker), and between them is a **rail of order tickets** (the queue). Waiters clip a
ticket and move on instead of waiting for each dish.

**The three big wins:**
1. **Decoupling** — the two sides run independently, each at its own speed.
2. **Buffering bursts** — a flood of events piles up safely in the queue.
3. **Resilience** — if the worker **crashes mid-processing**, unfinished events are **still
   in the queue**, not lost. On restart it picks them back up. *This is what makes the chaos
   test work: kill the worker → Redis re-delivers unacknowledged events → zero data loss.*

**Why Redis specifically:** fast, simple, widely used, and Redis Streams gives durability +
**acknowledgment** (the worker acknowledges an event only after it's safely saved — enabling
crash recovery). Chosen **over Kafka**, which is heavier and more complex than we need.
(See [design-decisions.md](design-decisions.md).)

**How it ties to the pool:** the ingestion service's pool holds connections **to Redis**.
Many event-handlers share that small bounded set of Redis connections instead of each
opening its own. (The temporary "sink" server on port 9001 is a stand-in for Redis during
development.)

## "Isn't Redis just a cache?" — cache vs. queue

Redis is famous as a **cache**, but that's just its most popular use — not what it *is*.
Redis is a general-purpose in-memory store with many data structures (cache, queue,
sessions, rate limiter, leaderboards…). We use its **Streams** feature as a **queue**.

**Cache and queue solve opposite problems:**

| | Cache | Queue (what we use) |
|---|---|---|
| Purpose | Keep a fast copy of data I'll **read again** | Hold a **line of work items** to process |
| Data flow | Same data read **many times** (for speed) | Each item flows through **once** |
| Everyday analogy | Sticky note with a phone number you reuse | Restaurant ticket rail — each ticket cooked once, then cleared |

**Which are we doing? A queue.** Ingestion *drops* each event in; the worker *takes each
event out and processes it once*. We are NOT re-reading the same event for speed (that's a
cache) — we're holding a line of events waiting to be saved.

**Interview line:** "Redis isn't just a cache — it's a general-purpose in-memory store. I'm
using its Streams data structure as a durable queue between ingestion and the worker, not as
a read cache."

## Object storage + the data-lake pattern (why save the SAME event twice)

The worker saves each event to **two** places: a **row in Postgres** *and* the **raw JSON
blob in object storage** (MinIO). That sounds redundant — why keep both?

Because they answer different questions:

- **Postgres (a database)** — structured rows with columns. Great for **queries**:
  "average speed", "events where satellites < 6". But you had to *pick the columns up front*,
  so anything you didn't model is lost.
- **Object storage (MinIO / S3)** — just a giant bucket of files ("objects"), each with a
  name (a "key" like `events/1690…-0.json`). No columns, no schema — it stores the **exact
  original bytes**. Cheap, endless, but you can't run rich queries over it.

**Keeping the raw copy is the "data lake" idea:** store the untouched original so you can
**reprocess it later** — if you realize next month you need a field you didn't put in
Postgres, it's still in the raw blob. The database is the *cooked* data; the lake is the
*raw ingredients* you never throw away.

**Fridge analogy:** Postgres is **meals you prepped into labeled containers** (fast to grab
exactly what you want). Object storage is the **pantry of raw ingredients** (bulky, but you
can cook anything later). Real pipelines keep both.

**MinIO** is software that speaks the **same API as Amazon S3**, but runs on your own
laptop/servers — so we get the real S3 experience locally, free, and could swap to actual S3
later by changing config. (See [design-decisions.md](design-decisions.md).)

## Docker Compose: a dependency graph, not a top-to-bottom script

`docker compose up` does **not** run the YAML file line by line like a script. The **order
services appear in the file doesn't matter.** Compose reads the whole file, builds a
**dependency graph** from the `depends_on` sections ("who needs whom"), then starts things in
**waves — as much in parallel as it can.**

**Cooking analogy:** you don't boil water, *then* chop onions, *then* stir — you boil water
*and* chop onions at the same time, and only drain the pasta once the water has actually
boiled. Compose does everything it can at once, but respects "can't start X until Y is ready."

For our stack: redis, postgres, minio have no dependencies → they start together in wave 1;
ingestion starts once redis has; **worker waits** for all three (its `depends_on`).

## "Started" vs. "healthy" — liveness vs. readiness

`depends_on` can wait two different amounts, and the difference caused a real bug:

- **`service_started`** — "the container has been launched." Docker started the process;
  it does **not** check whether the program inside is actually ready to do work.
- **`service_healthy`** — "the container's **healthcheck** is passing." Compose keeps the
  dependent service waiting until the check succeeds.

**The bug we hit:** the worker depended on Postgres with only `started`. Postgres's container
launched instantly, but a fresh Postgres needs a few seconds to initialize before it accepts
connections. The worker raced ahead, got **`connection refused`**, and exited. (Cleanly —
`Exited (0)` — because our `main()` prints the error and returns, so Compose didn't restart it.)

**The fix — a healthcheck:** give Postgres a tiny command Docker runs on a loop to ask "ready
yet?":
```yaml
healthcheck:
  test: ["CMD-SHELL", "pg_isready -U postgres"]  # Postgres's built-in "are you ready?" tool
  interval: 2s     # ask every 2 seconds
  timeout: 3s      # each check must answer within 3s
  retries: 10      # unhealthy only after 10 failures in a row
```
Then the worker waits on `condition: service_healthy` instead of `service_started`. It now
pauses a couple seconds until Postgres reports **Healthy**, then starts cleanly — no more
`connection refused`.

**Door-open vs. someone-answering analogy:** "started" = the restaurant unlocked its door;
"healthy" = someone's actually at the phone to take your order. Calling the instant the door
unlocks gets you silence.

**Interview line (this is a strong one):** "started vs. healthy is the difference between a
**liveness** signal and a **readiness** signal — the exact idea Kubernetes formalizes with
liveness and readiness probes. I hit the startup race, then fixed it with a Postgres
healthcheck gating the worker's `depends_on`."

## Consumer groups + acknowledgment (not losing work when a worker crashes)

**The problem:** the worker reads an event and saves it — but if it **crashes mid-save**, that
event can vanish, and nothing remembers it was in progress. The old code tracked its position
in an in-memory variable (`lastID`), which is *gone* the instant the worker dies.

**The fix in one sentence:** make the worker say **"done"** to Redis *after* it saves — so if
it crashes before saying "done," Redis still holds the event and hands it back on restart.

**To-do-list analogy:** you only cross off a task *after* you finish it. Pass out mid-task and
the item is **still not crossed off** — so when you wake up, you know to redo it. Nothing is
forgotten.

**The three Redis commands:**
- **`XGROUP CREATE`** — set up the scoreboard (a *consumer group*) on the stream, once.
- **`XREADGROUP`** — read *and claim*: Redis records the events as "given out, not yet done"
  (they go on the **pending list**).
- **`XACK`** — mark an event done, so Redis removes it from the pending list.

**The loop:** `XReadGroup → save to Postgres + MinIO → XAck`. If the worker crashes between
"claim" and "ack," the event stays on the pending list. On restart the worker reads with ID
`"0"` first (its own pending, delivered-but-unacked = the crash leftovers), reprocesses them,
then switches to `">"` (brand-new events).

**Proven with a chaos test:** we delivered 40 events to `worker-1` without acking (`XPENDING`
= 40 — exactly what a mid-save crash leaves behind), restarted the worker, and it replayed
those 40 from the pending list → `XPENDING` = 0, zero loss.

**This is "at-least-once" delivery** — the most common guarantee in production (payments,
orders, telemetry). Servers crash *routinely* (deploys, OOM, rescheduling), so real systems are
built to replay unfinished work. **Honest tradeoff:** a crash *after* save but *before* ack
reprocesses the event (a **duplicate**). Fixing that is **idempotency** (dedup on event ID) —
the approximation everyone uses for "exactly-once." Knowing *which* guarantee a use case needs
(at-most-once for metrics, at-least-once for money/data) is itself a core backend design skill.

**Interview line:** "I used a Redis consumer group with acknowledgment — the worker only acks
after saving to Postgres and object storage. I reproduced a mid-processing crash, saw the
in-flight events stuck as unacknowledged, restarted, and it replayed exactly those from the
pending list. Zero data loss — at-least-once, with idempotency as the next step for dedup."

## Kubernetes = an automated manager, not just "another way to run containers"

Docker Compose runs everything on **one machine**. If a container crashes, *you* have to
notice and restart it. **Kubernetes** is built to run containers across **many machines**
(*nodes*) and constantly enforce "is reality still matching what I was told to keep true" —
restarting a crashed container, spreading copies across machines, routing traffic to whatever's
currently healthy, all **automatically**, without anyone watching.

**Restaurant-chain analogy:** Compose is you personally running one kitchen — you relight a
dead burner yourself. Kubernetes is the chain's operations system: it doesn't cook, but it
walks every kitchen checking "is every station running the way I was told," and fixes drift on
its own.

**Why practice it for a project that doesn't need it:** for 7 containers on one laptop,
Kubernetes is genuine overkill — Compose is completely sufficient. The reason to build it
anyway: real infra teams (Tesla, Meta, NVIDIA, anyone running backend at scale) run Kubernetes
across real machines in production, and the concepts — Pod, Deployment, Service, self-healing —
work **identically** whether the cluster has 1 fake local node or 1,000 real ones. Nobody
learns Kubernetes by starting on a live 50-machine production cluster; everyone practices
locally first. It's a skill this project demonstrates, not a problem this project has.

## The core vocabulary: Node → Pod → Deployment → Service

Four words that build on each other, each solving the gap the previous one leaves:

- **Node** — one machine (real, or in local practice, a Docker container pretending to be
  one) that has CPU/memory and can run things.
- **Pod** — the smallest thing Kubernetes schedules: usually one container, wrapped with a
  little bookkeeping (an internal IP, some labels). One kitchen (Node) can host many trucks
  (Pods) at once — they're different levels, not the same thing.
- **Deployment** — a *standing rule*, not a one-time action: "always keep N copies of this
  Pod running." If one dies, the Deployment notices and creates a replacement, on its own,
  forever. This is the actual self-healing mechanism.
- **Service** — the problem: every time a Pod gets recreated, it gets a **brand-new internal
  IP**. A Service gives a group of Pods **one fixed address that never changes**, and quietly
  routes to whichever real Pod currently exists behind it.

**Company phone number analogy for Service vs. Pod:** the Service is the company's published
phone number — customers always dial the same number, forever. The Pod is the individual
employee's desk extension the call actually gets routed to internally — that extension changes
whenever staff rotates, but callers never see it.

**Proven live, not just described:** deleted the running `redis` Pod directly. The Deployment
noticed the replica count dropped and created a replacement automatically — new name, new IP,
even landed on a *different* node. The Service's IP never moved through any of it.

## Pod IP vs. Service IP — two separate, deliberately distinct address ranges

Kubernetes keeps **two separate virtual IP ranges**, on purpose:

- **Pod range** (e.g. `10.244.x.x`) — the *real* Pod's address, assigned fresh every time
  it's created.
- **Service range** (e.g. `10.96.x.x`) — a completely separate range where nothing actually
  "lives." Kubernetes' internal networking watches which real Pod IP currently backs a Service
  and silently rewrites traffic to it — invisible to the caller.

If the Pod dies and a new one appears with a different IP, the Service IP **doesn't change** —
only the invisible redirect target behind it updates. Keeping the two ranges visually distinct
makes it obvious, just from an IP, whether you're looking at something real-but-changeable or
stable-but-virtual.

## Secrets and ConfigMaps — getting credentials and config files into the cluster

**The problem both solve:** a Docker Compose bind mount (`./prometheus.yml:/etc/prometheus/...`)
assumes "your machine" is one specific place. In Kubernetes, a Pod could land on any node, so
there's no single machine's disk to mount from — the file's *content* has to live inside the
cluster itself instead.

- **ConfigMap** — for ordinary config files (e.g. `prometheus.yml`, Grafana's datasource
  provisioning). Stored in the cluster, mounted into a Pod at whatever path the app expects —
  same end result as the bind mount, different source.
- **Secret** — same idea, but for sensitive values (passwords). Created directly via
  `kubectl create secret ...` rather than written into a committed YAML file, so the real value
  never ends up in git. Referenced from a Deployment via `valueFrom.secretKeyRef` instead of a
  literal `value:`.

**Honest caveat worth knowing:** a Secret's value is **base64-encoded, not encrypted** —
trivially reversible (`echo <value> | base64 -d`). A Secret is really about keeping credentials
out of git-tracked files, not about making them unreadable. Real production setups add real
encryption on top (a vault system, encryption-at-rest).

**Composing a Secret into a larger string:** one env var's value can reference another env var
already defined earlier in the same list, via `$(VAR_NAME)` — e.g. building `POSTGRES_URL`
(`postgres://postgres:$(POSTGRES_PASSWORD)@postgres:5432/...`) from a password sourced from a
Secret, so the composite connection string still never contains the real password in the file.

## Getting a locally-built (not-from-a-registry) image into the cluster

`redis:7-alpine` "just works" on any node because it lives on Docker Hub — a shared location
any machine can pull from. A custom-built image like `event-telemetry-pipeline-ingestion:latest`
only exists in one place at first: the machine that built it.

**Three different answers to "how does Kubernetes get it," each real:**
1. **Shared local storage (what happened here, by luck of setup)** — Docker Desktop's "use
   containerd for pulling and storing images" setting makes Docker and `kind`'s nodes read from
   the *same* image storage. Build the image once with `docker build`, and it's already
   "present" as far as Kubernetes is concerned — `imagePullPolicy: IfNotPresent` finds it
   without ever attempting a pull. Only works because everything is one machine (this laptop).
2. **`kind load docker-image`** — for local testing when the storage *isn't* shared (or the
   image was rebuilt and the cluster's copy is stale): explicitly copies the image from
   Docker's storage directly into each `kind` node container.
3. **A real registry (the actual production answer)** — `docker build` → `docker tag` with a
   registry address (Docker Hub, GHCR, ECR) → `docker push`. Now *any* node, anywhere, including
   real physically-separate machines with no shared disk, can pull it over the network, exactly
   like Redis's maintainers uploaded `redis:7-alpine` once. Private registries add
   `imagePullSecrets` (yet another Secret) for authentication.

**The honest framing:** local development got a shortcut specific to running everything on one
machine. A real multi-machine cluster has no such shortcut — it needs the registry step,
always.

## `kubectl port-forward` — a temporary, one-person tunnel, not a permanent door

A Docker Compose `ports: "9000:9000"` mapping is **permanent** for as long as the container
runs — no separate command needed to "turn on" access. Kubernetes Services don't work that
way: a ClusterIP Service (the default) is only reachable **from inside the cluster** — nothing
outside (like a script running on your own laptop, not as a Pod) can reach it directly.

**`kubectl port-forward svc/ingestion 9000:9000`** bridges that gap, temporarily: opens port
9000 on your machine, tunnels anything that arrives there into port 9000 of the `ingestion`
Service. Same `local:remote` mental model as Compose's `ports:` syntax — just tunneling into a
cluster instead of mapping into one container. **The key difference: it only exists while that
exact command keeps running** — close the terminal, the tunnel's gone. Compose's mapping was
"always on"; this is "on only while I'm actively running it."

**Why the difference exists:** Compose only ever assumes "the outside world" is one laptop, so
a permanent mapping makes sense. Kubernetes is built for real clusters in all kinds of
deployments, so it doesn't hard-code one answer for external access — `port-forward` is just
the simple, always-available option for quick local testing.

**The real, permanent answers, for when many people need to reach in:**
- **`NodePort`** — opens one port directly on every node's real IP, permanently (no command to
  keep running). Simple but clunky — node IPs change, ports are ugly (30000-32767 range).
- **`LoadBalancer`** — the real production standard for public traffic. Kubernetes asks the
  **cloud provider** (AWS/GCP/Azure) to provision a real load balancer with a stable public
  address that spreads traffic across healthy Pods. Requires a real cloud provider behind the
  cluster — a local `kind` cluster has nothing to ask, so this would sit `<pending>` forever
  here.
- **`Ingress`** — one layer above Services, for HTTP/HTTPS specifically: routes different
  domains/paths (`api.company.com`, `dashboard.company.com`) to different Services behind one
  shared entry point, handles TLS in one place. What most real companies use for web traffic.

**Interview line:** "Locally, I used `kubectl port-forward` for testing, since ClusterIP
Services aren't reachable from outside the cluster by design. In real production, that's a
`LoadBalancer` or `Ingress` instead — permanent, for many users, provisioned by the cloud
provider — the same distinction as liveness vs. readiness earlier: a quick manual tool for
development versus the standing, automatic mechanism production actually needs."
