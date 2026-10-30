# Fork samur005 / lucx-ui-samur005 — install & update

Repo: https://github.com/samur005/lucx-ui-samur005  
Upstream: https://github.com/AlexeyLCP/lucx-ui  
Full guide (Russian): [FORK.md](FORK.md)

## Management menu `scripts/lucx.sh`

One command (as root), interactive Russian menu:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/lucx.sh)
```

Non-interactive: `lucx.sh install|update|uninstall|gh-sync [--no-push] [--no-build]|backup-full|backup-settings`.

1. **Install** — runs `scripts/install-fork.sh`; warns and offers a full backup if x-ui is already installed.
2. **Update** — disk/RAM+swap checks, backs up DB (SQLite `.backup` or `pg_dump` for PostgreSQL), binary and `/etc/x-ui`, runs `install-fork.sh` with `LUCX_ALLOW_REBOOT=0 LUCX_UFW_OPEN=0`, `GOFLAGS=-p=1/-p=2` by RAM, `nice`; then checks `systemctl is-active`, `x-ui -v`, journal for panic/fatal; rolls the binary back on failure.
3. **Uninstall** — type `yes`, optional full backup, stock `x-ui uninstall` (manual removal fallback).
4. **Sync code on GitHub** — merges `AlexeyLCP/lucx-ui` `main` into the fork branch in `/usr/local/src/lucx-fork-sync` with `--no-ff`; lock files take upstream, other conflicts get an interactive menu (markers, `$EDITOR`, keep ours for fork files, ours/theirs, abort). Checks markers, `go build`/`go vet` if Go is present, then a plain `git push` (gh auth or a one-shot token read with `read -s`). Never force-pushes. `--no-push`, `--no-build` flags.
5. **Full backup** — tar.gz in `/root/lucx-backups/<date>/` with DB, `/etc/x-ui`, `/usr/local/x-ui`, sidecar configs, systemd units, referenced certificates; prints size and sha256.
6. **Settings backup** — DB + `/etc/x-ui` + sidecar configs (qWDTT `passwords.json`, …), small archive.

---

## What this fork adds (vs AlexeyLCP/lucx-ui)

1. **Native vk-hash auto-generation for qWDTT** — Tunnels → qWDTT → **«Генератор vk_hash» / VK hash generator**: paste VK cookies → create call → fill `VkHashes`. Fallbacks: `LUCX_VK_HASH`, optional external `LUCX_WDTT_*`. Package `internal/lucx/vkcreator/`, API `/panel/api/tunnel/vk/*`. See [VKHASH.md](VKHASH.md), [CREDITS.md](../CREDITS.md).
2. **Inbound Templates** — **«Шаблоны» / Templates** on inbound create/edit modal with VLESS presets: XHTTP+Reality, gRPC+Reality, gRPC+TLS, WS+TLS, HTTPUpgrade+TLS, KCP.
3. **WB Stream room generator for olcRTC** — Settings → Cores → "Open tunnel configs" → olcRTC → **«Генератор комнат WB Stream» / WB Stream room generator**: save a stream.wb.ru session (`wbx-refresh` cookie or Bearer) → **Create room** → the room ID is written into the chosen olcRTC inbound (`settings.roomId`). Also a **Create room** button in the olcRTC inbound form and auto-create on save. Package `internal/lucx/wbcreator/`, API `/panel/api/tunnel/wb/*`. Room API from kulikov0/whitelist-bypass (MIT). See [WBROOM.md](WBROOM.md), [CREDITS.md](../CREDITS.md).
4. **Multi-profile subscription for the qWDTT Android app** — the app (SpaceNeuroX) parses a subscription body as a **single** `qwdtt://config?…` URI, so with several qWDTT inbounds (master + nodes) only the first profile was imported. Requests whose `User-Agent` starts with `qWDTT-Subscription` (case-insensitive; the app sends `qWDTT-Subscription/1.0`) now get the app's JSON format `{"subscriptionName", "version", "updatedAt", "profiles":[{name, peer, hashes, workers, port, password}]}` — one profile per enabled qWDTT inbound of the subId, same values as the `qwdtt://` links (node inbounds advertise the node). Base64 when `subEncrypt` is on (the app decodes it); same headers. No qWDTT inbounds → regular body; every other client (Happ, v2rayN, …) gets an unchanged subscription. The JSON also carries the client's traffic as `trafficUsedMb` (up + down; always sent, `0` when nothing is used yet) and `trafficLimitMb` (limit, only if set) in MiB — the same numbers as `Subscription-Userinfo`; the app reads traffic **only** from these JSON fields (it ignores headers) and shows "Трафик: X из Y" on the subscription card (it has no expiry display for subscriptions). Built on the master, nodes need no update; refresh (or re-add) the subscription in the app. Code: `internal/sub/qwdtt_app_sub.go`, hook in `internal/sub/controller.go`.
5. **Client delete with qWDTT / CSQTT / olcRTC inbounds** — these shared-password inbounds keep no per-client entries in `settings.clients` (node-adopted ones have no `clients` key at all); membership lives in `client_inbounds`. Upstream delete/detach required a `clients` array and failed with "invalid clients format in inbound settings". The fork treats a missing key as an empty list and removes a client linked to such an inbound via the link (stats, link, node full-delete) without rewriting the settings JSON. Code: `internal/web/service/client_sidecar_settings.go` + hooks in `client_inbound_apply.go`. Update the node with the fork build too so deletes made directly in the node panel work.

Stock `install.sh` / panel **Update** / `x-ui update` install the **upstream** binary **without** these features. Build from this repo to get them. Never use those for “updating the fork”.

## Branch

Until [PR #1](https://github.com/samur005/lucx-ui-samur005/pull/1) merges: use `feat/native-vk-hash-generator`. After merge: use `main`.

## Maintenance menu `lucx.sh`

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/lucx.sh)
```

A plain text menu: numbered items, pick by typing the number and Enter (input from `/dev/tty`, works via `curl | bash`). Submenus are numbered too, `0` = back/exit. Service items: `90` show all items (not only for the detected role), `91` toggle dry-run, `92` language RU/EN. Dangerous actions ask for confirmation and back up first. Non-interactive: `lucx <command> [--yes] [--dry-run] …`, help: `lucx help`. Log: `/var/log/lucx.log`, cloud creds: `/etc/x-ui/lucx-backup.env` (600).

| # | Command | What it does |
|---|---|---|
| 1 | `prepare` | Getting started / server prep — domain→IP, time, ports, RAM, GitHub; ✓/✗ checklist |
| 2 | `role` | Server role — master / node / single (from DB) |
| 3 | `install` | Install — build and install the fork (install-fork.sh) |
| 4 | `update` | Update — DB+binary backup, build, verify, auto-rollback |
| 5 | `uninstall` | Uninstall — stock x-ui uninstall after a full backup |
| 6 | `rollback` | Roll back version — pick x-ui.bak-*, verify, revert on failure |
| 7 | `preflight` | Pre-update check — disk, RAM, Go/Node, GitHub, new versions |
| 8 | `update-all` | Update all servers — from lucx-hosts.conf: nodes, then master |
| 9 | `backup-full` | Full backup — DB, /etc/x-ui, /usr/local/x-ui, sidecars, units, certs |
| 10 | `backup-settings` | Settings backup — DB, /etc/x-ui, passwords.json, sidecar configs |
| 11 | `restore` | Restore backup — pick archive, backup current, restart, verify |
| 12 | `backup-cron` | Scheduled backups — cron, rotation, Telegram upload |
| 13 | `cloud-sync` | Cloud backup sync — Yandex Disk (WebDAV) / S3 (rclone, aws-cli) |
| 14 | `migrate` | Migrate to new server — backup → scp → install → restore |
| 15 | `db-migrate` | SQLite ↔ PostgreSQL — stock x-ui migrate-db (back: from backup) |
| 16 | `status` | Status — version, service, ports, SSL, nodes, sidecars, RAM/disk |
| 17 | `logs` | Logs — x-ui and sidecars, error filter, follow |
| 18 | `restart` | Restart — panel / Xray / sidecars |
| 19 | `reset-access` | Reset access — username, password, path, port, 2FA |
| 20 | `ssl` | SSL — issue (stock x-ui menu/acme.sh) and renew |
| 21 | `geo` | Update geo files — geoip/geosite without rebuilding |
| 22 | `xray-core` | Update Xray core — pick version, via panel API |
| 23 | `cleanup` | Cleanup — old x-ui.bak, backups, Go/npm cache |
| 24 | `monitor` | Telegram monitoring — x-ui and nodes, alert on failure/recovery |
| 25 | `ufw` | Firewall (UFW) — show/open/close panel and inbound ports |
| 26 | `bbr` | BBR & network tuning — BBR, fq, limits, sysctl (revertable) |
| 27 | `swap` | Swap — show, create or grow swapfile |
| 28 | `fail2ban` | Fail2ban — protect SSH and panel login |
| 29 | `ssh-harden` | SSH port & key-only login — with checks, old port kept until confirmed |
| 30 | `latency` | Node latency & ports — ping/tcp to nodes, listening inbounds |
| 31 | `reality-check` | Reality camouflage check — donor reachable, TLS 1.3, h2 |
| 32 | `reality-rotate` | Rotate Reality keys — new keys and shortId (clients must refresh) |
| 33 | `qwdtt-clients` | qWDTT clients — devices list, remove test entries |
| 34 | `sub-check` | Subscription check — by subId/email: nodes, expiry, traffic |
| 35 | `traffic-report` | Traffic report — top clients, expiring soon, ≥90% quota |
| 36 | `clients-bulk` | Bulk client actions — extend, reset traffic, enable/disable |
| 37 | `hwid-reset` | Reset client HWID — remove bound devices (client_hwids) |
| 38 | `inbounds-export` | Export inbounds — to JSON file |
| 39 | `inbounds-import` | Import inbounds — from JSON via API, with backup |
| 40 | `node-link` | Link node to master — connectivity check and what to enter on master |
| 41 | `gh-sync` | Sync code on GitHub — merge upstream AlexeyLCP/lucx-ui, conflicts, push |
| 42 | `self-update` | Self-update / lucx command — install fresh lucx.sh as /usr/local/bin/lucx |

## Clean VPS install (from the GitHub fork)

### Quick install (one command)

As `root` on Ubuntu/Debian (x86_64 or aarch64):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/install-fork.sh)
```

[`scripts/install-fork.sh`](../scripts/install-fork.sh) automates manual steps 1–6 below:

1. installs packages (`curl git tar ca-certificates build-essential xz-utils iproute2`);
2. clones the fork into `/usr/local/src/lucx-ui-samur005` (if it already exists: stashes local changes, then `git reset --hard origin/<branch>`);
3. installs Go into `/usr/local/go` (skipped if Go ≥ the required version is already there; required = max of `GO_VER` and `go.mod`) and Node.js into `/usr/local/lib/nodejs` (skipped if Node ≥ 26 with npm ≥ 11 is already available), adds `PATH` entries in `/etc/profile.d/go.sh` / `nodejs.sh` if no existing `/etc/profile.d` script already has them;
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
| `GO_VER` | `1.27.3` | minimum Go version (raised to `go.mod` if that is higher) |
| `NODE_VER` | `26.12.0` | Node version to install when no suitable Node (≥ 26) exists |
| `LUCX_SKIP_SERVICE` | `0` | `1` = build only (no stock install.sh, no systemd, no binary swap, no `/etc/profile.d` writes) |
| `LUCX_UFW_OPEN` | `0` | `1` = if `ufw` is active, allow the public `x-ui`/xray ports |
| `LUCX_ALLOW_REBOOT` | `0` | `1` = do not intercept the stock installer's reboot |

After PR #1 merges into `main`, replace `feat/native-vk-hash-generator` with `main` in the URL and run with `LUCX_BRANCH=main`.

The manual steps below are the fallback.

### Manual install (fallback)

Full **from scratch** flow on Ubuntu/Debian as `root`.
Upstream `install.sh` creates the service, DB and ports; the **fork binary** (vk-hash + Templates) comes only from building [this repo](https://github.com/samur005/lucx-ui-samur005).

Until [PR #1](https://github.com/samur005/lucx-ui-samur005/pull/1) merges, use branch `feat/native-vk-hash-generator`. After merge, use `main` in every command below.

**Build requirements:** Go **1.27.3+** (`go.mod`), Node.js **≥ 26** and npm **≥ 11**.

**Upstream sync:** the overlay tracks the official [AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui) `main` — the version string comes from `internal/config` (`lucxVersion`); the fork adds no suffix of its own. Last merge: upstream `08c46308` (v3.10.2, lucx.309/310, Go 1.27.3). Upstream's kernel WireGuard backend is optional (per-inbound `kernel` flag in settings, off by default).

### Step 1 — packages

```bash
apt update
apt install -y curl git tar ca-certificates build-essential xz-utils
```

### Step 2 — Go 1.27.3+

```bash
GO_VER=1.27.3
curl -fL "https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz" -o /tmp/go.tgz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=/usr/local/go/bin:$PATH' >/etc/profile.d/go.sh
export PATH=/usr/local/go/bin:$PATH
go version
```

### Step 3 — Node.js 26+

```bash
NODE_VER=26.12.0
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
export PATH="/usr/local/go/bin:/usr/local/lib/nodejs/node-v26.12.0-linux-x64/bin:$PATH"

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

## LucX Panel Android app

`android/` contains a simple WebView wrapper for the panel (package `fun.antibs.lucxpanel`, Android 7.0+): multiple servers (name + full panel URL), backup upload/download, pull-to-refresh, system dark theme, per-server option to trust self-signed certificates. No push notifications.

The APK is built by GitHub Actions (`.github/workflows/android-apk.yml`) and published to the `apk-latest` prerelease:
**[lucx-panel.apk](https://github.com/samur005/lucx-ui-samur005/releases/download/apk-latest/lucx-panel.apk)**

Without `ANDROID_KEYSTORE_*` secrets the APK is signed with a throwaway key, so updating may require uninstalling the previous version.
