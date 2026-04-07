# Client-Jetson Network Stack

## What this is

This is a lightweight client that runs on the **Jetson Nano** (or any Linux SBC
onboard the rover). It sends periodic status/heartbeat packets to the Server-Pi
over TCP so the server knows the Jetson is alive and connected.

## Wire format

Same protocol as the PC client:

```
[4-byte big-endian length] [JSON payload] [4-byte CRC32]
```

- **Length** = size of (payload + CRC), not including the length prefix itself.
- **CRC32** = IEEE CRC-32 computed over the JSON payload only.

The JSON payload is a `StatusPacket`:

```json
{"type":"status","source":"jetson","message":"connected","ts":1711612800000}
```

## How to build and run

```bash
cd Client-Jetson/Network-Stack
go build -o jetson-client .
./jetson-client -server <server-ip>:8080
```

## Flags

| Flag        | Default          | Description                        |
|-------------|------------------|------------------------------------|
| `-server`   | `localhost:8080` | Server address (host:port)         |
| `-source`   | `jetson`         | Source label in packets            |
| `-message`  | `connected`      | Status message to send             |
| `-hz`       | `1`              | Send rate in Hz                    |

## Code structure

`client.go` is organized in this order:

1. **Constants** -- port, default send rate
2. **Types** -- `StatusPacket`
3. **CRC** -- `ComputeCRC`, `AppendCRC`
4. **TCP helpers** -- `writeAll`
5. **Core logic** -- `sendPacket`
6. **main**
