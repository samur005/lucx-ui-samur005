# Fork samur005 / lucx-ui-samur005 — install & update

Repo: https://github.com/samur005/lucx-ui-samur005  
Upstream: https://github.com/AlexeyLCP/lucx-ui  
Full guide (Russian): [FORK.md](FORK.md)

## What this fork adds (vs AlexeyLCP/lucx-ui)

1. **Native vk-hash auto-generation for qWDTT** — Tunnels → qWDTT → **«Генератор vk_hash» / VK hash generator**: paste VK cookies → create call → fill `VkHashes`. Fallbacks: `LUCX_VK_HASH`, optional external `LUCX_WDTT_*`. Package `internal/lucx/vkcreator/`, API `/panel/api/tunnel/vk/*`. See [VKHASH.md](VKHASH.md), [CREDITS.md](../CREDITS.md).
2. **Inbound Templates** — **«Шаблоны» / Templates** on inbound create/edit modal with VLESS presets: XHTTP+Reality, gRPC+Reality, gRPC+TLS, WS+TLS, HTTPUpgrade+TLS, KCP.
3. **WB Stream room generator for olcRTC** — Settings → Cores → "Open tunnel configs" → olcRTC → **«Генератор комнат WB Stream» / WB Stream room generator**: save a stream.wb.ru session (`wbx-refresh` cookie or Bearer) → **Create room** → the room ID is written into the chosen olcRTC inbound (`settings.roomId`). Also a **Create room** button in the olcRTC inbound form and auto-create on save. Package `internal/lucx/wbcreator/`, API `/panel/api/tunnel/wb/*`. Room API from kulikov0/whitelist-bypass (MIT). See [WBROOM.md](WBROOM.md), [CREDITS.md](../CREDITS.md).

Stock `install.sh` / panel **Update** / `x-ui update` install the **upstream** binary **without** these features. Build from this repo to get them. Never use those for “updating the fork”.

## Branch

Until [PR #1](https://github.com/samur005/lucx-ui-samur005/pull/1) merges: use `feat/native-vk-hash-generator`. After merge: use `main`.

## Clean VPS install (from the GitHub fork)

### Quick install (one command)

As `root` on Ubuntu/Debian (x86_64 or aarch64):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/install-fork.sh)
```

[`scripts/install-fork.sh`](../scripts/install-fork.sh) automates manual steps 1–6 below:

1. installs packages (`curl git tar ca-certificates build-essential xz-utils iproute2`);
2. clones the fork into `/usr/local/src/lucx-ui-samur005` (if it already exists: stashes local changes, then `git reset --hard origin/<branch>`);
3. installs Go into `/usr/local/go` (skipped if Go ≥ the required version is already there; required = max of `GO_VER` and `go.mod`) and Node.js into `/usr/local/lib/nodejs` (skipped if Node ≥ 22.12 with npm is already available), adds `PATH` entries in `/etc/profile.d/go.sh` / `nodejs.sh` if no existing `/etc/profile.d` script already has them;
4. **builds first** — frontend + Go binary into `/usr/local/src/lucx-ui-samur005/.install-fork/x-ui`; if the build fails it exits non-zero and **touches nothing** (service and current binary stay as they are);
5. if no panel exists yet (`/usr/local/x-ui/x-ui` + `x-ui` unit), runs the stock AlexeyLCP `install.sh` (interactive: port/SSL questions, prints URL, username and password — **save them**); on an existing panel this step is skipped;
6. stops `x-ui`, backs up `/usr/local/x-ui/x-ui.bak-<date>`, installs the fork binary, starts it and checks `systemctl is-active x-ui` (rolls back to the backup automatically if it does not come up);
7. prints the version, the ports `x-ui`/xray listen on and, if `ufw` is active, which public ports are not yet allowed (it opens them only with `LUCX_UFW_OPEN=1`).

**Re-running the same command updates** an existing panel to the latest fork build (stock `install.sh` is not run; DB and ports are kept).

**Reboot.** On a fresh server the stock `install.sh` may install a new kernel for AmneziaWG and reboot after 10 seconds. The fork script intercepts that reboot, installs the fork binary first and asks you to run `reboot` at the end. If the server does reboot mid-install (e.g. with `LUCX_ALLOW_REBOOT=1`), just run the same command again — the stock step is skipped and the fork binary is built and installed.

Optional environment overrides, e.g. `LUCX_BRANCH=main bash <(curl -fsSL …)`:

| Variable | Default | Meaning |
| --- | --- | --- |
| `LUCX_FORK_REPO` | `https://github.com/samur005/lucx-ui-samur005.git` | fork git URL |
| `LUCX_BRANCH` | `feat/native-vk-hash-generator` | branch to build (`main` after PR #1 merges) |
| `LUCX_SRC` | `/usr/local/src/lucx-ui-samur005` | source checkout directory |
| `GO_VER` | `1.27.1` | minimum Go version (raised to `go.mod` if that is higher) |
| `NODE_VER` | `24.20.0` | Node version to install when no suitable Node (≥ 22.12) exists |
| `LUCX_SKIP_SERVICE` | `0` | `1` = build only (no stock install.sh, no systemd, no binary swap, no `/etc/profile.d` writes) |
| `LUCX_UFW_OPEN` | `0` | `1` = if `ufw` is active, allow the public `x-ui`/xray ports |
| `LUCX_ALLOW_REBOOT` | `0` | `1` = do not intercept the stock installer's reboot |

After PR #1 merges into `main`, replace `feat/native-vk-hash-generator` with `main` in the URL and run with `LUCX_BRANCH=main`.

The manual steps below are the fallback.

### Manual install (fallback)

Full **from scratch** flow on Ubuntu/Debian as `root`.
Upstream `install.sh` creates the service, DB and ports; the **fork binary** (vk-hash + Templates) comes only from building [this repo](https://github.com/samur005/lucx-ui-samur005).

Until [PR #1](https://github.com/samur005/lucx-ui-samur005/pull/1) merges, use branch `feat/native-vk-hash-generator`. After merge, use `main` in every command below.

**Build requirements:** Go **1.27.1+** (`go.mod`), Node.js **≥ 24** and npm **≥ 10**.

### Step 1 — packages

```bash
apt update
apt install -y curl git tar ca-certificates build-essential xz-utils
```

### Step 2 — Go 1.27.1+

```bash
GO_VER=1.27.1
curl -fL "https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz" -o /tmp/go.tgz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=/usr/local/go/bin:$PATH' >/etc/profile.d/go.sh
export PATH=/usr/local/go/bin:$PATH
go version
```

### Step 3 — Node.js 24+

```bash
NODE_VER=24.20.0
curl -fL "https://nodejs.org/dist/v${NODE_VER}/node-v${NODE_VER}-linux-x64.tar.xz" -o /tmp/node.tar.xz
mkdir -p /usr/local/lib/nodejs
tar -C /usr/local/lib/nodejs -xJf /tmp/node.tar.xz
export PATH="/usr/local/lib/nodejs/node-v${NODE_VER}-linux-x64/bin:$PATH"
echo "export PATH=/usr/local/lib/nodejs/node-v${NODE_VER}-linux-x64/bin:\$PATH" >/etc/profile.d/nodejs.sh
node -v
npm -v
```

### Step 4 — base panel service (stock AlexeyLCP)

Creates `/usr/local/x-ui/`, the systemd unit, DB under `/etc/x-ui/`, and ports. **This is not the fork yet.**

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Save the panel URL (port + `webBasePath`), username and password from the script output.
Common defaults are around **29830** (panel) and **2096** (subscription) — trust `install.sh` output.

### Step 5 — clone the fork and build the feature binary

```bash
export PATH="/usr/local/go/bin:/usr/local/lib/nodejs/node-v24.20.0-linux-x64/bin:$PATH"

git clone -b feat/native-vk-hash-generator \
  https://github.com/samur005/lucx-ui-samur005.git \
  /usr/local/src/lucx-ui-samur005
cd /usr/local/src/lucx-ui-samur005

test -d internal/lucx/vkcreator
grep -n EnsureVkHashes internal/lucx/tunnel/qwdtt_inbound.go || true

cd frontend && npm ci && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui
cp -a /usr/local/x-ui/x-ui "/usr/local/x-ui/x-ui.bak-$(date +%Y%m%d%H%M)"
mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui
chmod +x /usr/local/x-ui/x-ui
systemctl start x-ui
systemctl status x-ui --no-pager
```

After PR #1 merges to `main`:

```bash
git clone -b main https://github.com/samur005/lucx-ui-samur005.git /usr/local/src/lucx-ui-samur005
# then the same npm ci / go build / binary replace
```

### Step 6 — firewall (if ufw is enabled)

Open the panel and subscription ports from step 4 (replace with yours):

```bash
ufw allow 29830/tcp
ufw allow 2096/tcp
ufw reload
```

### Step 7 — check

1. Open the URL from `install.sh` (or `https://IP:PORT/webBasePath/`).
2. **Tunnels → qWDTT → VK hash generator**.
3. **Inbounds → create/edit → Templates**.
4. **Tunnels → olcRTC → WB Stream room generator**.

---

## Build & replace binary (panel already installed)

Easiest: the [quick install command](#quick-install-one-command) — on an existing panel it skips stock `install.sh`, installs missing Go/Node, builds the fork and swaps the binary (with a backup).

Manually: if `systemctl status x-ui` already works, skip stock `install.sh` and run step 5 only. Install Go/Node (steps 2–3) first if missing.

## When AlexeyLCP releases — update fork, then rebuild VPS

Two-part flow. Run as root under `/usr/local/src/lucx-ui-samur005`. After PR #1 merges, substitute `main` for the feature branch.

### Part 1 — merge upstream, keep fork features

```bash
cd /usr/local/src/lucx-ui-samur005
git remote add upstream https://github.com/AlexeyLCP/lucx-ui.git 2>/dev/null || true
git fetch origin
git fetch upstream --tags
git checkout feat/native-vk-hash-generator
git pull --ff-only origin feat/native-vk-hash-generator
git merge upstream/main
```

On success: `git push origin feat/native-vk-hash-generator`.

On conflicts: keep `EnsureVkHashes`, `internal/lucx/tunnel/vkhash.go`, `internal/lucx/vkcreator/`, inbound Templates files, `internal/lucx/wbcreator/` + `wb_creator.go` + `olcrtc_wbroom.go` (and the `ensureOlcrtcWbRoom` calls in `inbound_lucx.go`), `OlcrtcWbPanel.tsx`, and `docs/FORK.md` — then add / commit / push.

GitHub **Sync fork → Update** is OK; **Discard** is forbidden.

### Part 2 — rebuild panel on VPS (ports unchanged)

**One command:** the quick install command does all of Part 2 — `git fetch` + `reset --hard` to the latest `origin/<branch>`, frontend + Go build, backup and binary swap, `systemctl is-active x-ui` check:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/install-fork.sh)
```

It stashes local changes in `/usr/local/src/lucx-ui-samur005` (restore with `git stash pop`), so push your Part 1 result to the fork first. Manual equivalent:

Adjust Node `PATH` if yours differs.

```bash
export PATH="/usr/local/lib/nodejs/node-v22.20.0-linux-x64/bin:$PATH"
cd /usr/local/src/lucx-ui-samur005
git checkout feat/native-vk-hash-generator
git pull --ff-only origin feat/native-vk-hash-generator
cd frontend && npm install --no-audit --no-fund && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui
cp -a /usr/local/x-ui/x-ui "/usr/local/x-ui/x-ui.bak-$(date +%Y%m%d%H%M)"
mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui
chmod +x /usr/local/x-ui/x-ui
systemctl start x-ui
systemctl is-active x-ui
ss -tlnp | grep -E '29830|2096'
```

**UI check again:** Tunnels → qWDTT → vk hash generator; Inbounds → Templates; Tunnels → olcRTC → WB Stream room generator (session status, Create room).

Hash-hook-only recovery: `scripts/apply-vkhash.sh` (see Russian doc for curl one-liner), then Part 2.

Full tables and notes: [FORK.md](FORK.md).
