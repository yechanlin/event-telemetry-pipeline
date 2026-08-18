# Project: Event Telemetry Pipeline with a Hand-Built Connection Pool

## Who this is for
Ye Chan, UCLA CS student (graduating 2027), preparing for backend/systems infrastructure
internship and new-grad applications (Tesla, Meta, NVIDIA, similar). Currently in the MLH
Meta Production Engineering Fellowship, learning Docker, VMs, and Kubernetes in parallel
with this project.

Background: Python, Django, PostgreSQL, AWS, React Native, plus robotics experience
(ROS2, LiDAR, Gazebo) from a Mars Rover team project. No longer has access to the Mars
Rover project or its data/assets — this is a from-scratch build.

Career track: **Backend / Systems Infrastructure Engineering.** Not Full Stack, not ML
Engineer. Genuinely interested in OS internals, kernel behavior, networking, and
concurrency — this project should lean into those.

## THE MOST IMPORTANT RULE: THIS IS A LEARNING PROJECT

**Do not write code, run commands, create files, or make any changes unless I
(Ye Chan) explicitly approve that specific step first.**

For every step:
1. Explain what we're about to build and *why* it exists in the pipeline.
2. Explain the key concepts involved (e.g., what a TCP handshake is doing, why a
   connection pool needs a timeout policy) — teach, don't just deliver.
3. Propose the specific code/command you want to run.
4. Wait for my explicit go-ahead before writing or executing anything.
5. After it runs, explain what happened and why, especially if something failed.

The goal is that I can defend every line of this project — especially the connection
pool — line-by-line in an interview. If you just hand me finished code, the project
is worthless to me. Slow, explained, approved steps only.

## How to explain things to me (writing style)

I'm new to this. Assume I'm a beginner, not that I already know the terms.

- **Plain language first.** Explain the idea in everyday words before any technical
  term. When a technical term is unavoidable, define it simply the first time.
- **Use analogies.** Everyday comparisons (phone lines, waiting in line, etc.) help me
  more than precise definitions.
- **Keep it short and clean.** Short sentences. Small chunks. Don't dump five paragraphs
  of jargon at once. Less is more.
- **One idea at a time.** Build up slowly. Don't stack many new concepts in one message.
- **Check I'm following.** It's fine to pause and make sure something landed before
  piling on the next concept.

If you catch yourself writing something I'd need a CS degree to parse, stop and rewrite
it simpler. I'd rather go slow and understand than move fast and be lost.

## Guiding principle: depth over breadth
The hand-built TCP connection pool is the centerpiece and must be fully understood,
not just working. It's fine — expected, even — for other components to stay simpler
as long as the pool is rock solid and I can explain it unprompted.

## What we're building
An event telemetry pipeline simulating autonomous-driving sensor data ingestion:

- **Python simulator** replaying real KITTI autonomous-driving sensor data
- **Go ingestion service** with a **manually implemented TCP connection pool**
  (acquire/release, timeout handling, exhaustion behavior — no library, no
  `database/sql`-style abstraction; build the primitives ourselves)
- **Redis Streams** for queuing between ingestion and processing
- **Go processing worker** that writes to PostgreSQL and object storage
- **Prometheus + Grafana** observability, with at least one real, meaningful alert
- **One load test** using k6, producing quantified throughput/latency results
- **One chaos scenario**: kill the worker mid-processing, verify recovery
- **Minimal Kubernetes deployment**, intentionally leveraging what I'm learning in
  the Meta PE fellowship in parallel

## Explicitly OUT of scope (README "Future Work" only — do not build these)
- Kafka (Redis Streams is the deliberate choice)
- GPU acceleration
- CARLA (KITTI replay only)
- Terraform / cloud deployment
- A custom load balancer
- A second chaos scenario
- Custom frontend visualization

## Build plan (3 weeks)

**Week 1 — Foundations**
- Build a TCP echo server from scratch (sockets, file descriptors, handshake) purely
  for understanding — this is throwaway/learning code, not part of the final pipeline
- Python simulator that replays KITTI sensor data
- Bare-bones Go ingestion pipe (no pool yet — just prove data flows end to end)

**Week 2 — Core system**
- Hand-built connection pool integrated into the ingestion service
- Go processing worker (reads from Redis Streams, writes to Postgres + object storage)
- Docker Compose wiring the full stack together

**Week 3 — Production polish**
- Prometheus/Grafana setup with one real alert
- k6 load test with quantified results
- Chaos test (kill worker mid-processing, verify recovery)
- Kubernetes deployment
- README with architecture diagram, design decisions, and Future Work section
- Demo video
- Resume bullet drafts (XYZ format) tailored to Tesla / Meta / NVIDIA

## Context
Ye Chan has a referral at at least one target company, so this project's primary value
is **interview technical depth**, not resume screening. Optimize explanations and
step pacing accordingly — this needs to hold up under a "walk me through this design"
conversation, not just look good in a repo.
