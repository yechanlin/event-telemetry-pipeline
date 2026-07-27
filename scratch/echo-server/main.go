package main

import (
	"bufio"   // helps us read text one line at a time
	"fmt"     // helps us print messages to the screen
	"net"     // gives us networking: connections, listening, etc.
	"syscall" // lets us peek at the raw OS-level file descriptor
)

func main() {
	// Open the "front desk" on port 9000 and put up the "open" sign.
	// "tcp" means a reliable phone-line-style connection.
	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		fmt.Println("could not start listening:", err)
		return
	}
	defer listener.Close() // close the front desk when the program ends
	fmt.Println("echo server is listening on port 9000")

	// Keep greeting people forever.
	for {
		// Wait for the next person to connect.
		// This line pauses here until someone arrives.
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("could not accept connection:", err)
			continue
		}
		fmt.Println("a client connected")

		// Hand this client to its own goroutine (a lightweight worker) so the
		// loop is immediately free to accept more clients. This is what lets the
		// server serve many clients at once instead of one at a time.
		go handle(conn)
	}
}

// handle reads lines from one connection and echoes them back.
func handle(conn net.Conn) {
	defer conn.Close() // hang up this phone line when we're done

	// Peek at the file descriptor number the OS gave this connection.
	// Go normally hides this number inside conn — we reach under the hood to see it.
	if raw, err := conn.(syscall.Conn).SyscallConn(); err == nil {
		raw.Control(func(fd uintptr) {
			fmt.Printf("handling connection on fd %d\n", fd)
		})
	}

	reader := bufio.NewReader(conn)

	for {
		// Read what the client typed, up to the end of a line.
		line, err := reader.ReadString('\n')
		if err != nil {
			// An error here usually means the client hung up.
			fmt.Println("client disconnected")
			return
		}
		// Send the exact same text back to them.
		conn.Write([]byte(line))
	}
}
