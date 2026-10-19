#!/usr/bin/env bash
# Observe the RECONSTRUCTED gateway's runtime behavior in an isolated sandbox,
# the same way observe-original.sh observes the shipped binary, so the two logs
# can be diffed. Config declares a ppanel-server module (repo perfect-panel/
# server) so the boot path exercises: check version -> download linux-amd64
# asset -> extract -> spawn child -> serve -> heartbeat.
#
# Usage:
#   ./observe-reconstructed.sh /abs/path/to/gateway-recon [port]
set -euo pipefail

BIN="${1:?usage: observe-reconstructed.sh /abs/path/to/gateway-recon [port]}"
PORT="${2:-18081}"
[ -x "$BIN" ] || { echo "not executable: $BIN" >&2; exit 2; }

HERE="$(cd "$(dirname "$0")" && pwd)"
OUT="$HERE/observed/reconstructed"
rm -rf "$OUT"; mkdir -p "$OUT"
SANDBOX="$(mktemp -d)"
trap 'kill "${GW_PID:-0}" 2>/dev/null || true; pkill -P "${GW_PID:-0}" 2>/dev/null || true; rm -rf "$SANDBOX"' EXIT

echo "== sandbox: $SANDBOX  port: $PORT =="
cp "$BIN" "$SANDBOX/gateway"
mkdir -p "$SANDBOX/etc"
cat > "$SANDBOX/etc/ppanel.yaml" <<YAML
Name: gateway
Host: 0.0.0.0
Port: $PORT
Mode: dev
Secret: testsecret
Modules:
  - ServiceName: ppanel-server
    Repository: https://github.com/perfect-panel/server
    ServiceVersion: v0.0.1
    ProxyPath: /
    ServiceURL: http://127.0.0.1:8080
    HeartbeatURL: http://127.0.0.1:8080/
    Kind: service
YAML
cp "$SANDBOX/etc/ppanel.yaml" "$OUT/used-config.yaml"

( cd "$SANDBOX" && strace -f -o "$OUT/strace.log" \
    -e trace=network,openat,open,rename,unlink,execve \
    env GATEWAY_WORKDIR="$SANDBOX" GATEWAY_MODE=dev GATEWAY_PORT="$PORT" SECRET_KEY=testsecret \
        ./gateway -f etc/ppanel.yaml ) >"$OUT/stdout.log" 2>&1 &
GW_PID=$!

for _ in $(seq 1 100); do
  curl -fsS -o /dev/null "http://127.0.0.1:$PORT/basic/heartbeat?service_name=ppanel-server&secret=testsecret" 2>/dev/null && break
  kill -0 "$GW_PID" 2>/dev/null || { echo "gateway exited early — see stdout.log"; break; }
  sleep 0.3
done

probe() { # method path
  local m="$1" p="$2" f
  f="$OUT/resp$(echo "$p" | tr '/?=&' '____').txt"
  curl -sS -m 5 -X "$m" -D - "http://127.0.0.1:$PORT$p" >"$f" 2>&1 || true
  echo "  probed $m $p -> ${f##*/}"
}
probe GET "/basic/heartbeat?service_name=ppanel-server&secret=testsecret"
probe GET "/basic/check/version?service_name=ppanel-server&secret=testsecret"
probe GET "/basic/heartbeat?service_name=ppanel-server&secret=wrong"

sleep 1
( cd "$SANDBOX" && find . -type f ! -name gateway -printf '%p\t%s bytes\n' | sort ) >"$OUT/sandbox-files.txt" 2>&1 || true
{
  echo "### outbound getaddrinfo / connect peers:"
  grep -aoE '"[a-z0-9.-]+\.(com|org|net|io)"' "$OUT/strace.log" 2>/dev/null | sort -u | head
  echo "### rename() (reveals .bak swap):"
  grep -aE 'rename\(' "$OUT/strace.log" 2>/dev/null | sed -E 's/^[0-9]+ +//' | sort -u | head -20
  echo "### execve() of child (module spawn):"
  grep -aE 'execve\(' "$OUT/strace.log" 2>/dev/null | grep -aE 'modules/|ppanel-server' | sed -E 's/^[0-9]+ +//' | sort -u | head
} >"$OUT/evidence.txt" 2>&1

kill "$GW_PID" 2>/dev/null || true; pkill -P "$GW_PID" 2>/dev/null || true; wait "$GW_PID" 2>/dev/null || true
pkill -f "$SANDBOX/modules/ppanel-server" 2>/dev/null || true
echo "== done. evidence in $OUT =="
