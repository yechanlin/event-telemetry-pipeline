package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ingestion/pool" // benchmark the REAL pool
)

func main() {
	mode := flag.String("mode", "pooled", "pooled | perdial")
	addr := flag.String("addr", "localhost:6379", "Redis address")
	total := flag.Int("n", 20000, "total events to send")
	conc := flag.Int("c", 50, "concurrent senders")
	poolSize := flag.Int("poolsize", 0, "pool size (pooled mode only; 0 = same as -c, no contention)")
	acquireTimeout := flag.Duration("timeout", 5*time.Second, "Acquire timeout (pooled mode only)")
	flag.Parse()

	payload := `{"frame":0,"lat":49.0,"lon":8.4,"speed_mps":13.17,"num_satellites":11}`
	cmd := buildXADD("bench", payload)

	// Pooled mode shares one pool sized to the concurrency, so both modes
	// run the same number of Redis ops in flight — only reuse vs. re-dial differs.
	var p *pool.Pool
	if *mode == "pooled" {
		size := *conc
		if *poolSize > 0 {
			size = *poolSize // deliberately smaller than -c, to force real contention
		}
		var err error
		if p, err = pool.New(*addr, size); err != nil {
			fmt.Println("could not build pool:", err)
			return
		}
	}

	perWorker := *total / *conc
	latency := make([][]time.Duration, *conc) // one slice per worker → no locking

	var wg sync.WaitGroup
	start := time.Now()
	for w := 0; w < *conc; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			lats := make([]time.Duration, 0, perWorker)
			for i := 0; i < perWorker; i++ {
				t0 := time.Now()
				if *mode == "pooled" {
					conn, err := p.Acquire(*acquireTimeout)
					if err != nil {
						continue
					}
					if doOne(conn, cmd) != nil {
						p.Discard(conn)
						continue
					}
					p.Release(conn)
				} else {
					conn, err := net.Dial("tcp", *addr) // fresh connection every event
					if err != nil {
						continue
					}
					doOne(conn, cmd)
					conn.Close()
				}
				lats = append(lats, time.Since(t0))
			}
			latency[id] = lats
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start)

	var all []time.Duration
	for _, l := range latency {
		all = append(all, l...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })

	n := len(all)
	fmt.Printf("mode=%s  events=%d  concurrency=%d\n", *mode, n, *conc)
	fmt.Printf("elapsed=%.2fs  throughput=%.0f events/sec\n", elapsed.Seconds(), float64(n)/elapsed.Seconds())
	fmt.Printf("p50=%v  p95=%v  p99=%v\n", pct(all, 50), pct(all, 95), pct(all, 99))
}

// doOne sends one XADD and consumes its reply (request→response).
func doOne(conn net.Conn, cmd []byte) error {
	if _, err := conn.Write(cmd); err != nil {
		return err
	}
	return readReply(bufio.NewReader(conn))
}

// pct returns the p-th percentile latency from a sorted slice.
func pct(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := p * len(sorted) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// --- tiny RESP helpers (copied from redis.go; package main can't be imported) ---

func buildXADD(stream, data string) []byte {
	args := []string{"XADD", stream, "*", "data", data}
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	return []byte(b.String())
}

func readReply(r *bufio.Reader) error {
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	switch line[0] {
	case '+', ':': // simple string / integer
		return nil
	case '-': // error
		return fmt.Errorf("redis error: %s", strings.TrimSpace(line))
	case '$': // bulk string (XADD returns the new entry ID here)
		n, _ := strconv.Atoi(strings.TrimSpace(line[1:]))
		if n < 0 {
			return nil
		}
		_, err := io.ReadFull(r, make([]byte, n+2)) // data + trailing \r\n
		return err
	}
	return nil
}
