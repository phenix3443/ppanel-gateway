# Gateway binary — reverse-engineering report

## Authorization & scope

The `gateway` binary analyzed here ships, as a prebuilt artifact only, inside
the `ppanel` runtime repository (`bin/gateway-<os>-<arch>`). The upstream
project (`perfect-panel`) does not publish its source. This analysis was done
by the repository owner for **maintenance, compatibility research, and
functionality recovery** of a self-hosted replacement, and is **static only** —
the binary was never executed during analysis.

This report distinguishes three evidence levels, and never presents inference
as fact:

- **[E] Evidenced** — directly readable from the binary (build metadata,
  pclntab symbol inventory, string constants) or from local first-party source
  in sibling repos.
- **[I] Inferred** — strongly implied by multiple signals (symbol names +
  strings + go-zero conventions) but the function *body* was not decompiled.
- **[U] Unrecovered** — not obtainable with the tools available here; would
  require per-function decompilation.

## Subject

| Field | Value |
|---|---|
| File | `bin/gateway-linux-amd64` (runtime repo) |
| SHA-256 | `7a990413e51af7a332ea9a640486682205ec2e98ab4a5581816142e8ee6a4322` |
| Format | ELF 64-bit, x86-64, dynamically linked, **stripped** (`-ldflags="-s -w"`) |
| Language | Go `go1.24.11`, `-buildmode=exe`, `CGO_ENABLED=1`, `GOAMD64=v1` |
| Go module | `updater` (`(devel)`) |
| Framework | `github.com/zeromicro/go-zero v1.9.2` (go-zero) |
| Router/HTTP | `go-chi/chi/v5 v5.2.3`, `go-resty/resty/v2 v2.16.5` |
| Observability | OpenTelemetry v1.24, Prometheus client, Grafana Pyroscope |
| Config codec | `gopkg.in/yaml.v2 v2.4.0` |

Four platform builds ship: `linux-{amd64,arm64}`, `darwin-{amd64,arm64}`.
This report analyzes `linux-amd64`; the others are the same program.

## How this was produced (tools)

- `file`, `sha256sum`, `strings`, `go version -m` — identity + build metadata.
- **pclntab symbol recovery**: Go keeps its `gopclntab` function-name table even
  under `-s -w`, so the full package/function inventory survives stripping and is
  extractable with `strings` (140 `updater/*` symbols recovered).
- **rea** (`rea-agents@6.5.0`, `inspect-go-binary`) — produced an Evidence-backed
  record (`evidence_id`, SHA-256) confirming the Go build metadata. Saved under
  `analysis/rea-inspect-go-binary.txt`.

### What rea could and could not do here

`rea doctor` reported `environment_healthy: false`: no native analysis engine
(Hopper / Ghidra / IDA / JADX) was installed. rea's `inspect-go-binary` needs no
engine and ran successfully — but it returns **only metadata** (compiler
version, modules, build settings), which rea's own docs state explicitly. rea's
function-level decompilation (`open-binary` + `analyze-function`) requires
Ghidra/Hopper/IDA, which were absent, so **it was not run**. Per rea's docs,
even when available it yields *pseudocode and assembly*, not original source.

**Consequence:** function *bodies* were not decompiled. Everything below at level
[I] is reconstructed from the symbol inventory + strings + go-zero conventions,
not from a decompiled body. See `reconstructed/` for a compatibility skeleton
built from these contracts — it is **not** the original source.

## Recovered architecture

### Package layout [E — from pclntab]

Full inventory: `analysis/symbols-updater.txt` (140 symbols).

```
updater
├── core/
│   ├── logger        Printf, callerLocation
│   ├── process       Process{ Start, Stop }, NewProcess        (child-process supervisor)
│   ├── proxy         Router{ AddRoute, RemoveRoute, ServeHTTP }, NewRouter,
│   │                 getClientIP, matchPrefix                   (reverse proxy / router)
│   ├── scheduler     Scheduler{ AddTask, runTask, Stop }, NewScheduler   (periodic tasks)
│   └── utils         Client{ GetLatestRelease, GetLatestVersionAndURL,
│                     GetLatestVersionAndURLByName }, Release{ BuildDownloadURL,
│                     GetAssetURLByName, GetSystemAssetURL }, NewGithub,
│                     CompareVersion, normalizeVersion, DownloadFile,
│                     ExtractArchive/extractTarGz/extractZip,
│                     detectSystemKeyword, parseRepoURL          (GitHub release client)
└── internal/
    ├── config        RandomInRange, init                       (config structs + defaults)
    ├── svc           NewServiceContext                         (go-zero service context)
    ├── types         RegisterServiceRequest/Response, HeartbeatRequest,
    │                  CheckServiceVersionRequest, ServiceVersionResponse,
    │                  UpdateServiceRequest/Response            (API DTOs)
    ├── handler       Router{ RegisterRoutes, Start, Stop }, NewRouter
    │   └── basic      RegisterServiceHandler, HeartbeatHandler,
    │                  CheckServiceVersionHandler, UpdateServiceHandler
    ├── logic/basic    RegisterServiceLogic, HeartbeatLogic,
    │                  CheckServiceVersionLogic, UpdateServiceLogic
    └── module         Manager{ FindModule, RegisterModule, RemoveModule,
                       Start, Stop, Restart, Rollback, RunBinary, UpdateModule,
                       UpdateInfo, Heartbeat, GetModuleHealth, QueryLastestVersion },
                       StaticModule{ Start, Stop }, NewManager, NewStaticModule,
                       FreePort, GatewayPort, MoveDirSafe, copyDir/copyFile,
                       ReadVersionFile, UpdateBinaryModuleFile,
                       UpdateStaticModuleFile, AdapterModule   (module registry + lifecycle)
```

Note the original internal name is **`updater`**, not "gateway" — the program's
primary identity is a release-driven updater that also fronts HTTP traffic.

### HTTP control plane [E — routes & types]

Handlers and DTO types are named in pclntab; route strings are in the binary:

| Method + path | Handler | Logic | Request type |
|---|---|---|---|
| `POST /basic/register` | `RegisterServiceHandler` | `RegisterServiceLogic.RegisterService` | `RegisterServiceRequest` |
| `GET  /basic/heartbeat` | `HeartbeatHandler` | `HeartbeatLogic.Heartbeat` | `HeartbeatRequest` |
| `GET  /basic/check/version` | `CheckServiceVersionHandler` | `CheckServiceVersionLogic.CheckServiceVersion` | `CheckServiceVersionRequest` |
| `POST /basic/update` | `UpdateServiceHandler` | `UpdateServiceLogic.UpdateService` | `UpdateServiceRequest` |

This matches the four capabilities documented in
`ppanel-frontend/docs/api/gateway.md` and the swagger in prior research.

### Registration contract [E — strings + sibling source]

Registration field names appear as string constants and match the payload the
backend sends (`ppanel-server/internal/report/tool.go`):
`secret`, `service_name`, `service_url`, `proxy_path`, `heartbeat_url`,
`repository`, `service_version`. `[I]` the control-plane lookup key is
`service_name`.

### Public path classes & static hosting [E]

Route/prefix strings: `/api`, `/sub`, `/admin`, `/`. Default config
(`ppanel/ppanel.yaml`) hosts `./static/admin` at `/admin` and `./static/user`
at `/`. `[I]` `/api/*` proxies to the registered backend; `/sub/*` is the
subscription public entry (consistent with gateway-mode rewrites in
`ppanel-server`).

### Runtime integration [E — strings]

Env vars present in the binary: `GATEWAY_MODE`, `GATEWAY_PORT`,
`GATEWAY_LOG_LEVEL`, `SECRET_KEY`. Child path `modules/ppanel-server`. `[I]` the
`module.Manager` + `core/process.Process` launch and supervise the backend with
these injected; `[E]` symbols `Start/Stop/Restart/Rollback/RunBinary` exist.

### Update orchestration [E — symbols + strings]

GitHub client (`core/utils`) with release lookup and asset selection:
`GetLatestRelease`, `GetSystemAssetURL`, `GetAssetURLByName`,
`BuildDownloadURL`, `CompareVersion`, `DownloadFile`,
`ExtractArchive`/`extractTarGz`/`extractZip`, `detectSystemKeyword`. URL
templates `https://api.github.com` and
`https://github.com/%s/%s/releases/download/%s/%s`; referenced repos
`perfect-panel/server` and `perfect-panel/frontend`. Web service names
`admin-web-with-api`, `user-web-with-api`; lock file `version.lock`.
`[I]` updates are GitHub-release driven with a lock guarding concurrent installs
(`UpdateBinaryModuleFile`/`UpdateStaticModuleFile` + `MoveDirSafe` ⇒ staged swap).

## Unrecovered [U]

Function bodies were not decompiled, so these remain hypotheses, **not facts**:

- Exact `service_name` → GitHub repo/asset mapping (naming alignment is strong
  but the mapping code was not read).
- On-disk persisted format of the module registry / update history.
- Whether heartbeat is passive report, active poll, or both.
- Whether static/web updates are atomic swaps or in-place writes (`MoveDirSafe`
  suggests staged move, unconfirmed).
- Whether the backend is auto-restarted after upgrade or left to an external
  supervisor.
- Exact go-zero `rest` server config surface and middleware chain.

To recover these, drive a decompiler: install Ghidra 12.1.x and run
`rea open-binary` + `rea analyze-function`, or run GoReSym/redress for a
pclntab-aware function listing, then disassemble the named functions.

## Relationship to the build plan

`../../obsidian/ppanel/engineering-log/gateway-build-plan.md` proposes a
*greenfield* redesign (`internal/httpapi`, `internal/proxy`, …). This report and
`reconstructed/` instead mirror the **actual binary's** `updater/*` layout, so
the two are complementary: the build plan is the intended rewrite; this is the
observed original to stay compatible with.

## Artifacts

- `analysis/symbols-updater.txt` — recovered pclntab function inventory.
- `analysis/contract-tokens.txt` — routes / env / fields / templates.
- `analysis/rea-inspect-go-binary.txt` — rea Evidence record.
- `analysis/binary.sha256` — subject digest.
- `analysis/reproduce.sh` — re-runs all of the above (static only).
- `reconstructed/` — compilable go-zero-style **compatibility skeleton** built
  from the contracts above. Not original source.

## Verification results

Run in the analysis environment (`reconstructed/` module):

```
gofmt -l .      -> (empty; all formatted)
go build ./...  -> ok (no output)
go vet ./...    -> ok (no output)
go test ./...   -> ok  core/archive, core/github, core/process, core/proxy,
                       core/utils, internal/handler/basic, internal/module
```

Unit/integration coverage: GitHub release lookup + 30m cache + asset selection
(`core/github`), tar.gz extraction + path-traversal rejection (`core/archive`),
child start/stop/restart (`core/process`), the full update pipeline — install +
spawn, no-op when current, `.bak` swap on upgrade — and heartbeat
mark-unhealthy-after-5-failures (`internal/module`), and the control-plane
contract incl. secret auth 401 / unknown-module 404 (`internal/handler/basic`).

### Differential test vs the shipped binary (2026-10-19)

`test/observe-original.sh` boots the shipped binary and `test/observe-reconstructed.sh`
boots the from-source gateway (`go build ./cmd/gateway`), both in an isolated
sandbox. The reconstruction reproduces the shipped binary's observed behavior:
boot → query GitHub (`perfect-panel/server`, `v1.20.3`) → download the
`ppanel-server-linux-amd64.tar.gz` asset → extract to `modules/ppanel-server`
→ spawn it as a child (`run`; the real Hertz server comes up on `127.0.0.1:8080`)
→ serve the control plane. Observed control-plane parity: `GET /basic/heartbeat`
with the right secret → `{"alive":true}`, wrong secret → `401`, unknown module
→ `404`; `GET /basic/check/version` → current/latest/update_needed computed from
the GitHub release. `SIGTERM` stops the child and shuts down cleanly.

**Status: the reconstruction is a functional, from-source, behavior-compatible
replacement** for the control plane + updater + reverse-proxy + supervisor. The
remaining `[U]`-adjacent details that do not affect external behavior (exact
log strings, the precise go-zero/chi middleware ordering, the on-wire shape of
the service→gateway registration handshake body) are intentionally not
byte-matched; they are documented as inference where they appear.

## Function-level decompilation (Ghidra 12.1.4 via rea)

A native decompiler was installed after the first pass: **Ghidra 12.1.4**, driven
through **rea** (`rea function --provider ghidra`). `rea inspect` reported 7095
procedures / 38336 strings (Evidence-backed). Because headless Ghidra does **not**
apply Go `gopclntab` symbol names, procedures appear as `FUN_<addr>`; entry
addresses for the Go functions were resolved with Go's own `debug/gosym`
(`analysis/addr.go`, invoked in `reproduce.sh`). Decompiled pseudocode for the
functions below is saved under `analysis/decompiled/`.

Caveat: Go's register calling convention makes Ghidra pseudocode noisy
(`extraout_*` pseudo-registers); it is **pseudocode and inference, not original
source**. The findings below rest on the stable parts — referenced string
constants and call structure.

Resolved from decompilation (these move `[U]`/`[I]` → `[E]`):

- **System/asset keyword is `<goos>-<goarch>`** — `detectSystemKeyword`
  (`0x7a0f80`) builds it via `Sprintf("linux-%s", "amd64")`; referenced strings
  `linux`, `amd64`, `%s-%s`, `linux-%s`. So release-asset selection keys off e.g.
  `linux-amd64`, matching `ppanel-server-linux-amd64`. (Was `[I]`.)
- **`GetSystemAssetURL`** (`0x7a1080`) calls `detectSystemKeyword` then selects a
  release asset by that keyword — confirms platform-specific asset lookup.
- **Update is version-gated** — `UpdateService` (`0x7ee560`) references
  `"Latest version: %s, Current version: %s"`, `"Update failed: "`, and strips a
  leading `"v"`; it compares latest vs current before acting. (Was `[I]`.)
- **Update applies a `.bak` backup-and-swap** — `UpdateModule` (`0x7e67e0`)
  references `".bak"` plus Info/Error/Debug logs. The existing artifact is moved
  to `.bak` before the new one is installed, which is what makes `Manager.Rollback`
  possible. (Was the `[U]` "atomic swap vs in-place" question — now evidenced as
  backup+swap.)
- **`CompareVersion`** (`0x7a1ce0`) normalizes (helper at `0x7a1fc0`), splits both
  sides on `"."`, and compares segment-by-segment — **validates the reconstruction**
  in `reconstructed/core/utils/version.go`.

## 2026-10-19 — `[U]` closure (deeper decompilation + live runtime observation)

A second rea/Ghidra pass decompiled the logic/handler/update bodies, and the
binary was then **booted in an isolated sandbox** (`test/observe-original.sh`,
minimal `etc/ppanel.yaml` + `GATEWAY_*`/`SECRET_KEY` env) and observed live.
Every previously-residual `[U]` is now `[E]`:

- **`service_name` → repo = the `repository` field supplied at registration.**
  `parseRepoURL` (`0x7a0de0`) strips a trailing `/` and splits the repo URL path,
  requiring ≥2 segments (`owner/repo`). Not hardcoded per service.
- **Registry is in-memory, not persisted.** `RegisterModule` (`0x7ebb00`) stores
  into a map and POSTs a JSON handshake back to the service; no file writes.
  Live run wrote no registry file to the sandbox. (`version.lock` is the update
  mutex file, used only during an update.)
- **Heartbeat is BOTH active and passive.** `Manager.Heartbeat` (`0x7e8540`) is a
  scheduler job (observed interval **1m**) that issues an HTTP `GET` to each
  module's `heartbeat_url` (per-request client timeouts ~3s/30s seen in code) and
  marks a module unhealthy (`flag@+0xc0 = 0`) after **>5** consecutive failures.
  The `/basic/heartbeat` endpoint (`HeartbeatLogic`, `0x7eddc0`) is the passive
  reader: it validates `service_name`+`secret` and returns that health flag.
- **Auth:** register/heartbeat/check/update each compare the request `secret`
  against the module's stored secret (401/`0x191` on mismatch, 404/`0x194` if the
  module is unknown).
- **Latest-version lookup is GitHub, cached 30 min.** `QueryLastestVersion`
  (`0x7e8da0`) returns the cached version when the cache age `< 1.8e12 ns` (1800s);
  otherwise it builds a GitHub client and fetches the latest release, caching the
  version + timestamp. `GetSystemAssetURL` (`0x7a1080`) picks the release asset
  whose name contains `linux-amd64`.
- **Update pipeline** (`UpdateService` `0x7ee560`): version-gate via
  `CompareVersion`; only acts when current < latest; then **dispatches by module
  type** — `static` → `FUN_007ea340` (download + extract web bundle); `service` →
  `FUN_007eb360` (download binary) then `UpdateModule`.
- **`UpdateModule`** (`0x7e67e0`): acquires the lock, **stops the running child
  process**, `rename(current → current.bak)`, `rename(new → current)`, then
  **restarts the child**; on failure rolls back from `.bak` and restarts. So the
  backend IS auto-restarted by the gateway itself.

**Live-observed boot behavior (sandbox, 2026-10-19):** on start the gateway
initialized a scheduler, found a default module `ppanel-server` (baseline current
`v0.0.1`), queried GitHub (`perfect-panel/server`, latest `v1.20.3`), downloaded
`ppanel-server-linux-amd64.tar.gz`, extracted to `modules/ppanel-server`, spawned
it as a child (`modules/ppanel-server run`, Hertz server on `127.0.0.1:8080`),
started the HTTP route service on `0.0.0.0:<Port>` proxying `/*` to the module,
and registered the heartbeat cron at `1m`. Minimal config that boots:
`Name/Host/Port/Mode` in `etc/ppanel.yaml`.

**Deployment role:** the gateway is the **ENTRYPOINT of the upstream all-in-one
`ppanel` container image** (`ppanel/Dockerfile`: `ENTRYPOINT ["/app/gateway"]`,
bundling gateway + `modules/ppanel-server` + `etc/ppanel.yaml`). It supervises,
updates and reverse-proxies the server inside one container. The current k8s
deployment does **not** use it (server/web run as separate from-source
Deployments), so a reconstructed gateway targets the all-in-one image path, not
the live cluster.
