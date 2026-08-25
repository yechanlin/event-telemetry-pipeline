package main

import (
	"bufio"
	"encoding/json" // Go's JSON reader/writer
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ingestion/pool" // our hand-built connection pool
)

// getenv returns the environment variable `key`, or `fallback` if it's unset/empty.
// Lets the same binary run locally (defaults) or in Docker Compose (env overrides).
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getenvInt is getenv, but parsed as an integer -- for tunables like pool size
// and timeout that we want configurable without a code change (e.g. to
// deliberately shrink them for a saturation test, without touching the
// real defaults).
func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

// TelemetryEvent is the shape of the JSON the Python simulator sends.
// The `json:"..."` tags map each JSON key to a field.
type TelemetryEvent struct {
	Frame         int     `json:"frame"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
	Alt           float64 `json:"alt"`
	SpeedMPS      float64 `json:"speed_mps"`
	AccelMPS2     float64 `json:"accel_mps2"`
	YawRateRPS    float64 `json:"yaw_rate_rps"`
	NumSatellites int     `json:"num_satellites"`
}

func main() {
	// Register the pool's metrics, then serve them on a separate HTTP port.
	// Separate from :9000 on purpose — :9000 speaks our raw line-delimited-JSON
	// protocol to simulators, not HTTP; Prometheus needs a normal HTTP endpoint.
	prometheus.MustRegister(pool.InUse, pool.AcquiresTotal, pool.AcquireTimeoutsTotal)
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		fmt.Println("metrics available at :2112/metrics")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			fmt.Println("metrics server error:", err)
		}
	}()

	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		fmt.Println("could not start listening:", err)
		return
	}
	defer listener.Close()
	fmt.Println("ingestion service listening on port 9000")

	// Pool size and acquire timeout are configurable, defaulting to the real
	// production-shaped values -- overridable (e.g. to deliberately shrink
	// them for a saturation test) without touching these defaults.
	poolSize := getenvInt("POOL_SIZE", 5)
	acquireTimeout := time.Duration(getenvInt("ACQUIRE_TIMEOUT_MS", 2000)) * time.Millisecond

	// Create a pool of reusable connections to Redis.
	p, err := pool.New(getenv("REDIS_ADDR", "localhost:6379"), poolSize)
	if err != nil {
		fmt.Println("could not build connection pool:", err)
		return
	}
	fmt.Println("connection pool ready:", p.Available(), "connections to Redis")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("could not accept connection:", err)
			continue
		}
		fmt.Println("simulator connected")
		go handle(conn, p, acquireTimeout)
	}
}

func handle(conn net.Conn, p *pool.Pool, acquireTimeout time.Duration) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	count := 0
	for {
		// Read one event (up to the newline boundary) — same trick as the echo server.
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("simulator disconnected after %d events\n", count)
			return
		}

		// Turn the JSON text into a filled-in TelemetryEvent.
		var event TelemetryEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			fmt.Println("could not parse event:", err)
			continue // skip a bad line, keep going
		}
		count++

		// Forward the event downstream through the pool.
		if err := forward(p, line, acquireTimeout); err != nil {
			fmt.Println("could not forward event:", err)
			continue
		}
	}
}

// forward sends one event to Redis (XADD), borrowing a connection from the pool.
func forward(p *pool.Pool, line string, acquireTimeout time.Duration) error {
	conn, err := p.Acquire(acquireTimeout) // borrow a connection from the pool
	if err != nil {
		return err // pool was saturated and we timed out
	}

	// Send "XADD events * data <json>" encoded in Redis's RESP wire format.
	payload := strings.TrimRight(line, "\r\n") // drop the newline delimiter
	if _, err := conn.Write([]byte(buildXADD("events", payload))); err != nil {
		p.Discard(conn) // the connection broke — throw it away, dial a fresh one
		return err
	}

	// Read Redis's reply (the new entry ID) so the connection stays reusable.
	if _, err := readReply(bufio.NewReader(conn)); err != nil {
		p.Discard(conn)
		return err
	}

	p.Release(conn) // done — put the connection back in the pool
	return nil
}
