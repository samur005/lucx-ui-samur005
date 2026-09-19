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
