"""
Throwaway load tool: opens many concurrent connections to the LIVE ingestion
service (port 9000) and blasts events on each, unpaced -- unlike simulator.py,
which is one connection at a slow, paced 10Hz. Only useful for deliberately
saturating a shrunk pool (see POOL_SIZE/ACQUIRE_TIMEOUT_MS in docker-compose.yml)
to prove pool_acquire_timeouts_total and its alert actually fire for real.
"""

import json
import socket
import threading

HOST = "127.0.0.1"
PORT = 9000
CONNECTIONS = 50
EVENTS_PER_CONNECTION = 100

EVENT = json.dumps({
    "frame": 0, "lat": 49.0, "lon": 8.4, "alt": 100.0,
    "speed_mps": 13.17, "accel_mps2": 0.0, "yaw_rate_rps": 0.0,
    "num_satellites": 11,
}) + "\n"


def hammer(conn_id):
    with socket.create_connection((HOST, PORT)) as sock:
        for _ in range(EVENTS_PER_CONNECTION):
            sock.sendall(EVENT.encode())
    print(f"connection {conn_id} done")


def main():
    threads = [threading.Thread(target=hammer, args=(i,)) for i in range(CONNECTIONS)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    print(f"done: {CONNECTIONS} connections x {EVENTS_PER_CONNECTION} events, unpaced")


if __name__ == "__main__":
    main()
