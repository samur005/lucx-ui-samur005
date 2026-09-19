#!/usr/bin/env bash
# Re-apply qWDTT vk-hash autogen onto a LucX source tree.
set -euo pipefail
REPO_URL="${REPO_URL:-https://github.com/samur005/lucx-ui-samur005.git}"
SRC="${SRC:-/usr/local/src/lucx-ui-samur005}"
need() { command -v "$1" >/dev/null || { echo "need $1"; exit 1; }; }
need git
if [[ ! -d "$SRC/.git" ]]; then
  mkdir -p "$(dirname "$SRC")"
  git clone "$REPO_URL" "$SRC"
else
  git -C "$SRC" fetch origin
  git -C "$SRC" checkout main
  git -C "$SRC" pull --ff-only origin main || true
fi
INB="$SRC/internal/lucx/tunnel/qwdtt_inbound.go"
[[ -f "$INB" ]] || { echo "missing $INB"; exit 1; }
git -C "$SRC" checkout origin/main -- internal/lucx/tunnel/vkhash.go 2>/dev/null || true
[[ -f "$SRC/internal/lucx/tunnel/vkhash.go" ]] || { echo "vkhash.go missing"; exit 1; }
if ! grep -q 'EnsureVkHashes' "$INB"; then
  python3 - "$INB" <<'PY'
import pathlib, sys
p = pathlib.Path(sys.argv[1])
t = p.read_text()
old = "\treturn cfg.Merge(), true"
new = """\tcfg = cfg.Merge()\n\tif c2, err := cfg.EnsureVkHashes(); err == nil {\n\t\tcfg = c2\n\t}\n\treturn cfg, true"""
if old in t:
    p.write_text(t.replace(old, new, 1))
    print("hook inserted")
else:
    print("WARN: insert EnsureVkHashes by hand")
PY
else
  echo "hook already present"
fi
echo "patched $SRC"
