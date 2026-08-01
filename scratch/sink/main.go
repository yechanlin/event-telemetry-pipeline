package main

import (
	"bufio"
	"fmt"
	"net"
)

// A tiny stand-in for the downstream queue (later: Redis).
// It accepts connections and prints the events it receives.
func main() {
	ln, err := net.Listen("tcp", ":9001")
	if err != nil {
		fmt.Println("could not listen:", err)
		return
	}
	defer ln.Close()
	fmt.Println("downstream sink listening on port 9001")

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		fmt.Print("downstream received: ", line)
	}
}
