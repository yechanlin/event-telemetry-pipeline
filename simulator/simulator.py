import json
import socket
import time
from pathlib import Path

# Where the KITTI OXTS reading files live (relative to this script).
DRIVE = "2011_09_26/2011_09_26_drive_0001_sync"
DATA_DIR = Path(__file__).resolve().parents[1] / "data" / "kitti" / DRIVE / "oxts" / "data"

# The ingestion service to send telemetry to.
HOST = "127.0.0.1"   # 127.0.0.1 = "this computer" (same as localhost)
PORT = 9000

# Which position in each line is which field (0-based), from KITTI's dataformat.txt.
LAT, LON, ALT = 0, 1, 2
VF = 8         # forward velocity (m/s)
AF = 14        # forward acceleration (m/s^2)
WU = 22        # yaw rate: angular rate around the up axis (rad/s)
NUMSATS = 26   # number of GPS satellites


def read_event(file_path, frame):
    """Turn one KITTI reading file into a telemetry event (a dict)."""
    n = file_path.read_text().split()   # split the line into its 30 numbers
    return {
        "frame": frame,
        "lat": float(n[LAT]),
        "lon": float(n[LON]),
        "alt": float(n[ALT]),
        "speed_mps": float(n[VF]),
        "accel_mps2": float(n[AF]),
        "yaw_rate_rps": float(n[WU]),
        "num_satellites": int(n[NUMSATS]),
    }


def main():
    files = sorted(DATA_DIR.glob("*.txt"))   # all readings, in time order
    print(f"found {len(files)} readings")

    # Dial the ingestion service. The `with` block auto-closes the socket at the end.
    with socket.create_connection((HOST, PORT)) as sock:
        print(f"connected to ingestion service at {HOST}:{PORT}")
        for frame, file_path in enumerate(files):
            event = read_event(file_path, frame)
            line = json.dumps(event) + "\n"    # one event + newline boundary
            sock.sendall(line.encode())        # send it over the connection as bytes
            print(f"sent frame {frame}")
            time.sleep(0.1)                    # ~10 readings per second (10 Hz)

    print("done, connection closed")


if __name__ == "__main__":
    main()
