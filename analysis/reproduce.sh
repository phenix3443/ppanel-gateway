#!/usr/bin/env bash
# Reproduce the gateway binary analysis.
#
# This re-runs every extraction step behind REVERSE-ENGINEERING.md. It performs
# STATIC inspection only: it never executes the gateway binary.
#
# Prerequisites:
#   - Go toolchain (for `go version -m`)
#   - binutils (`strings`, `sha256sum`), `file`
#   - Node.js >= 22.19 + npx (for the optional rea step)
#
# Usage:
#   ./reproduce.sh /absolute/path/to/gateway-linux-amd64
set -euo pipefail

GW="${1:-}"
if [[ -z "$GW" || ! -f "$GW" ]]; then
  echo "usage: $0 /absolute/path/to/gateway-<os>-<arch>" >&2
  echo "the gateway binaries ship in the ppanel runtime repo under bin/" >&2
  exit 2
fi

HERE="$(cd "$(dirname "$0")" && pwd)"

echo "== 1. identity =="
file "$GW"
sha256sum "$GW" | tee "$HERE/binary.sha256"

echo "== 2. Go build metadata =="
go version -m "$GW" | head -80

echo "== 3. recovered package/function inventory (from pclntab, survives -s -w) =="
strings -n 6 "$GW" | grep -E '^updater/' | sort -u | tee "$HERE/symbols-updater.txt" | wc -l

echo "== 4. discrete contract tokens =="
strings -n 5 "$GW" | grep -aoE \
  '/(basic|api|sub|admin)(/[a-z/]+)?|GATEWAY_[A-Z_]+|SECRET_KEY|service_name|proxy_path|service_url|heartbeat_url|service_version|repository|version\.lock|admin-web-with-api|user-web-with-api|modules/ppanel-server|static/(admin|user)|etc/ppanel\.yaml' \
  | sort | uniq -c | sort -rn

echo "== 5. GitHub URL templates =="
strings -n 5 "$GW" | grep -aoE 'https://(api\.)?github\.com[^ "]*' | sort -u

echo "== 6. (optional) rea Evidence-backed Go metadata =="
if command -v npx >/dev/null 2>&1; then
  npx -y rea-agents@latest inspect-go-binary "$GW" 2>/dev/null \
    | tee "$HERE/rea-inspect-go-binary.txt" | head -20
  echo "(full rea Evidence record saved to rea-inspect-go-binary.txt)"
else
  echo "npx not found; skipping rea step" >&2
fi

echo
echo "NOTE: steps 1-6 above are static metadata/string/symbol extraction."
echo "Per-function decompilation is a separate step that needs a native"
echo "decompiler (Ghidra) driven via rea; it was performed for key functions"
echo "and is documented below plus in REVERSE-ENGINEERING.md. Decompiled"
echo "pseudocode is saved under analysis/decompiled/."

# ----------------------------------------------------------------------------
# Optional: function-level decompilation (requires Ghidra + rea)
#
#   export GHIDRA_INSTALL_DIR=/path/to/ghidra_12.1.x_PUBLIC
#   export JAVA_HOME=/path/to/jdk-21        # Ghidra needs JDK 21+
#
# Resolve Go function entry addresses (headless Ghidra does NOT apply pclntab
# names, so target functions by address):
#   go run "$HERE/addr.go" "$GW" 'core/utils.detectSystemKeyword' \
#       'core/utils.(*Release).GetSystemAssetURL' 'logic/basic.(*UpdateServiceLogic).UpdateService'
#
# Decompile one by its entry address:
#   rea function "$GW" 0x7a0f80 --provider ghidra --format json
# ----------------------------------------------------------------------------
