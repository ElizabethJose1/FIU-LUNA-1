"""
Local Jetson camera bring-up script for issue #12.

Goal:
- Open a connected camera on the Jetson.
- Validate that frames are actually arriving.
- Show a local live preview for quick hardware testing.

Hardware target:
- This script is intended for Jetson developer boards in general, including
  Jetson Nano, Jetson Nano 2GB, and Jetson Orin Nano systems.
- IMX219 camera modules are a good match for this path, but board connector
  style, ribbon adapters, and Jetson image support can vary by model.
- The final proof is still whether the Jetson can open the camera and receive
  frames reliably.

Out of scope:
- Dashboard integration.
- Streaming video through the Go server or Raspberry Pi.
"""

import argparse
import os
import sys
import time

try:
    import cv2
except ImportError:
    print("OpenCV is required for camera preview. Install it on the Jetson, then retry.")
    raise SystemExit(1)


DEFAULT_WIDTH = 1280
DEFAULT_HEIGHT = 720
DEFAULT_FPS = 30
DEFAULT_SENSOR_ID = 0
DEFAULT_FLIP_METHOD = 0
DEFAULT_V4L2_DEVICE = "/dev/video0"
DEFAULT_STATUS_INTERVAL = 5.0
DEFAULT_MAX_FAILURES = 5
WINDOW_TITLE = "Jetson Camera Preview"


def parse_args():
    parser = argparse.ArgumentParser(
        description="Bring up a Jetson camera feed and preview it locally."
    )
    parser.add_argument(
        "--source",
        choices=("auto", "jetson", "v4l2"),
        default="auto",
        help="Camera source to try first. 'auto' prefers Jetson CSI and falls back to V4L2.",
    )
    parser.add_argument(
        "--sensor-id",
        type=int,
        default=DEFAULT_SENSOR_ID,
        help="CSI sensor ID for Jetson cameras.",
    )
    parser.add_argument(
        "--v4l2-device",
        default=DEFAULT_V4L2_DEVICE,
        help="Fallback V4L2 device path, such as /dev/video0.",
    )
    parser.add_argument(
        "--width",
        type=int,
        default=DEFAULT_WIDTH,
        help="Requested frame width.",
    )
    parser.add_argument(
        "--height",
        type=int,
        default=DEFAULT_HEIGHT,
        help="Requested frame height.",
    )
    parser.add_argument(
        "--fps",
        type=int,
        default=DEFAULT_FPS,
        help="Requested frames per second.",
    )
    parser.add_argument(
        "--flip-method",
        type=int,
        default=DEFAULT_FLIP_METHOD,
        help="Jetson GStreamer flip method.",
    )
    parser.add_argument(
        "--headless",
        action="store_true",
        help="Skip the preview window and only validate frame capture.",
    )
    parser.add_argument(
        "--status-interval",
        type=float,
        default=DEFAULT_STATUS_INTERVAL,
        help="Seconds between preview status messages.",
    )
    parser.add_argument(
        "--max-failures",
        type=int,
        default=DEFAULT_MAX_FAILURES,
        help="Consecutive frame failures allowed before exiting.",
    )
    return parser.parse_args()


def build_jetson_pipeline(sensor_id, width, height, fps, flip_method):
    return (
        "nvarguscamerasrc sensor-id={sensor_id} ! "
        "video/x-raw(memory:NVMM), width=(int){width}, height=(int){height}, "
        "format=(string)NV12, framerate=(fraction){fps}/1 ! "
        "nvvidconv flip-method={flip_method} ! "
        "video/x-raw, width=(int){width}, height=(int){height}, format=(string)BGRx ! "
        "videoconvert ! "
        "video/x-raw, format=(string)BGR ! "
        "appsink drop=true sync=false"
    ).format(
        sensor_id=sensor_id,
        width=width,
        height=height,
        fps=fps,
        flip_method=flip_method,
    )


def open_jetson_camera(args):
    pipeline = build_jetson_pipeline(
        args.sensor_id,
        args.width,
        args.height,
        args.fps,
        args.flip_method,
    )
    print("Trying Jetson CSI camera path via GStreamer.")
    capture = cv2.VideoCapture(pipeline, cv2.CAP_GSTREAMER)
    if capture.isOpened():
        return capture, f"Jetson CSI sensor {args.sensor_id}"
    capture.release()
    return None, None


def open_v4l2_camera(args):
    print(f"Trying V4L2 fallback at {args.v4l2_device}.")
    if os.name != "nt" and not os.path.exists(args.v4l2_device):
        print(f"V4L2 device not found: {args.v4l2_device}")
        return None, None

    capture = cv2.VideoCapture(args.v4l2_device, cv2.CAP_V4L2)
    if not capture.isOpened():
        capture.release()
        return None, None

    capture.set(cv2.CAP_PROP_FRAME_WIDTH, args.width)
    capture.set(cv2.CAP_PROP_FRAME_HEIGHT, args.height)
    capture.set(cv2.CAP_PROP_FPS, args.fps)
    return capture, f"V4L2 device {args.v4l2_device}"


def open_camera(args):
    attempts = []
    if args.source == "auto":
        attempts = (open_jetson_camera, open_v4l2_camera)
    elif args.source == "jetson":
        attempts = (open_jetson_camera,)
    else:
        attempts = (open_v4l2_camera,)

    for attempt in attempts:
        capture, label = attempt(args)
        if capture is not None:
            print(f"Camera opened successfully using {label}.")
            return capture, label

    return None, None


def validate_first_frame(capture):
    ok, frame = capture.read()
    if not ok or frame is None or frame.size == 0:
        return False, None
    return True, frame


def preview_loop(capture, first_frame, args, source_label):
    print("First frame received successfully.")
    print("Entering live preview. Press 'q' or Esc to exit.")

    frame_count = 1
    consecutive_failures = 0
    start_time = time.time()
    last_status_time = start_time
    frame = first_frame

    while True:
        if not args.headless:
            cv2.imshow(WINDOW_TITLE, frame)
            key = cv2.waitKey(1) & 0xFF
            if key in (27, ord("q")):
                print("Exit requested by user.")
                break

        ok, next_frame = capture.read()
        if not ok or next_frame is None or next_frame.size == 0:
            consecutive_failures += 1
            print(
                f"Frame read failed from {source_label} "
                f"({consecutive_failures}/{args.max_failures})."
            )
            if consecutive_failures >= args.max_failures:
                print("Camera stream became unstable. Exiting preview.")
                break
            continue

        frame = next_frame
        frame_count += 1
        consecutive_failures = 0

        now = time.time()
        if now - last_status_time >= args.status_interval:
            elapsed = max(now - start_time, 1e-6)
            fps = frame_count / elapsed
            print(
                f"Preview running from {source_label}: "
                f"{frame_count} frames, approx {fps:.1f} FPS."
            )
            last_status_time = now


def cleanup(capture, headless):
    print("Starting camera cleanup.")
    if capture is not None:
        capture.release()
    if not headless:
        cv2.destroyAllWindows()
    print("Cleanup complete.")


def main():
    args = parse_args()

    print("Starting Jetson camera bring-up test.")
    print(
        "Requested settings: "
        f"source={args.source}, resolution={args.width}x{args.height}, fps={args.fps}"
    )

    capture = None
    try:
        capture, source_label = open_camera(args)
        if capture is None:
            print("Camera could not be opened with the selected settings.")
            print(
                "Confirm the camera ribbon and connector match the target "
                "Jetson board, the camera is supported by the installed Jetson "
                "image, and OpenCV has access to the expected CSI or V4L2 path."
            )
            return 1

        ok, first_frame = validate_first_frame(capture)
        if not ok:
            print("Camera opened, but no valid frame was received.")
            return 1

        preview_loop(capture, first_frame, args, source_label)
        return 0
    except KeyboardInterrupt:
        print("Interrupted by user.")
        return 0
    finally:
        cleanup(capture, args.headless)


if __name__ == "__main__":
    sys.exit(main())
