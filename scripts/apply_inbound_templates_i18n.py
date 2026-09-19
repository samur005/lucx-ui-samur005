#!/usr/bin/env python3
"""Insert pages.inbounds.form.templates.* into every translation locale."""
from __future__ import annotations

import json
from pathlib import Path

BUTTON = {
    "en-US": "Templates",
    "ru-RU": "Шаблоны",
    "zh-CN": "模板",
    "zh-TW": "範本",
    "fa-IR": "قالب‌ها",
    "ar-EG": "قوالب",
    "es-ES": "Plantillas",
    "id-ID": "Template",
    "ja-JP": "テンプレート",
    "pt-BR": "Modelos",
    "tr-TR": "Şablonlar",
    "uk-UA": "Шаблони",
    "vi-VN": "Mẫu",
}
APPLIED = {
    "en-US": "Template applied",
    "ru-RU": "Шаблон применён",
    "zh-CN": "已应用模板",
    "zh-TW": "已套用範本",
    "fa-IR": "قالب اعمال شد",
    "ar-EG": "تم تطبيق القالب",
    "es-ES": "Plantilla aplicada",
    "id-ID": "Template diterapkan",
    "ja-JP": "テンプレートを適用しました",
    "pt-BR": "Modelo aplicado",
    "tr-TR": "Şablon uygulandı",
    "uk-UA": "Шаблон застосовано",
    "vi-VN": "Đã áp dụng mẫu",
}

TEMPLATES_STATIC = {
    "vlessXhttpReality": "VLESS + XHTTP + Reality",
    "vlessGrpcReality": "VLESS + gRPC + Reality",
    "vlessGrpcTls": "VLESS + gRPC + TLS",
    "vlessWsTls": "VLESS + WS + TLS",
    "vlessHttpupgradeTls": "VLESS + HTTPUpgrade + TLS",
    "vlessKcp": "VLESS + KCP",
}


def templates_block(code: str) -> str:
    button = BUTTON.get(code, "Templates").replace("\\", "\\\\").replace('"', '\\"')
    applied = APPLIED.get(code, "Template applied").replace("\\", "\\\\").replace('"', '\\"')
    lines = [
        '      "templates": {',
        f'        "button": "{button}",',
        f'        "applied": "{applied}",',
    ]
    for k, v in TEMPLATES_STATIC.items():
        comma = "," if k != "vlessKcp" else ""
        lines.append(f'        "{k}": "{v}"{comma}')
    lines.append("      },")
    return "\n".join(lines)


def main() -> None:
    tdir = Path("internal/web/translation")
    for path in sorted(tdir.glob("*.json")):
        code = path.stem
        text = path.read_text(encoding="utf-8")
        if '"vlessXhttpReality"' in text:
            print(f"skip {path.name} (already present)")
            continue
        needle = '"autoFill":'
        idx = text.find(needle)
        if idx < 0:
            raise SystemExit(f"autoFill not found in {path}")
        line_end = text.find("\n", idx)
        insertion = text[: line_end + 1] + templates_block(code) + "\n" + text[line_end + 1 :]
        json.loads(insertion)  # validate
        path.write_text(insertion, encoding="utf-8")
        print(f"updated {path.name}")


if __name__ == "__main__":
    main()
