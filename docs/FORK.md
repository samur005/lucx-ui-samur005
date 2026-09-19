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

**Никогда не используйте** кнопку Update в панели, `x-ui update` или стоковый `install.sh` для «обновления форка» — они поставят бинарник апстрима без фич.

---

## Установка (сборка из форка)

**Требования:**

- Go по версии из `go.mod` (сейчас **1.27.1+**)
- Node.js по `frontend/package.json` / `.nvmrc` (сейчас **≥ 24**, npm ≥ 10) — на VPS путь к Node может отличаться; при необходимости поправьте `PATH`
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

## Когда у AlexeyLCP вышел новый релиз — обновление форка без потери фич, затем VPS

Канонический порядок из двух частей. Команды ниже — на VPS (или любом клоне) от root в каталоге `/usr/local/src/lucx-ui-samur005`. Пока открыт PR #1 используйте ветку `feat/native-vk-hash-generator`; после merge в `main` замените имя ветки на `main`.

| Действие | Исходники на GitHub | Бинарник на VPS |
| --- | --- | --- |
| Кнопка Update в панели / `x-ui update` / стоковый `install.sh` | не трогает ваш fork | **затирает** кастомный бинарник стоком AlexeyLCP |
| GitHub **Sync fork → Discard commits** | **стирает** коммиты форка (`vkcreator`, Templates, …) | VPS не трогает |
| **Часть 1:** `git merge upstream/main` + push (сохранить фичи при конфликтах) | фичи остаются | VPS не трогает |
| **Часть 2:** сборка frontend + Go и замена бинарника | не меняет GitHub | порты и БД сохраняются |

### Не делайте

- **Sync fork → Discard commits** на GitHub. Допустимо только **Sync fork → Update** (если интерфейс предлагает Update без Discard).
- Update в панели / `x-ui update` / стоковый `install.sh` — после этого снова нужна сборка из форка (или останетесь на стоке без фич).

### Часть 1 — подтянуть апстрим AlexeyLCP, сохранив vk-hash и шаблоны inbound

```bash
cd /usr/local/src/lucx-ui-samur005
git remote add upstream https://github.com/AlexeyLCP/lucx-ui.git 2>/dev/null || true
git fetch origin
git fetch upstream --tags
git checkout feat/native-vk-hash-generator
git pull --ff-only origin feat/native-vk-hash-generator
git merge upstream/main
```

**При успешном merge без конфликтов:**

```bash
git push origin feat/native-vk-hash-generator
```

**При конфликтах** разрешите их, **сохранив** файлы и логику форка:

- `EnsureVkHashes` и `internal/lucx/tunnel/vkhash.go`
- `internal/lucx/vkcreator/`
- файлы inbound Templates (кнопка «Шаблоны», пресеты, i18n)
- `docs/FORK.md`

Затем `git add` → `git commit` → `git push origin feat/native-vk-hash-generator`.

На GitHub: **Sync fork → Update** — допустимо; **Discard** — запрещено.

После merge PR #1 в `main` в командах выше используйте `main` вместо `feat/native-vk-hash-generator`.

### Часть 2 — пересобрать панель на VPS (порты не меняются)

Если путь к Node на сервере другой — поправьте `PATH` (пример ниже — типичный для установки Node 22 в `/usr/local/lib/nodejs/…`; ориентируйтесь на версию из `frontend/package.json` / `.nvmrc`).

```bash
export PATH="/usr/local/lib/nodejs/node-v22.20.0-linux-x64/bin:$PATH"
cd /usr/local/src/lucx-ui-samur005
git checkout feat/native-vk-hash-generator
git pull --ff-only origin feat/native-vk-hash-generator
cd frontend && npm install --no-audit --no-fund && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui
cp -a /usr/local/x-ui/x-ui "/usr/local/x-ui/x-ui.bak-$(date +%Y%m%d%H%M)"
mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui
chmod +x /usr/local/x-ui/x-ui
systemctl start x-ui
systemctl is-active x-ui
ss -tlnp | grep -E '29830|2096'
```

После merge PR #1 в `main` в checkout/pull используйте `main`.

**Проверка UI после пересборки:**

1. **Туннели → qWDTT → генератор vk_hash** — cookies, генерация, заполнение `VkHashes`.
2. **Inbounds → «Шаблоны»** — пресеты подставляются.

### Если стёрся только хук EnsureVkHashes

Скрипт восстановления hash-хука (не поднимает Templates и полный vkcreator с нуля — для этого нужен git):

```bash
curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/apply-vkhash.sh | bash
```

После merge в `main` URL можно заменить на `.../main/scripts/apply-vkhash.sh`. Затем снова выполните **Часть 2** (сборка и замена бинарника).

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
