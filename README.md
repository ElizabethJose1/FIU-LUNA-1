# FIU-Luna1

Distributed rover control system for the NASA Lunabotics competition. Three networked nodes work together: a **PC client** captures gamepad input, a **Jetson client** sends status heartbeats, and a **Raspberry Pi server** receives everything, formats it into byte arrays, and forwards commands to an Arduino over serial.

## Project Overview

```
Client-PC/           Operator laptop (Linux) — reads gamepad, streams to server
Client-Jetson/       Jetson Nano onboard rover — sends status heartbeats
Server-Pi/           Raspberry Pi onboard rover — central TCP server + Arduino serial
Embedded-Processor/  Arduino firmware (WIP)
```

### What's Implemented

**Network Protocol** — All nodes share a length-prefixed JSON + CRC32 wire protocol:
```
[4-byte big-endian length][JSON payload][4-byte big-endian CRC32]
```
Max packet size is 8192 bytes. CRC32 (IEEE) is computed over the JSON payload only.

**Client-PC (controller input)**
- Reads gamepad via Linux evdev (`/dev/input/event*`)
- Polls at ~33 Hz, normalizes analog axes to 0–255
- Streams `ControllerState` JSON (buttons, sticks, triggers, d-pad, sequence number) over TCP
- Logs every sent packet to `sent_packets.jsonl` for correlation with server errors
- Auto-reconnects on disconnect (3s retry)

**Client-Jetson (status heartbeat)**
- Sends a `StatusPacket` (type, source, message, timestamp) at 1 Hz
- Pure stdlib Go, no external dependencies
- Same CRC32 wire protocol as Client-PC

**Server-Pi (central hub)**
- TCP server on port 8080, accepts both PC and Jetson clients
- CRC32 verification on every incoming packet
- Batch packet logging — groups of 10, only writes batches that contain errors or sequence gaps
- Configurable byte formatter converts `ControllerState` → Arduino byte array via JSON config
- Serial output to Arduino at 9600 baud (`/dev/ttyACM0`), with optional CRC and ACK support

**Byte Formatting** — Two config templates:
- 6-byte (default): start marker + analog fields + button bits + end marker
- 8-byte extended: full button bitfield + all analog axes + frame markers (0xFF)

---

## Setting Up Client-PC on Linux

### Prerequisites

- **Go 1.21+** installed ([go.dev/dl](https://go.dev/dl/))
- A USB gamepad (Xbox, DualShock, etc.) connected to the machine
- The Server-Pi must be running and reachable on the network

### 1. Clone the repository

```bash
git clone https://github.com/<your-org>/LUNABOTICS.git
cd LUNABOTICS/Client-PC/Network-Stack
```

### 2. Install Go dependencies

```bash
go mod tidy
```

### 3. Find your gamepad device

Plug in your controller and identify its event device:

```bash
cat /proc/bus/input/devices
```

Look for your gamepad name and note the `event` number (e.g., `event4`). Verify it works:

```bash
cat /dev/input/event4 | xxd | head
```

You should see binary data scrolling when you press buttons. If you get a permission error:

```bash
sudo chmod a+r /dev/input/event*
```

Or add your user to the `input` group for a persistent fix:

```bash
sudo usermod -aG input $USER
# Log out and back in for this to take effect
```

### 4. Build the client

```bash
go build -o client client.go
```

### 5. Run

```bash
./client -server <PI_IP>:8080
```

Replace `<PI_IP>` with the Raspberry Pi's IP address on your network.

**Available flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | `localhost:8080` | Server address (ip:port) |
| `-y-north` | `true` | Swap X/Y axis mapping so Y is North |
| `-debug-events` | `false` | Log raw evdev events to console |

### Example

```bash
# Connect to the Pi at 192.168.1.50, with debug output
./client -server 192.168.1.50:8080 -debug-events
```

The client will scan for a gamepad on startup, connect to the server, and begin streaming controller state at ~33 Hz. If the connection drops, it retries every 3 seconds automatically.

---

## Setting Up Server-Pi

```bash
cd Server-Pi/Network-Stack
go mod tidy
go build -o server server.go
./server -public -serial-device /dev/ttyACM0
```

| Flag | Default | Description |
|------|---------|-------------|
| `-port` | `8080` | TCP listen port |
| `-public` | `false` | Bind to 0.0.0.0 (required for remote clients) |
| `-config` | *(built-in 6-byte)* | Path to byte-mapping JSON config |
| `-serial-device` | `/dev/ttyACM0` | Arduino serial port |
| `-serial-crc` | `false` | Append CRC32 to serial data |
| `-serial-ack` | `false` | Expect ACK (0x06) from Arduino |
| `-packet-log` | `packet_errors.jsonl` | Error log file path |

## Setting Up Client-Jetson

```bash
cd Client-Jetson/Network-Stack
go build -o jetsonclient client.go
./jetsonclient -server <PI_IP>:8080
```

| Flag | Default | Description |
|------|---------|-------------|
| `-server` | `localhost:8080` | Server address |
| `-source` | `jetson` | Source label in packets |
| `-message` | `connected` | Status message |
| `-hz` | `1` | Send rate in Hz |

