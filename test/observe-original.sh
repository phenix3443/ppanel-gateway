#!/usr/bin/env bash
# Observe the SHIPPED gateway binary's runtime behavior in an isolated sandbox.
#
# This is controlled-replay evidence for the `[U]` items that static
# decompilation cannot pin cleanly (GitHub release URL construction, on-disk
# persisted format, version.lock flow, heartbeat semantics, update pipeline).
# It runs the binary ONLY inside a throwaway temp dir, under strace, with no
# privileged access, and never points it at the real ppanel runtime tree.
#
# Usage:
#   ./observe-original.sh /abs/path/to/gateway-linux-amd64 [port]
#
# Output: test/observed/original/ — endpoint responses, created files,
# and the filtered strace (connect peers = which GitHub host; renames = .bak
# swap; opened files = registry/version.lock paths).
set -euo pipefail

BIN="${1:?usage: observe-original.sh /abs/path/to/gateway-<os>-<arch> [port]}"
PORT="${2:-18080}"
[ -x "$BIN" ] || { echo "not executable: $BIN" >&2; exit 2; }
command -v strace >/dev/null || { echo "strace required" >&2; exit 2; }

HERE="$(cd "$(dirname "$0")" && pwd)"
OUT="$HERE/observed/original"
rm -rf "$OUT"; mkdir -p "$OUT"
SANDBOX="$(mktemp -d)"
trap 'kill "${GW_PID:-0}" 2>/dev/null || true; rm -rf "$SANDBOX"' EXIT

echo "== sandbox: $SANDBOX  port: $PORT =="
cp "$BIN" "$SANDBOX/gateway"

# go-zero config at etc/ppanel.yaml [E contract token]. Keys are the config
# struct field names recovered from strings (PascalCase, go-zero default).
# Start minimal; the boot log names any missing required field so we iterate.
mkdir -p "$SANDBOX/etc"
cat > "$SANDBOX/etc/ppanel.yaml" <<YAML
Name: gateway
Host: 0.0.0.0
Port: $PORT
Mode: dev
YAML

# Controlled env [E env tokens]: GATEWAY_MODE/GATEWAY_PORT/GATEWAY_LOG_LEVEL/SECRET_KEY.
( cd "$SANDBOX" && strace -f -o "$OUT/strace.log" \
    -e trace=network,openat,open,rename,unlink,execve \
    env GATEWAY_MODE=dev GATEWAY_PORT="$PORT" GATEWAY_LOG_LEVEL=debug SECRET_KEY=testsecret \
        ./gateway ) >"$OUT/stdout.log" 2>&1 &
GW_PID=$!
cp "$SANDBOX/etc/ppanel.yaml" "$OUT/used-config.yaml" 2>/dev/null || true

# Wait for the control-plane port (max 10s).
for _ in $(seq 1 50); do
  if curl -fsS -o /dev/null "http://127.0.0.1:$PORT/" 2>/dev/null \
     || curl -fsS -o /dev/null "http://127.0.0.1:$PORT/basic/heartbeat" 2>/dev/null; then
    break
  fi
  kill -0 "$GW_PID" 2>/dev/null || { echo "gateway exited early — see stdout.log"; break; }
  sleep 0.2
done

probe() { # method path [data]
  local m="$1" p="$2" d="${3:-}" f
  f="$OUT/resp$(echo "$p" | tr '/?=&' '____').txt"
  { echo "# $m $p";
    if [ "$m" = POST ]; then
      curl -sS -m 5 -X POST -H 'Content-Type: application/json' -d "$d" -D - "http://127.0.0.1:$PORT$p"
    else
      curl -sS -m 5 -D - "http://127.0.0.1:$PORT$p"
    fi; } >"$f" 2>&1 || true
  echo "  probed $m $p -> ${f##*/}"
}

SAMPLE='{"service_name":"ppanel-server","service_url":"http://127.0.0.1:9000","proxy_path":"/api","heartbeat_url":"http://127.0.0.1:9000/health","repository":"https://github.com/perfect-panel/server","service_version":"1.0.0","secret":"testsecret"}'

probe POST "/basic/register" "$SAMPLE"
probe GET  "/basic/heartbeat?service_name=ppanel-server"
probe GET  "/basic/check/version?service_name=ppanel-server"
probe POST "/basic/update" '{"service_name":"ppanel-server"}'

sleep 1
# Snapshot what the binary wrote in its working dir.
( cd "$SANDBOX" && find . -type f ! -name gateway -printf '%p\t%s bytes\n' ) >"$OUT/sandbox-files.txt" 2>&1 || true
# Capture any registry / lock files verbatim (small text only).
( cd "$SANDBOX" && find . -type f ! -name gateway -size -64k \
    \( -name '*.json' -o -name '*.lock' -o -name '*.yaml' -o -name '*.db' -o -name '*registry*' -o -name '*module*' \) \
    -exec sh -c 'echo "=== $1 ==="; cat "$1"' _ {} \; ) >"$OUT/sandbox-state.txt" 2>&1 || true

kill "$GW_PID" 2>/dev/null || true; wait "$GW_PID" 2>/dev/null || true

echo "== extracting evidence from strace =="
{
  echo "### outbound connect() peers (which host it dials, e.g. GitHub):"
  grep -aoE 'connect\([0-9]+, \{sa_family=AF_INET[6]?[^}]*\}' "$OUT/strace.log" 2>/dev/null | sort -u | head -40
  echo; echo "### getaddrinfo / DNS names:"
  grep -aoE '"[a-z0-9.-]+\.(com|org|net|io)"' "$OUT/strace.log" 2>/dev/null | sort -u | head
  echo; echo "### rename() calls (reveals .bak backup-and-swap):"
  grep -aE 'rename\(' "$OUT/strace.log" 2>/dev/null | sed -E 's/^[0-9]+ +//' | sort -u | head -40
  echo; echo "### opened files outside the binary (registry / version.lock / config):"
  grep -aE 'openat?\(' "$OUT/strace.log" 2>/dev/null | grep -aE '\.(json|lock|yaml|yml|db)"' | sed -E 's/^[0-9]+ +//' | sort -u | head -60
} >"$OUT/evidence.txt" 2>&1

echo "== done. evidence in $OUT =="
ls -la "$OUT"
