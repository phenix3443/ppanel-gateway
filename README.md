# ppanel-gateway

Self-hosted replacement for the closed-source `gateway` binary that ships
(binary-only) inside the `ppanel` runtime. Upstream `perfect-panel` does not
publish gateway source; this repo is the owner's compatible reimplementation.

## Contents

| Path | What it is |
|---|---|
| `REVERSE-ENGINEERING.md` | Static analysis of the shipped gateway binary, with evidence levels (evidenced / inferred / unrecovered). |
| `analysis/` | Raw analysis artifacts + a re-runnable `reproduce.sh` (static only; never executes the binary). |
| `reconstructed/` | A compilable Go skeleton that mirrors the recovered package layout and implements the **observed** HTTP contract. **Not original source** — a from-the-outside reimplementation. |

## Status

- Structure (packages, functions, types, routes, env, GitHub release flow) is
  recovered and documented.
- `reconstructed/` builds, vets, and tests clean, but is a **contract skeleton**:
  the update pipeline and child-process supervision are stubbed (their function
  bodies were not decompiled — see the `[U]` section of the report).
- The greenfield redesign plan lives separately in
  `obsidian/ppanel/engineering-log/gateway-build-plan.md`.

## Quick start

```sh
# Re-run the binary analysis (needs the gateway binary from the ppanel runtime repo):
./analysis/reproduce.sh /abs/path/to/ppanel/bin/gateway-linux-amd64

# Build & test the reconstruction:
cd reconstructed && go build ./... && go test ./...
```
