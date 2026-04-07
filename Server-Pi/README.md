# Server-Pi Notes

## Method 2 Serial Gate

The Raspberry Pi server now uses the "Method 2" safety check before sending
controller bytes to the Arduino.

- The serial port can stay open.
- Every controller packet is still received and parsed by `Network-Stack/server.go`.
- Before each serial write, the server reads `/tmp/rover_state`.
- The server can also publish controller-driven state change requests to
  `/tmp/rover_state_request`.
- Serial writes are only allowed when the rover state is `TELEOP`.
- `IDLE`, `AUTO`, missing state data, malformed state data, and stale state data
  all block the serial write.

The rover state file is published by `Rover/main.c` in this format:

```text
STATE,unix_timestamp_ms
```

Example:

```text
TELEOP,1775358012435
```

## Manual Test Controls

`Rover/main.c` includes simple stdin controls for testing state transitions:

- `i` + `Enter` -> switch to `IDLE`
- `t` + `Enter` -> switch to `TELEOP`
- `a` + `Enter` -> switch to `AUTO`

Each transition updates `/tmp/rover_state`, which is what the Go server uses
for Method 2 gating.

## Controller State Requests

The network stack can now request rover state changes from controller packets.
The intended mapping is:

- hold `SELECT` (`SCB`) for at least 0.5s
- while still holding it:
  - `Y / N` -> `TELEOP`
  - `B / E` -> `AUTO`
  - `X / W` -> `IDLE`

The Go server writes one-shot requests to `/tmp/rover_state_request`, and
`Rover/main.c` consumes that file in its main loop before updating
`/tmp/rover_state`.

## Quick Test Flow

1. Start the rover state machine from `Server-Pi/Rover`.
2. Start the Go server from `Server-Pi/Network-Stack`.
3. Switch the rover to `IDLE` and confirm controller packets do not get sent to
   the Arduino.
4. Switch the rover to `TELEOP` and confirm serial writes resume.
5. Switch back to `IDLE` or `AUTO` and confirm writes stop again.

## Safety Note

The server also checks that the rover state timestamp is fresh. If the
`/tmp/rover_state` file stops updating for too long, the server treats that as
unsafe and blocks serial writes.
