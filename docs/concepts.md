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
