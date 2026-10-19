# ppanel-gateway

A from-source, behavior-compatible implementation of the ppanel all-in-one
container's `gateway` entrypoint: a supervisor that, for each configured module,
checks its latest GitHub release, downloads and extracts the platform asset,
runs the module as a child process, reverse-proxies incoming requests to it, and
polls its heartbeat.

## When the gateway is used

Only the all-in-one `ppanel` runtime image, where one container runs the server
and both web apps behind the gateway. The k3s and Docker Compose deployments do
**not** run a gateway: each component ships its own image, and the three things
the gateway does are covered by the platform instead.

| Gateway role | Replaced by |
|---|---|
| Prefix reverse proxy (`internal/proxy`) | Ingress rules (k3s) / published ports (compose) |
| Child-process supervision (`internal/process`, `internal/module`) | Kubernetes Deployments / compose services |
| GitHub-release self-update (`/basic/check/version`, `/basic/update`) | Image tags + image automation |

In the split deployments nothing calls `/basic/*`; build this only if the
all-in-one image is ever needed again.

## Layout

```
cmd/gateway      main — loads etc/ppanel.yaml, wires everything, serves
internal/
  config         YAML config load + env overrides + module bootstrap
  github         GitHub release lookup, asset selection, download (30m cache)
  archive        tar.gz extraction with path-traversal guards
  process        child-process supervisor (start/stop/restart)
  proxy          longest-prefix reverse-proxy router
  module         service registry + heartbeat poller + update pipeline
  server         control-plane HTTP API (/basic/*) + proxy + static hosting
  version        dotted numeric version comparison
test/            differential-test harnesses vs the original binary
```

## Build & test

```sh
go build ./...
go test ./...

# cross-compile a release binary (CGO disabled, stripped)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" ./cmd/gateway
```

## Control plane

| Method | Path                    | Purpose                                 |
|--------|-------------------------|-----------------------------------------|
| POST   | `/basic/register`       | a module announces itself               |
| GET    | `/basic/heartbeat`      | report a module's current health flag   |
| GET    | `/basic/check/version`  | current vs latest release               |
| POST   | `/basic/update`         | run the update pipeline for a module    |

Each endpoint validates the module's `secret` (401 on mismatch, 404 if unknown).
Public `/api` and `/sub` traffic is reverse-proxied to registered upstreams;
`/admin` and `/` are served from `static/`.

## Configuration

`etc/ppanel.yaml` (`-f` to override). Minimal boot needs `Name`/`Host`/`Port`/
`Mode`; `Modules` declares supervised services. Env overrides: `GATEWAY_PORT`,
`GATEWAY_MODE`, `SECRET_KEY`, `GATEWAY_WORKDIR`.
