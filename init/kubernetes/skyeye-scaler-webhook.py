"""Webhook for skyeye-scaler: scales the SkyEye Deployment between 0 and 1.

- "run" with players > 0: scale to 1.
- "run" with 0 players (the scaler's stop-delay grace period, also right after a scaler restart): no change.
- "stop": scale to 0.
- Watchdog: no request for WATCHDOG_SECONDS (the scaler exits while SRS is unreachable): scale to 0.

Runs as a sidecar of skyeye-scaler and listens on localhost only. Uses the pod's
ServiceAccount, which may only get/patch deployments/scale of the target Deployment.
"""

import json
import os
import signal
import ssl
import sys
import threading
import time
import urllib.request
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

SA_DIR = "/var/run/secrets/kubernetes.io/serviceaccount"
NAMESPACE = os.environ.get("TARGET_NAMESPACE") or open(f"{SA_DIR}/namespace").read().strip()
DEPLOYMENT = os.environ.get("TARGET_DEPLOYMENT", "skyeye")
LISTEN = os.environ.get("LISTEN_ADDRESS", "127.0.0.1")
PORT = int(os.environ.get("LISTEN_PORT", "8080"))
WATCHDOG_SECONDS = int(os.environ.get("WATCHDOG_SECONDS", "300"))
API = "https://{}:{}".format(
    os.environ["KUBERNETES_SERVICE_HOST"], os.environ.get("KUBERNETES_SERVICE_PORT", "443")
)
SCALE_URL = f"{API}/apis/apps/v1/namespaces/{NAMESPACE}/deployments/{DEPLOYMENT}/scale"
SSL_CTX = ssl.create_default_context(cafile=f"{SA_DIR}/ca.crt")

_lock = threading.Lock()
_last_request = time.monotonic()


def log(**fields):
    fields = {"time": datetime.now(timezone.utc).isoformat(), **fields}
    print(json.dumps(fields), flush=True)


def kube(method, body=None, content_type=None):
    # The projected token is rotated by the kubelet, so read it on every call.
    token = open(f"{SA_DIR}/token").read().strip()
    req = urllib.request.Request(SCALE_URL, method=method, data=body)
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Accept", "application/json")
    if content_type:
        req.add_header("Content-Type", content_type)
    with urllib.request.urlopen(req, context=SSL_CTX, timeout=10) as resp:
        return json.load(resp)


def scale_to(desired, **why):
    with _lock:
        current = kube("GET").get("spec", {}).get("replicas", 0)
        if current != desired:
            body = json.dumps({"spec": {"replicas": desired}}).encode()
            kube("PATCH", body, "application/merge-patch+json")
            log(msg="scaled", deployment=DEPLOYMENT, replicas_from=current, replicas_to=desired, **why)
        return current


def watchdog():
    while True:
        time.sleep(30)
        idle = time.monotonic() - _last_request
        if idle > WATCHDOG_SECONDS:
            try:
                scale_to(0, reason="watchdog", idle_seconds=int(idle))
            except Exception as exc:
                log(msg="error", where="watchdog", error=repr(exc))


class Handler(BaseHTTPRequestHandler):
    def _reply(self, code, payload):
        data = json.dumps(payload).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == "/healthz":
            self._reply(200, {"ok": True})
        else:
            self._reply(404, {"error": "not found"})

    def do_POST(self):
        global _last_request
        try:
            length = int(self.headers.get("Content-Length", "0"))
            payload = json.loads(self.rfile.read(length) or b"{}")
            action = payload.get("action")
            players = int(payload.get("players") or 0)
            if action not in ("run", "stop"):
                self._reply(400, {"error": f"unknown action {action!r}"})
                return
            _last_request = time.monotonic()
            if action == "stop":
                desired = 0
            elif players > 0:
                desired = 1
            else:
                self._reply(200, {"changed": False, "reason": "run without players: grace period"})
                return
            current = scale_to(desired, action=action, players=players)
            self._reply(200, {"replicas": desired, "changed": current != desired})
        except Exception as exc:  # the scaler logs non-2xx and retries at the next interval
            log(msg="error", where="request", error=repr(exc))
            self._reply(500, {"error": str(exc)})

    def log_message(self, fmt, *args):
        pass


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))
    log(msg="listening", address=LISTEN, port=PORT, namespace=NAMESPACE, deployment=DEPLOYMENT,
        watchdog_seconds=WATCHDOG_SECONDS)
    threading.Thread(target=watchdog, daemon=True).start()
    try:
        ThreadingHTTPServer((LISTEN, PORT), Handler).serve_forever()
    except KeyboardInterrupt:
        sys.exit(0)
