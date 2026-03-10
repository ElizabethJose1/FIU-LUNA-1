// client.go (Linux evdev version)
// Reads /dev/input/event* and maps to ControllerState, then sends JSON+CRC with 4-byte length prefix.
//
// Requires: go get golang.org/x/sys/unix
// Run on Jetson/RPi/Linux (not Windows).

package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	DEFAULT_PORT = 8080
	SEND_RATE_HZ = 33 // ~30ms between sends

	// evdev event types
	EV_KEY = 0x01
	EV_ABS = 0x03

	// ABS axes (from linux input-event-codes.h, matches your Python mapping) :contentReference[oaicite:3]{index=3}
	ABS_X     = 0x00
	ABS_Y     = 0x01
	ABS_Z     = 0x02
	ABS_RZ    = 0x05
	ABS_HAT0X = 0x10
	ABS_HAT0Y = 0x11
	ABS_GAS   = 0x09  // right trigger :contentReference[oaicite:4]{index=4}
	ABS_BRAKE = 0x0A  // left trigger :contentReference[oaicite:5]{index=5}

	// Buttons (typical gamepad codes; your Python maps these via ecodes) :contentReference[oaicite:6]{index=6}
	BTN_SOUTH  = 0x130
	BTN_EAST   = 0x131
	BTN_NORTH  = 0x133
	BTN_WEST   = 0x134
	BTN_TL     = 0x136
	BTN_TR     = 0x137
	BTN_TL2    = 0x138
	BTN_TR2    = 0x139
	BTN_SELECT = 0x13a
	BTN_START  = 0x13b
	BTN_THUMBL = 0x13d
	BTN_THUMBR = 0x13e

	// Some devices expose X/Y explicitly; Python swaps BTN_X/BTN_Y when y_north=True :contentReference[oaicite:7]{index=7}
	BTN_X = 0x133 // often same as BTN_NORTH on some mappings; kept for optional swap logic
	BTN_Y = 0x134 // often same as BTN_WEST on some mappings; kept for optional swap logic
)

// ControllerState holds all controller inputs (same as your original)
type ControllerState struct {
	North       uint8 `json:"N"`
	East        uint8 `json:"E"`
	South       uint8 `json:"S"`
	West        uint8 `json:"W"`
	LeftBumper  uint8 `json:"LB"`
	RightBumper uint8 `json:"RB"`
	LeftStick   uint8 `json:"LS"`
	RightStick  uint8 `json:"RS"`
	Select      uint8 `json:"SELECT"`
	Start       uint8 `json:"START"`

	LeftX        uint8 `json:"LjoyX"`
	LeftY        uint8 `json:"LjoyY"`
	RightX       uint8 `json:"RjoyX"`
	RightY       uint8 `json:"RjoyY"`
	LeftTrigger  uint8 `json:"LT"`
	RightTrigger uint8 `json:"RT"`
	DPadX        int8  `json:"dX"`
	DPadY        int8  `json:"dY"`

	Timestamp int64 `json:"ts"`
}

func (c *ControllerState) String() string {
	return fmt.Sprintf("Btns[N:%d E:%d S:%d W:%d] Joy[LX:%d LY:%d RX:%d RY:%d] Trig[L:%d R:%d] DPad[%d,%d]",
		c.North, c.East, c.South, c.West,
		c.LeftX, c.LeftY, c.RightX, c.RightY,
		c.LeftTrigger, c.RightTrigger,
		c.DPadX, c.DPadY)
}

// Linux input_event (matches kernel struct layout)
type inputEvent struct {
	Time  unix.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// ABS info from EVIOCGABS ioctl
type absInfo struct {
	Value      int32
	Min        int32
	Max        int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

// ioctl numbers (golang doesn't ship these for all arches; this one is stable for EVIOCGABS)
func evioCGAbs(axis uint) uintptr {
	// _IOR('E', 0x40 + axis, struct input_absinfo)
	const (
		iocRead  = 2
		iocNrbits  = 8
		iocTypebits = 8
		iocSizebits = 14
		iocDirshift  = iocNrbits + iocTypebits + iocSizebits
		iocTypeshift = iocNrbits
		iocSizeshift = iocNrbits + iocTypebits
	)
	// 'E' = 0x45
	ioc := func(dir, typ, nr, size uintptr) uintptr {
		return (dir << iocDirshift) | (typ << iocTypeshift) | (nr << 0) | (size << iocSizeshift)
	}
	return ioc(iocRead, 0x45, 0x40+axis, unsafe.Sizeof(absInfo{}))
}

type evdevDevice struct {
	fd       int
	path     string
	absCache map[uint16]absInfo
	yNorth   bool
}

// normalize like your Python AxisEvent: ((value-min)/(max-min))*255 :contentReference[oaicite:8]{index=8}
// DPad hats are NOT normalized (kept as -1/0/1) :contentReference[oaicite:9]{index=9}
func (d *evdevDevice) normalizeAbs(code uint16, v int32) (uint8, bool) {
	// DPad hat: pass-through mapping (-1/0/1) handled elsewhere
	if code == ABS_HAT0X || code == ABS_HAT0Y {
		return 0, false
	}

	info, ok := d.absCache[code]
	if !ok {
		// try lazy query
		var ai absInfo
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(d.fd), evioCGAbs(uint(code)), uintptr(unsafe.Pointer(&ai)))
		if errno != 0 {
			return 127, true // fallback center
		}
		d.absCache[code] = ai
		info = ai
	}

	den := float64(info.Max - info.Min)
	if den <= 0 {
		return 127, true
	}
	norm := (float64(v-info.Min) / den) * 255.0
	if norm < 0 {
		norm = 0
	}
	if norm > 255 {
		norm = 255
	}
	return uint8(norm), true
}

func openEvdev(path string, yNorth bool) (*evdevDevice, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	return &evdevDevice{
		fd:       fd,
		path:     path,
		absCache: make(map[uint16]absInfo),
		yNorth:   yNorth,
	}, nil
}

func (d *evdevDevice) close() {
	_ = unix.Close(d.fd)
}

// readEvent reads one inputEvent (non-blocking)
func (d *evdevDevice) readEvent() (inputEvent, bool, error) {
	var ev inputEvent
	buf := (*[unsafe.Sizeof(ev)]byte)(unsafe.Pointer(&ev))[:]
	n, err := unix.Read(d.fd, buf)
	if err != nil {
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			return inputEvent{}, false, nil
		}
		return inputEvent{}, false, err
	}
	if n != len(buf) {
		return inputEvent{}, false, fmt.Errorf("short read: got %d, want %d", n, len(buf))
	}
	return ev, true, nil
}

// findEvdevController picks the first /dev/input/event* that can be opened.
// (Barebones heuristic—good enough for first draft. You can tighten later.)
func findEvdevController(yNorth bool) (*evdevDevice, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil || len(paths) == 0 {
		return nil, fmt.Errorf("no /dev/input/event* devices found")
	}
	for _, p := range paths {
		d, err := openEvdev(p, yNorth)
		if err == nil {
			log.Printf("Using evdev device: %s", p)
			return d, nil
		}
	}
	return nil, fmt.Errorf("could not open any /dev/input/event*")
}

// readController maintains your same “ticker send loop”, but pulls freshest state from evdev
func readController(dev *evdevDevice, conn net.Conn) error {
	ticker := time.NewTicker(time.Second / SEND_RATE_HZ)
	defer ticker.Stop()

	state := &ControllerState{
		LeftX:  127, LeftY: 127,
		RightX: 127, RightY: 127,
	}

	for range ticker.C {
		// Drain all pending events to update current state
		for {
			ev, ok, err := dev.readEvent()
			if err != nil {
				return fmt.Errorf("evdev read: %w", err)
			}
			if !ok {
				break
			}

			switch ev.Type {
			case EV_ABS:
				code := uint16(ev.Code)
				val := ev.Value

				// DPad hats: keep -1/0/1 as in Python (not normalized) :contentReference[oaicite:10]{index=10}
				if code == ABS_HAT0X {
					if val < -1 {
						val = -1
					} else if val > 1 {
						val = 1
					}
					state.DPadX = int8(val)
					continue
				}
				if code == ABS_HAT0Y {
					if val < -1 {
						val = -1
					} else if val > 1 {
						val = 1
					}
					state.DPadY = int8(val)
					continue
				}

				// Normalize analog axes to 0..255 like Python AxisEvent :contentReference[oaicite:11]{index=11}
				norm, isAnalog := dev.normalizeAbs(code, val)
				if !isAnalog {
					continue
				}

				switch code {
				case ABS_X:
					state.LeftX = norm
				case ABS_Y:
					state.LeftY = norm
				case ABS_Z:
					state.RightX = norm
				case ABS_RZ:
					state.RightY = norm
				case ABS_BRAKE:
					state.LeftTrigger = norm
				case ABS_GAS:
					state.RightTrigger = norm
				}

			case EV_KEY:
				code := uint16(ev.Code)
				pressed := uint8(0)
				if ev.Value != 0 {
					pressed = 1
				}

				// Optional X/Y swap behavior like your Python ButtonEvent(y_north=True) :contentReference[oaicite:12]{index=12}
				if dev.yNorth {
					if code == BTN_X {
						code = BTN_Y
					} else if code == BTN_Y {
						code = BTN_X
					}
				}

				switch code {
				case BTN_NORTH:
					state.North = pressed
				case BTN_EAST:
					state.East = pressed
				case BTN_SOUTH:
					state.South = pressed
				case BTN_WEST:
					state.West = pressed
				case BTN_TL:
					state.LeftBumper = pressed
				case BTN_TR:
					state.RightBumper = pressed
				case BTN_SELECT:
					state.Select = pressed
				case BTN_START:
					state.Start = pressed
				case BTN_THUMBL:
					state.LeftStick = pressed
				case BTN_THUMBR:
					state.RightStick = pressed
				}
			}
		}

		state.Timestamp = time.Now().UnixMilli()

		// Marshal + CRC + length-prefix (same as your existing pipeline)
		b, err := json.Marshal(state)
		if err != nil {
			return fmt.Errorf("marshal state: %w", err)
		}
		if len(b) > MaxPacketSize {
			log.Printf("state too large (%d bytes), skipping send", len(b))
			continue
		}

		pkt := AppendCRC(b)
		hdr := make([]byte, 4)
		binary.BigEndian.PutUint32(hdr, uint32(len(pkt)))

		if _, err := conn.Write(hdr); err != nil {
			return fmt.Errorf("write header: %w", err)
		}
		if _, err := conn.Write(pkt); err != nil {
			return fmt.Errorf("write packet: %w", err)
		}

		fmt.Println(state)
	}

	return nil
}

func runClient(serverAddr string, yNorth bool) error {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	log.Println("Connected to server")

	for {
		dev, err := findEvdevController(yNorth)
		if err != nil {
			log.Println("Waiting for controller (evdev)...")
			time.Sleep(2 * time.Second)
			continue
		}

		err = readController(dev, conn)
		dev.close()

		if err != nil {
			if strings.Contains(err.Error(), "broken pipe") {
				return fmt.Errorf("server disconnected")
			}
			log.Printf("Controller error: %v", err)
			time.Sleep(time.Second)
		}
	}
}

func main() {
	serverAddr := flag.String("server", fmt.Sprintf("localhost:%d", DEFAULT_PORT), "Server address")
	yNorth := flag.Bool("y-north", true, "Swap X/Y mapping to make Y act as North (Python-compatible)")
	flag.Parse()

	if flag.NArg() > 0 {
		*serverAddr = flag.Arg(0)
	}
	if !strings.Contains(*serverAddr, ":") {
		*serverAddr = fmt.Sprintf("%s:%d", *serverAddr, DEFAULT_PORT)
	}

	log.Printf("Connecting to %s (Ctrl+C to stop)", *serverAddr)

	for {
		if err := runClient(*serverAddr, *yNorth); err != nil {
			log.Printf("Connection error: %v", err)
		}
		time.Sleep(3 * time.Second)
	}
}