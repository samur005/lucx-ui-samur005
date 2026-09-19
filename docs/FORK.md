# Форк samur005 / lucx-ui-samur005 — установка и обновление

Репозиторий: https://github.com/samur005/lucx-ui-samur005  
Апстрим: https://github.com/AlexeyLCP/lucx-ui  
English: [FORK.en.md](FORK.en.md)

Этот документ — практическая инструкция по **сборке бинарника с фичами форка**, их использованию и **обновлению без затирания** патчей.

> **Ветка.** Пока открыт [PR #1](https://github.com/samur005/lucx-ui-samur005/pull/1), клонируйте и собирайте ветку `feat/native-vk-hash-generator`. После merge в `main` используйте `main` (см. команды ниже).

---

## Чем форк отличается от апстрима

Стоковый LucX (релизы AlexeyLCP и `install.sh` апстрима) **не включает** перечисленное ниже. Чтобы получить эти возможности, нужна сборка **из этого репозитория**.

### 1. Native автогенерация vk-hash для qWDTT

Панель умеет сама создать VK-звонок и заполнить поле `VkHashes`:

1. **Туннели (Tunnels)** → карточка **qWDTT** → блок **«Генератор vk_hash»**.
2. Вставить cookies (`remixsid=` или полный `Cookie`) → **«Сохранить cookies»**.
3. **«Сгенерировать vk_hash»** → поле `VkHashes` заполняется → сохранить конфиг туннеля.

Держите звонок открытым («завершить у себя», не «для всех»), пока клиентам нужен hash. При необходимости — **«Завершить звонок»**.

**Порядок fallback при пустом `vkHashes` (EnsureVkHashes):**

1. Значение уже в конфиге inbound/туннеля.
2. Переменная окружения `LUCX_VK_HASH`.
3. **Native** — cookies панели (`lucxVkCookies`) → VK `calls.start` → hash.
4. Опционально внешний WDTT: `POST {LUCX_WDTT_URL}/panel/api/vk/call/create` (`LUCX_WDTT_*`).

**Код и API:**

- пакет `internal/lucx/vkcreator/`
- API (под авторизацией панели): `/panel/api/tunnel/vk/*` (`status`, `cookies`, `cookies/clear`, `create`, `stop`)
- хук `EnsureVkHashes` в `internal/lucx/tunnel/`

Подробнее: [VKHASH.md](VKHASH.md), кратко [vkhash-autogen.md](vkhash-autogen.md).  
Происхождение VK Creator (WDTT / GPL-3): [CREDITS.md](../CREDITS.md).

В боковом меню **нет** отдельного пункта «Авторизация VK». Генератор живёт на карточке qWDTT; поле hash также видно в форме inbound протокола qWDTT (**VK hashes**).

### 2. Inbound Templates («Шаблоны»)

В модалке создания/редактирования inbound есть кнопка **«Шаблоны»**. Она подставляет пресет протокола VLESS + транспорт + security:

| Пресет |
| --- |
| VLESS + XHTTP + Reality |
| VLESS + gRPC + Reality |
| VLESS + gRPC + TLS |
| VLESS + WS + TLS |
| VLESS + HTTPUpgrade + TLS |
| VLESS + KCP |

После выбора шаблона форма вызывает штатные обработчики сети/безопасности (keypair Reality, TLS, KCP finalmask и т.п.). Дальше заполните порт, SNI/dest и остальные поля как обычно.

---

## Важно: `install.sh` апстрима ≠ бинарник форка

Скрипт установки AlexeyLCP (и даже `install.sh` из клона форка, если он качает релиз апстрима) ставит **стоковый** бинарник **без** native vk-hash и **без** кнопки «Шаблоны».

Чтобы получить фичи форка, нужно **собрать frontend + Go из этого репо** и заменить `/usr/local/x-ui/x-ui`.

Если панели ещё нет: сначала поставьте сток (`install.sh` апстрима) **или** следуйте сервисным инструкциям проекта, затем замените бинарник сборкой форка. База (`/etc/x-ui/`), сервис и порты при замене бинарника сохраняются.

---

## Установка (сборка из форка)

**Требования:**

- Go по версии из `go.mod` (сейчас **1.27.1+**)
- Node.js по `frontend/package.json` / `.nvmrc` (сейчас **≥ 24**, npm ≥ 10)
- Уже установленный сервис `x-ui` **или** готовность поставить сток, затем заменить бинарник

### Клон и сборка

Пока PR #1 не влит:

```bash
git clone -b feat/native-vk-hash-generator https://github.com/samur005/lucx-ui-samur005.git /usr/local/src/lucx-ui-samur005
cd /usr/local/src/lucx-ui-samur005

# sanity-check фич форка
test -d internal/lucx/vkcreator
grep -n EnsureVkHashes internal/lucx/tunnel/qwdtt_inbound.go || true

cd frontend && npm ci && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui
mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui
systemctl start x-ui
systemctl status x-ui --no-pager
```

После merge PR #1 в `main`:

```bash
git clone -b main https://github.com/samur005/lucx-ui-samur005.git /usr/local/src/lucx-ui-samur005
# дальше те же npm ci / go build / замена бинарника
```

Если каталог `/usr/local/x-ui` ещё не существует — сначала выполните установку стока:

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

затем повторите сборку и `mv` бинарника из форка.

### Опциональные переменные (`/etc/default/x-ui`)

```bash
# готовый hash без native/WDTT
# LUCX_VK_HASH=...

# внешний WDTT как запасной путь (не обязателен при native cookies)
# LUCX_WDTT_URL=https://turn.example:2860/wdtt
# LUCX_WDTT_USER=admin
# LUCX_WDTT_PASS=...
```

После правок env: `systemctl restart x-ui`.

### Быстрая проверка в UI

1. **Туннели → qWDTT → «Генератор vk_hash»** — сохранить cookies, сгенерировать, убедиться что `VkHashes` заполнилось.
2. **Inbounds → создать/редактировать → «Шаблоны»** — выбрать пресет, убедиться что network/security подставились.

---

## Обновление без потери форка

| Действие | Исходники на GitHub | Бинарник на VPS |
| --- | --- | --- |
| Кнопка Update в панели / `x-ui update` | не трогает ваш fork | **затирает** кастомный бинарник стоком AlexeyLCP |
| GitHub **Sync fork → Discard commits** | **стирает** коммиты форка (`vkcreator`, Templates, …) | VPS не трогает |
| `git fetch upstream` + `git merge upstream/main` | фичи остаются, если при конфликтах их сохранить | VPS не трогает — нужна повторная сборка |

### Не делайте

- **Sync fork → Discard commits** на GitHub.
- Полагаться только на `x-ui update` / Update в панели — после этого снова нужна сборка из форка (или останетесь на стоке без фич).

### Рекомендуемый путь (подтянуть апстрим, сохранить форк)

```bash
cd /usr/local/src/lucx-ui-samur005

git remote add upstream https://github.com/AlexeyLCP/lucx-ui.git 2>/dev/null || true
git fetch upstream

# пока фичи только в ветке PR:
git checkout feat/native-vk-hash-generator
git merge upstream/main
# после merge PR #1: работайте из main и мержите туда же

# при конфликтах сохраните:
#   - EnsureVkHashes / internal/lucx/tunnel/vkhash.go
#   - internal/lucx/vkcreator/
#   - UI Templates (InboundTemplatesButton, inbound-templates.ts, i18n keys)
#   - QwdttVkPanel и API /panel/api/tunnel/vk/*

git push origin HEAD

cd frontend && npm ci && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui && mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui && systemctl start x-ui
```

### Если стёрся только хук EnsureVkHashes

Скрипт восстановления hash-хука (не поднимает Templates и полный vkcreator с нуля — для этого нужен git):

```bash
curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/apply-vkhash.sh | bash
```

После merge в `main` URL можно заменить на `.../main/scripts/apply-vkhash.sh`. Затем снова `go build` и замена бинарника.

---

## Связанные документы

| Документ | О чём |
| --- | --- |
| [VKHASH.md](VKHASH.md) | Детали vk-hash, меню, env, AntiBS |
| [vkhash-autogen.md](vkhash-autogen.md) | Краткая шпаргалка autogen |
| [UPDATE-VKHASH.md](UPDATE-VKHASH.md) | Только про обновления и риск затирания |
| [CREDITS.md](../CREDITS.md) | Provenance WDTT → `vkcreator` |
| [README.ru_RU.md](../README.ru_RU.md) | Короткая витрина форка |

Апстримная документация панели (общий быстрый старт, Docker и т.д.) — в корневом [README.md](../README.md).
