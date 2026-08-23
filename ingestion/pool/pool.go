// Package pool provides a hand-built, bounded pool of reusable TCP connections.
package pool

import (
	"fmt"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// InUse reports how many pooled connections are currently checked out.
// It's a gauge — a number that goes up and down — updated every time a
// connection is borrowed (Acquire) or returned (Release/Discard).
var InUse = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "pool_connections_in_use",
	Help: "Number of pooled Redis connections currently checked out.",
})

// Pool is a bounded set of reusable phone lines (connections) to one downstream address.
type Pool struct {
	conns chan net.Conn // the bowl: open connections waiting to be used
	addr  string        // the server address we dial to make a connection
}

// New makes a pool and fills the bowl with `size` open phone lines.
func New(addr string, size int) (*Pool, error) {
	p := &Pool{
		conns: make(chan net.Conn, size), // a bowl that holds `size` connections
		addr:  addr,
	}

	for i := 0; i < size; i++ {
		conn, err := net.Dial("tcp", addr) // dial one phone line
		if err != nil {
			return nil, err // if dialing fails, stop and report the problem
		}
		p.conns <- conn // drop the open line into the bowl
	}

	return p, nil
}

// Acquire takes one phone line off the rack. It waits if the rack is empty,
// but only up to `timeout` — then it gives up and returns an error.
func (p *Pool) Acquire(timeout time.Duration) (net.Conn, error) {
	select {
	case conn := <-p.conns:
		InUse.Inc() // one more connection just left the rack
		return conn, nil // a phone was available — take it
	case <-time.After(timeout):
		return nil, fmt.Errorf("timed out waiting for a connection")
	}
}

// Release puts a phone line back on the rack for reuse.
func (p *Pool) Release(conn net.Conn) {
	p.conns <- conn // drop it back in the bowl
	InUse.Dec()      // one fewer connection checked out
}

// Discard throws away a broken connection and dials a fresh replacement,
// so the pool stays at full size.
func (p *Pool) Discard(conn net.Conn) error {
	conn.Close() // hang up the dead line
	InUse.Dec()  // it was checked out; it no longer is (broken or not)

	newConn, err := net.Dial("tcp", p.addr) // dial a fresh replacement
	if err != nil {
		return err // couldn't replace it (downstream down?) — pool is one smaller for now
	}
	p.conns <- newConn // put the fresh line on the rack
	return nil
}

// Available reports how many connections are currently free on the rack.
func (p *Pool) Available() int {
	return len(p.conns)
}
