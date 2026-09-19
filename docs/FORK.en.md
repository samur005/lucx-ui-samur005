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

## Build & replace binary (first install)

Needs Go from `go.mod` and Node from `frontend/package.json` (adjust `PATH` if Node lives elsewhere on the VPS).

```bash
git clone -b feat/native-vk-hash-generator https://github.com/samur005/lucx-ui-samur005.git /usr/local/src/lucx-ui-samur005
cd /usr/local/src/lucx-ui-samur005
cd frontend && npm ci && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui && mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui && systemctl start x-ui
```

If the panel is not installed yet, run upstream `install.sh` first, then replace the binary as above.

Optional env in `/etc/default/x-ui`: `LUCX_VK_HASH`, `LUCX_WDTT_URL` / `LUCX_WDTT_USER` / `LUCX_WDTT_PASS`.

**UI check:** Tunnels → qWDTT → vk hash generator; Inbounds → Templates («Шаблоны»).

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
