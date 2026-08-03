package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// buildXADD encodes the command  XADD <stream> * data <payload>  in Redis's RESP
// wire format: an array of length-prefixed strings.
//
//	*5\r\n              -> 5 arguments follow
//	$4\r\nXADD\r\n      -> a 4-char argument: XADD
//	$<len>\r\n<stream>  -> the stream name
//	$1\r\n*\r\n         -> "*" (let Redis pick the entry ID)
//	$4\r\ndata\r\n      -> the field name: data
//	$<len>\r\n<payload> -> the event JSON
func buildXADD(stream, payload string) string {
	args := []string{"XADD", stream, "*", "data", payload}

	var sb strings.Builder
	fmt.Fprintf(&sb, "*%d\r\n", len(args)) // "*N" = N arguments follow
	for _, a := range args {
		fmt.Fprintf(&sb, "$%d\r\n%s\r\n", len(a), a) // "$len" then the argument
	}
	return sb.String()
}

// readReply reads exactly one Redis reply from the connection. We must read it so
// the connection stays in sync for the next reuse (otherwise the next borrower would
// read this leftover reply by mistake). Returns the reply text, or an error if Redis
// reported one.
func readReply(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "", fmt.Errorf("empty reply from redis")
	}

	switch line[0] {
	case '+', ':': // simple string or integer
		return line[1:], nil
	case '-': // error reply
		return "", fmt.Errorf("redis error: %s", line[1:])
	case '$': // bulk string (e.g. the new entry's ID)
		n, err := strconv.Atoi(line[1:])
		if err != nil {
			return "", fmt.Errorf("bad bulk length %q: %w", line, err)
		}
		if n < 0 {
			return "", nil // nil reply
		}
		data, err := reader.ReadString('\n') // the value line + \r\n
		if err != nil {
			return "", err
		}
		return strings.TrimRight(data, "\r\n"), nil
	}
	return "", fmt.Errorf("unexpected reply: %q", line)
}
