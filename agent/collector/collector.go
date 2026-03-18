package collector

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
)

// MetricType corresponds to the enum in C++
type MetricType int32

const (
	NCCL_ALL_REDUCE MetricType = iota
	CUDA_MALLOC
	CUDA_MEMCPY
)

// MetricEvent corresponds to the struct in C++
// We must be careful with padding. C++ struct:
// struct MetricEvent { Type type; long long duration_us; size_t count; };
type MetricEvent struct {
	Type       MetricType
	_          [4]byte // Padding to align int64
	DurationUs int64
	Count      uint64
}

type Collector struct {
	socketPath string
	MetricsChan chan MetricEvent
}

func NewCollector(path string) *Collector {
	return &Collector{
		socketPath:  path,
		MetricsChan: make(chan MetricEvent, 1000),
	}
}

func (c *Collector) Start() error {
	if _, err := os.Stat(c.socketPath); err == nil {
		_ = os.Remove(c.socketPath)
	}

	addr, err := net.ResolveUnixAddr("unixgram", c.socketPath)
	if err != nil {
		return err
	}

	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		return err
	}

	if err := os.Chmod(c.socketPath, 0777); err != nil {
		return err
	}

	go c.listen(conn)
	return nil
}

func (c *Collector) listen(conn *net.UnixConn) {
	defer conn.Close()
	buf := make([]byte, 1024)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Printf("Error reading from UDS: %v\n", err)
			continue
		}

		if n < 24 { // Expecting at least 24 bytes based on struct alignment
			continue
		}

		var event MetricEvent
		err = binary.Read(bytes.NewReader(buf[:n]), binary.LittleEndian, &event)
		if err != nil {
			fmt.Printf("Error decoding event: %v\n", err)
			continue
		}

		c.MetricsChan <- event
	}
}
