package main

import (
	"bufio"
	"encoding/json" // Go's JSON reader/writer
	"fmt"
	"net"
)

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
	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		fmt.Println("could not start listening:", err)
		return
	}
	defer listener.Close()
	fmt.Println("ingestion service listening on port 9000")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("could not accept connection:", err)
			continue
		}
		fmt.Println("simulator connected")
		go handle(conn)
	}
}

func handle(conn net.Conn) {
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
		fmt.Printf("event #%d: frame=%d speed=%.2f m/s sats=%d\n",
			count, event.Frame, event.SpeedMPS, event.NumSatellites)
	}
}
