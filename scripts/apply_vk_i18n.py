#!/usr/bin/env python3
"""Apply VK i18n keys to en-US/ru-RU and refresh QwdttVkPanel.tsx."""
from __future__ import annotations

import json
from pathlib import Path

ROOT = Path(".")
patch = json.loads((ROOT / "scripts" / "vk-i18n-patch.json").read_text(encoding="utf-8"))


def patch_locale(path: Path, vk: dict, tip: str) -> None:
    data = json.loads(path.read_text(encoding="utf-8"))
    q = data["pages"]["tunnels"]["qwdtt"]
    q["form"]["vkHashesTip"] = tip
    new_q: dict = {}
    for k, v in q.items():
        new_q[k] = v
        if k == "toasts":
            new_q["vk"] = vk
    if "vk" not in new_q:
        new_q["vk"] = vk
    data["pages"]["tunnels"]["qwdtt"] = new_q
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"patched {path} title={vk['title']!r}")


en = ROOT / "internal/web/translation/en-US.json"
ru = ROOT / "internal/web/translation/ru-RU.json"
if en.stat().st_size < 1000:
    raise SystemExit(f"en-US.json still corrupt size={en.stat().st_size}")

patch_locale(en, patch["en_vk"], patch["en_tip"])
patch_locale(ru, patch["ru_vk"], patch["ru_tip"])

(ROOT / "frontend/src/pages/tunnels/QwdttVkPanel.tsx").write_text(patch["panel"], encoding="utf-8")
print("wrote QwdttVkPanel.tsx")

probe = ROOT / "tmp-i18n-size-probe.txt"
if probe.exists():
    probe.unlink()
    print("removed probe")
