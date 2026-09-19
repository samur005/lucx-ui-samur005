#!/usr/bin/env python3
import base64, gzip
from pathlib import Path
b64 = Path("scripts/payloads/InboundFormModal.tsx.gz.b64").read_text().strip()
data = gzip.decompress(base64.b64decode(b64))
Path("frontend/src/pages/inbounds/form/InboundFormModal.tsx").write_bytes(data)
print("decoded", len(data), "bytes")
