# LucX fork samur005 — vk-hash, установка, обновления

Репозиторий: https://github.com/samur005/lucx-ui-samur005  
Апстрим: https://github.com/AlexeyLCP/lucx-ui

## Что уже в форке

- `internal/lucx/tunnel/vkhash.go` — `EnsureVkHashes()`
- хук в `internal/lucx/tunnel/qwdtt_inbound.go`
- `scripts/apply-vkhash.sh` — повторно вставить патч после merge

Стоковый LucX **не генерирует** VK call hash. Поле `vkHashes` у qWDTT пустое → подписка пустая.  
Форк при пустом поле подставляет hash из:

1. `LUCX_VK_HASH`
2. native VK Creator (cookies в панели → `calls.start`)
3. иначе `POST {LUCX_WDTT_URL}/panel/api/vk/call/create`

AntiBS2vpn бот ходит в **тот же HTTP API** панели (`/login`, inbound/client, `/sub/{id}`). Отдельного «API форка» нет. Переключение бота = тот же URL панели LucX в карточке локации.

## Где в меню панели окно / поле VK

В LucX **нет отдельного пункта «Авторизация VK»** в боковом меню.

Живые hash вводятся здесь:

1. **Inbounds (Инбаунды)** → inbound протокола **qWDTT** → поле **VK hashes** (`settings.vkHashes`).
2. **Tunnels (Туннели)** → карточка **qWDTT** → то же поле **VK hashes**.

**Native (этот форк):** Tunnels → qWDTT → блок **VK hash generator**
(или API `/panel/api/tunnel/vk/*`). Cookies → create → поле `vkHashes`.

Внешний WDTT по-прежнему опционален:

- URL вида `https://turn.…/wdtt/`
- `POST /panel/api/vk/call/create` → `{ vk_hash }`

Цепочка:

```
[native cookies | LUCX_VK_HASH | WDTT] → LucX qWDTT.vkHashes → /sub/… → клиент
```

Бот AntiBS2vpn умеет сам дописать `vkHashes` в inbound LucX, даже если бинарник панели стоковый.

## Установка панели с форка

### Вариант A — релиз AlexeyLCP

У форка **нет GitHub Releases**. `install.sh` даже из форка качает tar.gz AlexeyLCP.

```bash
bash <(curl -Ls https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Стоковый бинарник. Go-автоген в бинарнике не попадет. Подписки заполняет бот.

### Вариант B — сборка из этого форка

Нужны **Go ≥ версии из `go.mod` (сейчас 1.27+)** и Node 20.

```bash
git clone https://github.com/samur005/lucx-ui-samur005.git /usr/local/src/lucx-ui-samur005
cd /usr/local/src/lucx-ui-samur005
test -f internal/lucx/tunnel/vkhash.go
grep -n EnsureVkHashes internal/lucx/tunnel/qwdtt_inbound.go

cd frontend && npm ci && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui
mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui
systemctl start x-ui
```

`/etc/default/x-ui`:

```
LUCX_WDTT_URL=https://turn.example:2860/wdtt
LUCX_WDTT_USER=admin
LUCX_WDTT_PASS=...
# или LUCX_VK_HASH=...
```

## Как обновляться с AlexeyLCP и не затереть vk-hash

| Действие | Исходники форка | Бинарник на VPS |
|---|
| Update в панели / `x-ui update` | не трогает GitHub | **затирает** свой бинарник |
| GitHub Sync fork → Discard | **стирает** `vkhash.go` | не трогает VPS |
| `git merge upstream/main` | патч остаётся, если не выкинешь файлы | не трогает VPS |

```bash
cd lucx-ui-samur005
git remote add upstream https://github.com/AlexeyLCP/lucx-ui.git
git fetch upstream
git merge upstream/main
# конфликт в qwdtt_inbound.go — оставь EnsureVkHashes
git push origin main
```

Не жми «Sync fork / Discard my commits».

После merge: либо `x-ui update` (сток + бот пишет hash), либо снова `go build` из форка.

## Если код стёрся

```bash
curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/main/scripts/apply-vkhash.sh | bash
```

Хук:

```go
cfg = cfg.Merge()
if c2, err := cfg.EnsureVkHashes(); err == nil {
    cfg = c2
}
return cfg, true
```
