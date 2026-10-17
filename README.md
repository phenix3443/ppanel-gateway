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

## Who needs this

Only the all-in-one `ppanel` runtime image, where one container runs the server
and both web apps behind the gateway. **The k3s and Docker Compose deployments
do not run a gateway at all**: each repo ships its own image, and the three
things the gateway does are covered by the platform instead.

| Gateway role | Replaced by |
|---|---|
| Prefix reverse proxy (`core/proxy`) | Ingress rules (k3s) / published ports (compose) |
| Child-process supervisor (`core/process`, `module.Manager`) | Kubernetes Deployments / compose services |
| GitHub-release self-update (`/basic/check/version`, `/basic/update`) | Image tags + Flux image automation |

Neither `ppanel-server` nor `ppanel-frontend` calls `/basic/*` any more (the admin
version card has a test asserting it makes no gateway calls). Finish
`reconstructed/` only if the all-in-one image is ever needed again.

## Quick start

```sh
# Re-run the binary analysis (needs the gateway binary from the ppanel runtime repo):
./analysis/reproduce.sh /abs/path/to/ppanel/bin/gateway-linux-amd64

# Build & test the reconstruction:
cd reconstructed && go build ./... && go test ./...
```
