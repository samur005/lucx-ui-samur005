# qWDTT vk-hash autogen

Полный текст: [VKHASH.md](VKHASH.md)

При пустом `vkHashes`:

1. `LUCX_VK_HASH`
2. **native** — cookies в панели (`lucxVkCookies`) → `calls.start` → hash
3. иначе `POST {LUCX_WDTT_URL}/panel/api/vk/call/create` (внешний WDTT)

В UI: **Tunnels → qWDTT → VK hash generator** — вставить remixsid, Save cookies, Generate vk_hash.

`x-ui update` с AlexeyLCP стирает кастомный бинарник. Исходники держи `git merge upstream/main`.
