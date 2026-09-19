#!/usr/bin/env python3
import base64, gzip
from pathlib import Path
a = Path("scripts/payloads/InboundFormModal.tsx.gz.b64.a").read_text().strip()
b = Path("scripts/payloads/InboundFormModal.tsx.gz.b64.b").read_text().strip()
data = gzip.decompress(base64.b64decode(a + b))
Path("frontend/src/pages/inbounds/form/InboundFormModal.tsx").write_bytes(data)
print("decoded", len(data), "bytes")
