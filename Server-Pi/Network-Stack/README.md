# Server-Pi Network Stack

## What this is

This is the server that runs on the **Raspberry Pi** onboard the rover. It
listens for TCP connections from both the PC client (controller input) and the
Jetson client (status heartbeats). Incoming packets are CRC-verified, logged in
batches for debugging, and the controller state is formatted into bytes and
forwarded to an Arduino over serial.

## Wire format

Same protocol as the clients:

```
[4-byte big-endian length] [JSON payload] [4-byte CRC32]
```

- **Length** = size of (payload + CRC), not including the length prefix itself.
- **CRC32** = IEEE CRC-32 computed over the JSON payload only.

## How to build and run

```bash
cd Server-Pi/Network-Stack
go build -o server .
./server -port 8080 -serial-device /dev/ttyACM0
```

## Flags

| Flag              | Default              | Description                                |
|-------------------|----------------------|--------------------------------------------|
| `-port`           | `8080`               | TCP listen port                            |
| `-public`         | `false`              | Bind to 0.0.0.0 instead of localhost       |
| `-config`         | (built-in 6-byte)    | Path to byte-mapping JSON config           |
| `-serial-device`  | `/dev/ttyACM0`       | Serial device path for Arduino             |
| `-serial-crc`     | `false`              | Append CRC32 to serial writes              |
| `-serial-ack`     | `false`              | Expect 0x06 ACK from Arduino after write   |
| `-packet-log`     | `packet_errors.jsonl`| Path to packet error log                   |

## Packet logging

Packets are grouped into batches of 10. If any packet in a batch has an error
(CRC mismatch, JSON parse failure, size violation, or sequence gap), the entire
batch is written to `packet_errors.jsonl`. Clean batches are silently discarded.

Each line in the log is one packet:

```json
{"seq":42,"crc32":3847261509,"received_at":1711612800030,"status":"OK"}
{"seq":43,"crc32":0,"received_at":1711612800060,"status":"CRC_FAIL","raw_payload":"base64..."}
```

Batches are separated by a blank line.

## Byte config templates

The server converts `ControllerState` JSON into a fixed-length byte array for
the Arduino. Two templates are included:

- `byte_config.json` -- 6-byte format (default)
- `byte_config_8byte.json` -- 8-byte extended format

Use `-config <file>` to switch.

## Code structure

`server.go` is organized in this order:

1. **Constants** -- ports, baud rate, batch size
2. **Types -- Protocol** -- `ControllerState`, `StatusPacket`
3. **Types -- Byte Formatting** -- `ByteFormatter`, `ByteConfig`, mappings
4. **Types -- Packet Logging** -- `PacketLog`, `Batch`, `BatchLogger`
5. **Types -- Serial** -- `SerialManager`
6. **CRC** -- `ComputeCRC`, `AppendCRC`, `VerifyPacket`
7. **Byte Formatter** -- `DefaultConfig`, `Format`, `LoadConfig`
8. **Batch Logger** -- `Record`, `flush`, `Close`
9. **Serial Manager** -- `openArduino`, `Write`, reconnect logic
10. **TCP helpers** -- `formatBytes`, `tryParseStatusPacket`
11. **Client handler** -- `handleClient`
12. **main**
