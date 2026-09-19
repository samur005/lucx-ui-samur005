# Fork samur005 / lucx-ui-samur005 — install & update

Repo: https://github.com/samur005/lucx-ui-samur005  
Upstream: https://github.com/AlexeyLCP/lucx-ui  
Full guide (Russian): [FORK.md](FORK.md)

## What this fork adds (vs AlexeyLCP/lucx-ui)

1. **Native vk-hash auto-generation for qWDTT** — Tunnels → qWDTT → **«Генератор vk_hash» / VK hash generator**: paste VK cookies → create call → fill `VkHashes`. Fallbacks: `LUCX_VK_HASH`, optional external `LUCX_WDTT_*`. Package `internal/lucx/vkcreator/`, API `/panel/api/tunnel/vk/*`. See [VKHASH.md](VKHASH.md), [CREDITS.md](../CREDITS.md).
2. **Inbound Templates** — **«Шаблоны» / Templates** on inbound create/edit modal with VLESS presets: XHTTP+Reality, gRPC+Reality, gRPC+TLS, WS+TLS, HTTPUpgrade+TLS, KCP.

Stock `install.sh` / `x-ui update` installs the **upstream** binary **without** these features. Build from this repo to get them.

## Branch

Until [PR #1](https://github.com/samur005/lucx-ui-samur005/pull/1) merges: clone `-b feat/native-vk-hash-generator`. After merge: use `main`.

## Build & replace binary

Needs Go from `go.mod` (currently **1.27.1+**) and Node from `frontend/package.json` (currently **≥ 24**).

```bash
git clone -b feat/native-vk-hash-generator https://github.com/samur005/lucx-ui-samur005.git /usr/local/src/lucx-ui-samur005
cd /usr/local/src/lucx-ui-samur005
cd frontend && npm ci && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui && mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui && systemctl start x-ui
```

If the panel is not installed yet, run upstream `install.sh` (or follow project service docs), then replace the binary as above.

Optional env in `/etc/default/x-ui`: `LUCX_VK_HASH`, `LUCX_WDTT_URL` / `LUCX_WDTT_USER` / `LUCX_WDTT_PASS`.

## Update without losing the fork

- Do **not** GitHub **Sync fork → Discard commits**.
- Do **not** rely on `x-ui update` alone (replaces with stock binary).
- Recommended: `git remote add upstream https://github.com/AlexeyLCP/lucx-ui.git`, `git fetch upstream`, `git merge upstream/main`, resolve conflicts keeping `EnsureVkHashes` / `vkcreator` / Templates files, `git push`, rebuild frontend+Go, replace binary.
- Hash-hook recovery only: `scripts/apply-vkhash.sh` (see Russian doc for curl one-liner).

Details and tables: [FORK.md](FORK.md).
