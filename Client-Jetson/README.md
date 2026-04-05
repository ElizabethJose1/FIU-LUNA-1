# Client-Jetson

Jetson-side components for rover status reporting and camera bring-up.

## Camera Bring-Up

Use these commands on the Jetson to verify that an IMX219-based camera is
connected correctly and can be opened locally before attempting dashboard or
network integration.

### 1. Start with NVIDIA's camera preview tool

```bash
DISPLAY=:0.0 nvgstcapture-1.0
```

If needed, explicitly try the first sensor:

```bash
DISPLAY=:0.0 nvgstcapture-1.0 --sensor-id=0
```

### 2. Check whether a V4L2 device appears

```bash
ls /dev/video*
```

If `v4l2-ctl` is installed:

```bash
v4l2-ctl --list-devices
```

### 3. Test the local camera script

After implementing
`Neural-Network/Gazebo/controllers/sensors/camera_stream.py`, run:

```bash
python3 Neural-Network/Gazebo/controllers/sensors/camera_stream.py --source auto
```

If the Jetson CSI path does not open, try the V4L2 fallback:

```bash
python3 Neural-Network/Gazebo/controllers/sensors/camera_stream.py --source v4l2 --v4l2-device /dev/video0
```

If you only want to validate frame capture without opening a preview window:

```bash
python3 Neural-Network/Gazebo/controllers/sensors/camera_stream.py --source auto --headless
```

## What Success Looks Like

- `nvgstcapture-1.0` opens a live preview on the Jetson display.
- The camera script opens the device, receives a valid first frame, and stays
  stable during preview.
- If preview fails, check ribbon seating, Jetson camera support, and whether
  the correct CSI or V4L2 path is available.
