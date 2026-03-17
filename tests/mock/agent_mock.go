package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
)

// MetricEvent must match the C++ struct alignment
type MetricEvent struct {
	Type       int32
	Padding    int32 // Added to match C++ alignment (4 bytes)
	DurationUs int64
	Count      uint64
}

func main() {
	socketPath := "/tmp/spy.sock" // Use /tmp for mock tests
	_ = os.Remove(socketPath)

	addr, err := net.ResolveUnixAddr("unixgram", socketPath)
	if err != nil {
		fmt.Printf("Failed to resolve socket: %v\n", err)
		return
	}

	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		fmt.Printf("Failed to listen on UDS: %v\n", err)
		return
	}
	defer conn.Close()
	defer os.Remove(socketPath)

	// Ensure the socket is accessible
	_ = os.Chmod(socketPath, 0777)

	fmt.Printf("[Mock Agent] Listening on %s...\n", socketPath)

	buf := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFromUnix(buf)
		if err != nil {
			fmt.Printf("Read error: %v\n", err)
			break
		}

		var event MetricEvent
		err = binary.Read(bytes.NewReader(buf[:n]), binary.LittleEndian, &event)
		if err != nil {
			fmt.Printf("Decode error: %v\n", err)
			continue
		}

		fmt.Printf("[Mock Agent] Received Metric: Type=%d, Duration=%d us, Count=%d\n",
			event.Type, event.DurationUs, event.Count)
	}
}
