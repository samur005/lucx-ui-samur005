# qWDTT vk-hash autogen (fork samur005)

Стоковый LucX не создаёт VK call hash. Форк при пустом `vkHashes` подставляет hash:

1. `LUCX_VK_HASH`
2. `POST {LUCX_WDTT_URL}/panel/api/vk/call/create`

`/etc/default/x-ui`:

```
LUCX_WDTT_URL=https://turn.antibs.fun:2860/wdtt
LUCX_WDTT_USER=admin
LUCX_WDTT_PASS=...
# или LUCX_VK_HASH=...
```

Чтобы панель на VPS подхватила код: собрать бинарник из этого репо. `x-ui update` с AlexeyLCP этот код затрёт.
