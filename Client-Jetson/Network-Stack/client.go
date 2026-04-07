// Client-Jetson Network Stack
// Sends periodic status packets to the server over TCP using a
// length-prefixed JSON+CRC32 protocol. Used for heartbeat / telemetry
// from the Jetson to the Pi.
//
// Wire format: [4-byte big-endian length][JSON payload][4-byte CRC32]

package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"log"
	"net"
	"strings"
	"time"
)

// ============================================================================
// Constants
// ============================================================================

const (
	DefaultPort = 8080
	DefaultHz   = 1
)

// MaxPacketSize is the upper bound for a single JSON payload in bytes.
var MaxPacketSize = 8192

// ============================================================================
// Types
// ============================================================================

// StatusPacket is a lightweight message sent from the Jetson to the server.
type StatusPacket struct {
	Type      string `json:"type"`
	Source    string `json:"source"`
	Message   string `json:"message"`
	Timestamp int64  `json:"ts"`
}

func (s *StatusPacket) String() string {
	return fmt.Sprintf("Source:%s Message:%s", s.Source, s.Message)
}

// ============================================================================
// CRC (IEEE CRC-32)
// ============================================================================

func ComputeCRC(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

// AppendCRC appends a 4-byte big-endian CRC32 to the payload.
func AppendCRC(data []byte) []byte {
	c := ComputeCRC(data)
	out := make([]byte, len(data)+4)
	copy(out, data)
	binary.BigEndian.PutUint32(out[len(data):], c)
	return out
}

// ============================================================================
// TCP helpers
// ============================================================================

// writeAll writes the entire buffer to conn, handling partial writes.
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

// ============================================================================
// Core logic
// ============================================================================

// sendPacket marshals, frames, and sends a single status packet.
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

// ============================================================================
// main
// ============================================================================

func main() {
	serverAddr := flag.String("server", fmt.Sprintf("localhost:%d", DefaultPort), "Server address")
	source := flag.String("source", "jetson", "Source label included in packets")
	message := flag.String("message", "connected", "Status message to send")
	hz := flag.Int("hz", DefaultHz, "Send rate in Hz")
	flag.Parse()

	if flag.NArg() > 0 {
		*serverAddr = flag.Arg(0)
	}
	if !strings.Contains(*serverAddr, ":") {
		*serverAddr = fmt.Sprintf("%s:%d", *serverAddr, DefaultPort)
	}
	if *hz <= 0 {
		*hz = DefaultHz
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
