# Client-PC Network Stack

## What this is

This is the operator-side client that runs on a **Linux laptop**. It reads a
gamepad (Xbox/DualShock) via Linux evdev, packages the controller state as JSON,
and streams it to the Server-Pi over TCP at ~33 Hz.

## Wire format

Every packet on the wire looks like this:

```
[4-byte big-endian length] [JSON payload] [4-byte CRC32]
```

- **Length** = size of (payload + CRC), not including the length prefix itself.
- **CRC32** = IEEE CRC-32 computed over the JSON payload only.

## How to build and run

```bash
cd Client-PC/Network-Stack
go build -o client .
./client -server <server-ip>:8080
```

## Flags

| Flag             | Default          | Description                             |
|------------------|------------------|-----------------------------------------|
| `-server`        | `localhost:8080` | Server address (host:port)              |
| `-y-north`       | `true`           | Swap X/Y so Y acts as North             |
| `-debug-events`  | `false`          | Log raw evdev events for debugging      |

## Packet logging

Each session writes `sent_packets.jsonl` in the working directory. Every line
is one sent packet:

```json
{"seq":1,"crc32":2891736145,"sent_at":1711612800000}
```

This log can be correlated with the server's `packet_errors.jsonl` using the
`seq` and `crc32` fields.

## Code structure

`client.go` is organized in this order:

1. **Constants** -- port, send rate, evdev codes
2. **Types** -- `ControllerState`, evdev structs
3. **CRC** -- `ComputeCRC`, `AppendCRC`
4. **TCP helpers** -- `writeAll`
5. **evdev helpers** -- device open, read, normalize
6. **Core logic** -- `readController`, `applyEvent`, `runClient`
7. **main**
