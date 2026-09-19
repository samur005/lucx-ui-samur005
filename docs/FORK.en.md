# Fork samur005 / lucx-ui-samur005 — install & update

Repo: https://github.com/samur005/lucx-ui-samur005  
Upstream: https://github.com/AlexeyLCP/lucx-ui  
Full guide (Russian): [FORK.md](FORK.md)

## What this fork adds (vs AlexeyLCP/lucx-ui)

1. **Native vk-hash auto-generation for qWDTT** — Tunnels → qWDTT → **«Генератор vk_hash» / VK hash generator**: paste VK cookies → create call → fill `VkHashes`. Fallbacks: `LUCX_VK_HASH`, optional external `LUCX_WDTT_*`. Package `internal/lucx/vkcreator/`, API `/panel/api/tunnel/vk/*`. See [VKHASH.md](VKHASH.md), [CREDITS.md](../CREDITS.md).
2. **Inbound Templates** — **«Шаблоны» / Templates** on inbound create/edit modal with VLESS presets: XHTTP+Reality, gRPC+Reality, gRPC+TLS, WS+TLS, HTTPUpgrade+TLS, KCP.

Stock `install.sh` / panel **Update** / `x-ui update` install the **upstream** binary **without** these features. Build from this repo to get them. Never use those for “updating the fork”.

## Branch

Until [PR #1](https://github.com/samur005/lucx-ui-samur005/pull/1) merges: use `feat/native-vk-hash-generator`. After merge: use `main`.

## Clean VPS install (from the GitHub fork)

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

---

## Build & replace binary (panel already installed)

If `systemctl status x-ui` already works, skip stock `install.sh` and run step 5 only. Install Go/Node (steps 2–3) first if missing.

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

On conflicts: keep `EnsureVkHashes`, `internal/lucx/tunnel/vkhash.go`, `internal/lucx/vkcreator/`, inbound Templates files, and `docs/FORK.md` — then add / commit / push.

GitHub **Sync fork → Update** is OK; **Discard** is forbidden.

### Part 2 — rebuild panel on VPS (ports unchanged)

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

**UI check again:** Tunnels → qWDTT → vk hash generator; Inbounds → Templates.

Hash-hook-only recovery: `scripts/apply-vkhash.sh` (see Russian doc for curl one-liner), then Part 2.

Full tables and notes: [FORK.md](FORK.md).
