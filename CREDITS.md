# Credits / Provenance

## VK Creator (native vk_hash)

In-process VK call creation for qWDTT `vkHashes` was adapted from
[ildarmaga/wdtt](https://github.com/ildarmaga/wdtt) (WDTT panel VK Creator):

- `panel/vk_call_api.go` → `internal/lucx/vkcreator/api.go`
- `panel/vk_creator.go` / store / handlers → `internal/lucx/vkcreator/{creator,store}.go`,
  `internal/web/controller/vk_creator.go`
- `pkg/vkhash` → `internal/lucx/vkcreator/parse.go`

WDTT is licensed under **GPL-3.0**. Only the minimal call/create + cookie
handling needed for native `vk_hash` generation was ported. Those files carry
source attribution headers. The rest of LucX-UI remains under
**PolyForm Noncommercial 1.0.0**.

If you redistribute a build that includes these VK Creator files, review both
licenses for your use case.

## WB Stream room generator (olcRTC, provider wbstream)

`internal/lucx/wbcreator/api.go` (`ParseRoomID`, create-room request shape) is
adapted from [kulikov0/whitelist-bypass](https://github.com/kulikov0/whitelist-bypass)
`relay/wbstream/api.go`. The slide-v3 token refresh, cookie store, controller,
inbound hook and UI were written for this fork. No GPL (WDTT / PWDTT) code is
used for this feature. See [docs/WBROOM.md](docs/WBROOM.md).

whitelist-bypass is distributed under the MIT License:

```
MIT License

Copyright (c) 2026

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
