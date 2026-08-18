# zfstool

Read-only **ZFS** and host inspection tool: pools, topology, datasets, snapshots, zvols, SMART (via `smartctl`), ARC, and related data. It does **not** create, destroy, or change pools or datasets.

The default command opens a **desktop UI** (native **WebKit** window on Linux when built with CGO). The same interface can run in a **browser**, and a small **HTTP API** is available for scripting or remote use over SSH port forwarding.

**License:** [MIT](LICENSE) · **Module:** `github.com/nixomose/zfstool`

---

## What it does

- **GUI / web UI:** Browse storage pools, drill down into vdevs and disks, view dataset and zvol properties, and open SMART details where `smartctl` is available. Partition maps, mounted volumes (`/`, `/boot`, …), and dataset usage bars are included even when they are not ZFS.
- **API server:** Serves a JSON HTTP API over a **Unix domain socket** (and optionally TCP) for the UI and for tools that speak HTTP.
- **Host view:** OS, kernel, memory, ZFS versions, ARC stats, disks (SSD vs HDD), and mounts.

Data comes from `zfs(8)`, `zpool(8)`, and related read-only sources on the machine where the API server runs.

---

## Requirements

| Component | Purpose |
|-----------|---------|
| **OpenZFS userland** (`zfs`, `zpool`) | Required for meaningful data |
| **Go 1.22+** | Build |
| **GTK 3 + WebKit2GTK** | Native GUI (`make build`); package names vary by distro (e.g. `libgtk-3-dev`, `libwebkit2gtk-4.1-dev` on recent Debian/Ubuntu) |
| **`smartctl`** (package `smartmontools`) | Optional; disk SMART in the UI |
| **`lsblk`** / **`df`** | Optional; partition maps and mounted volumes |

Ubuntu **22.04** ships WebKit **4.0**; **24.04+** typically uses **4.1**. This repo vendors a small `webview` tweak for 4.1; see [`third_party/README.md`](third_party/README.md) if you need **4.0**.

---

## Install (Go)

```bash
go install github.com/nixomose/zfstool/cmd/zfstool@latest
```

Ensure `$(go env GOPATH)/bin` is on your `PATH`. You still need CGO and GTK/WebKit libraries on disk for the **native** window; a pure-Go install without them falls back to building a **browser**-based UI if you use `CGO_ENABLED=0` or the `browser_gui` build tag (see **Build** below).

---

## Usage

### Desktop app (default)

```bash
zfstool
# same as:
zfstool gui
```

With **no** `-agent-socket` / `-agent-url`, the app starts an **embedded API server** on a private socket and tears it down when you exit. **`ZFSTOOL_SOCKET` is not used** for this mode.

Point at an existing API server:

```bash
zfstool -agent-socket /run/zfstool/agent.sock
zfstool -agent-url http://127.0.0.1:8787
```

### API server

Serves the API on a Unix socket (default under `XDG_RUNTIME_DIR` or `/run/zfstool/agent.sock` if set up that way). Optional TCP:

```bash
zfstool server -socket /run/zfstool/agent.sock
zfstool server -socket /run/zfstool/agent.sock -http 127.0.0.1:8787
```

See [`deploy/zfstool-agent.service`](deploy/zfstool-agent.service) and [`deploy/PACKAGING.txt`](deploy/PACKAGING.txt) for **systemd** layout.

### Web front-end + static UI

`zfstool web` does **not** start the API server. Run `zfstool server` first (or use the systemd unit), then start the web front-end. It proxies **`/v1/*`** to the API server and serves the bundled UI on **`/`**:

```bash
# terminal 1
zfstool server -socket /run/zfstool/agent.sock

# terminal 2
zfstool web -listen 127.0.0.1:8787 -agent-socket /run/zfstool/agent.sock
```

Then open http://127.0.0.1:8787/ .

- **Loopback** access is unauthenticated by default.
- For **remote** access, set **`ZFSTOOL_WEB_USER`** and **`ZFSTOOL_WEB_PASSWORD`** (or **`ZFSTOOL_WEB_BCRYPT_HASH`**) as documented in [`internal/web/auth.go`](internal/web/auth.go).

### Other commands

```bash
zfstool version    # or: zfstool -v
zfstool help
```

### UI shortcuts (native / web)

- **F5** — refresh the current view (scroll position preserved where possible).
- **Back** (breadcrumb bar) / **Alt+Left** — return to the previous view.
- **Ctrl+Q** / **Ctrl+W** — close the **native** window only (when the WebView binding is present).
- List filters accept comma-separated terms and `!exclude` (state is kept in the browser).

---

## Build

From a git clone:

```bash
# OS packages for a full native GUI build (see scripts/install-deps.sh)
make deps

# Native WebKit window (recommended on Linux desktop)
make build
# binary: ./bin/zfstool
```

| Target | Description |
|--------|-------------|
| `make build` | **CGO on**, native WebKit window (`GOFLAGS` cleared on the build line). |
| `make build-headless` | **CGO off**; UI opens in the **default browser**. |
| `make build-browser` | **CGO on** but **`-tags browser_gui`**; still uses the browser for UI. |
| `make install` | `make build` then install to `$(PREFIX)/bin` (default `/usr/local/bin`). |
| `make clean` | Remove `bin/`, RPM build tree, stray `./zfstool`. |

Override embedded version string:

```bash
make build VERSION=0.2.0
```

Plain one-liner equivalent to `make build`:

```bash
GOFLAGS= CGO_ENABLED=1 go build -o zfstool ./cmd/zfstool
```

If a **browser tab** opens instead of a window, you likely built with CGO disabled, `browser_gui` tags, or a global **`GOFLAGS`**; use `make build` or the line above.

---

## Packaging

| Target | Notes |
|--------|--------|
| `make deb` | Debian binary package; output in the **parent** directory of the repo. Run `make deb-deps` first on Debian/Ubuntu. |
| `make rpm` | Binary RPM under `build/rpm/RPMS/<arch>/`. Run `make rpm-deps` first. |
| `make srpm` | Source RPM under `build/rpm/SRPMS/`. |

More detail: [`deploy/PACKAGING.txt`](deploy/PACKAGING.txt).

---

## API

The API server exposes **`GET`** (and selected **`POST`**) routes under **`/v1/`**, for example:

- `/v1/host`, `/v1/pools`, `/v1/pools/{pool}/status`, `/v1/datasets`, `/v1/datasets/properties`
- `/v1/disks` (all block devices with partitions, media type, and pool membership), `/v1/disk/{dev}/smart`
- `/v1/mounts` (mounted filesystems including non-ZFS volumes)
- `/v1/pools/{pool}/history`, `/maintenance`, `/properties`, `/devices`
- `/v1/bookmarks`, `/v1/snapshots/holds`, `/v1/iostat`, `/v1/graph`, `/v1/kernel-log`, `/v1/module-params`, `/v1/zfs-allow`
- `GET /v1/browse?dataset=&path=` — list files/dirs under a filesystem or snapshot mount (confined)
- `POST /v1/zfs-diff` — `{ "from", "to" }`

The UI and `zfstool web` proxy this tree to the API server socket or TCP backend.

---

## Remoting

Three steps: **API server** on the ZFS host, **web front-end** on that host, then **connect** from your workstation.

```bash
# 1) on the ZFS host — API server (or: sudo systemctl enable --now zfstool-agent)
zfstool server -socket /run/zfstool/agent.sock

# 2) on the ZFS host — web UI (proxies /v1 to the socket above)
zfstool web -listen 127.0.0.1:8787 -agent-socket /run/zfstool/agent.sock

# 3) on your workstation — SSH forward, then open the UI
ssh -L 8787:127.0.0.1:8787 user@zfs-host
# browser: http://127.0.0.1:8787/
# or desktop GUI:
zfstool gui -agent-url http://127.0.0.1:8787
```

**API-only** (no bundled UI): `zfstool server -socket /run/zfstool/agent.sock -http 127.0.0.1:8787`.

**Packaged API server:** after installing the `.deb` / RPM:

```bash
sudo systemctl enable --now zfstool-agent
# API: unix:/run/zfstool/agent.sock
```

Non-loopback `zfstool web` requires `ZFSTOOL_WEB_USER` + `ZFSTOOL_WEB_PASSWORD` (or bcrypt hash). See the **Remote** page in the UI for copy-paste snippets.

---

## Security notes

- Tooling is intended to be **read-only** with respect to ZFS; still run the API server with appropriate **filesystem and socket permissions**.
- Do not expose **`zfstool web`** to untrusted networks without **HTTP auth** (env-based basic auth as implemented in the code).
- **SSH example:** `ssh -L 8787:127.0.0.1:8787 user@zfs-host` then open the UI against `127.0.0.1:8787`.

---

## Contributing / third party

- **Issues & PRs:** [github.com/nixomose/zfstool](https://github.com/nixomose/zfstool)
- **Vendored WebKit binding:** `third_party/webview_go` (upstream MIT; local `replace` in `go.mod`).
