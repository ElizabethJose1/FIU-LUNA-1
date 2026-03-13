package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

const (
	defaultPort = 8080
	defaultHz   = 1
)

type StatusPacket struct {
	Type      string `json:"type"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	Timestamp int64  `json:"ts"`
}

func (s *StatusPacket) String() string {
	return fmt.Sprintf("Source:%s Message:%s", s.Source, s.Message)
}

func writeAll(conn net.Conn, buf []byte) error {
	written := 0
	for written < len(buf) {
		n, err := conn.Write(buf[written:])
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("tcp write returned 0 bytes")
		}
		written += n
	}
	return nil
}

func sendPacket(conn net.Conn, pkt *StatusPacket) error {
	b, err := json.Marshal(pkt)
	if err != nil {
		return fmt.Errorf("marshal packet: %w", err)
	}
	if len(b) > MaxPacketSize {
		return fmt.Errorf("packet too large: %d", len(b))
	}

	framed := AppendCRC(b)
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(framed)))

	if err := writeAll(conn, hdr); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if err := writeAll(conn, framed); err != nil {
		return fmt.Errorf("write packet: %w", err)
	}
	return nil
}

func main() {
	serverAddr := flag.String("server", fmt.Sprintf("localhost:%d", defaultPort), "Server address")
	source := flag.String("source", "jetson", "Source label included in packets")
	message := flag.String("message", "connected", "Status message to send")
	hz := flag.Int("hz", defaultHz, "Send rate in Hz")
	flag.Parse()

	if flag.NArg() > 0 {
		*serverAddr = flag.Arg(0)
	}
	if !strings.Contains(*serverAddr, ":") {
		*serverAddr = fmt.Sprintf("%s:%d", *serverAddr, defaultPort)
	}
	if *hz <= 0 {
		*hz = defaultHz
	}

	for {
		conn, err := net.Dial("tcp", *serverAddr)
		if err != nil {
			log.Printf("Connection error: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		log.Printf("Connected to %s", *serverAddr)
		ticker := time.NewTicker(time.Second / time.Duration(*hz))
		for range ticker.C {
			pkt := &StatusPacket{
				Type:      "status",
				Source:    *source,
				Message:   *message,
				Timestamp: time.Now().UnixMilli(),
			}
			if err := sendPacket(conn, pkt); err != nil {
				ticker.Stop()
				log.Printf("Send error: %v", err)
				_ = conn.Close()
				time.Sleep(3 * time.Second)
				break
			}
			fmt.Println(pkt)
		}
	}
}
