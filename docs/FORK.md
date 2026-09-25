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

## Установка на чистый VPS (из GitHub-форка)

### Быстрая установка одной командой

От `root` на Ubuntu/Debian (x86_64 или aarch64):

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/install-fork.sh)
```

Скрипт [`scripts/install-fork.sh`](../scripts/install-fork.sh) делает всё то же, что ручные шаги 1–6 ниже:

1. ставит пакеты (`curl git tar ca-certificates build-essential xz-utils iproute2`);
2. клонирует форк в `/usr/local/src/lucx-ui-samur005` (если каталог уже есть — сохраняет локальные правки в `git stash` и делает `git reset --hard origin/<ветка>`);
3. ставит Go в `/usr/local/go` (пропускает, если там уже Go ≥ нужной версии; версия берётся максимальной из `GO_VER` и `go.mod`) и Node.js в `/usr/local/lib/nodejs` (пропускает, если уже есть Node ≥ 22.12 с npm), при необходимости прописывает `PATH` в `/etc/profile.d/go.sh` и `nodejs.sh` (если PATH уже прописан в другом файле `/etc/profile.d` — не трогает);
4. **сначала собирает** frontend и Go-бинарник в `/usr/local/src/lucx-ui-samur005/.install-fork/x-ui` — если сборка упала, скрипт выходит с ошибкой и **ничего не трогает** (ни сервис, ни текущий бинарник);
5. если панели ещё нет (`/usr/local/x-ui/x-ui` + unit `x-ui`) — запускает стоковый `install.sh` AlexeyLCP (интерактивный: вопросы про порт/SSL, в конце URL, логин и пароль — **сохраните их**); если панель уже стоит — этот шаг пропускается;
6. останавливает `x-ui`, делает бэкап `/usr/local/x-ui/x-ui.bak-<дата>`, ставит бинарник форка, запускает и проверяет `systemctl is-active x-ui` (если не поднялся — автоматически откатывает на бэкап);
7. печатает версию, порты, которые слушают `x-ui`/xray, и — если включён `ufw` — какие из публичных портов ещё не открыты (сам открывает только с `LUCX_UFW_OPEN=1`).

**Повторный запуск той же команды = обновление** существующей панели до свежей сборки форка (стоковый `install.sh` при этом не запускается, БД и порты не меняются).

**Перезагрузка.** Стоковый `install.sh` на свежем сервере может поставить новое ядро для AmneziaWG и сам перезагрузить сервер через 10 секунд. Скрипт форка перехватывает эту перезагрузку, сначала ставит бинарник форка и в конце просит выполнить `reboot` вручную. Если сервер всё же перезагрузился посреди установки (например, с `LUCX_ALLOW_REBOOT=1`), просто запустите ту же команду ещё раз — стоковый шаг будет пропущен, скрипт соберёт и поставит бинарник форка.

Переменные окружения (необязательно), например `LUCX_BRANCH=main bash <(curl -fsSL …)`:

| Переменная | По умолчанию | Смысл |
| --- | --- | --- |
| `LUCX_FORK_REPO` | `https://github.com/samur005/lucx-ui-samur005.git` | git-URL форка |
| `LUCX_BRANCH` | `feat/native-vk-hash-generator` | ветка для сборки (после merge PR #1 — `main`) |
| `LUCX_SRC` | `/usr/local/src/lucx-ui-samur005` | каталог с исходниками |
| `GO_VER` | `1.27.1` | минимальная версия Go (если `go.mod` требует больше — ставится версия из `go.mod`) |
| `NODE_VER` | `24.20.0` | какую версию Node ставить, если подходящей (≥ 22.12) нет |
| `LUCX_SKIP_SERVICE` | `0` | `1` — только сборка (без стокового install.sh, systemd, замены бинарника и записи в `/etc/profile.d`) |
| `LUCX_UFW_OPEN` | `0` | `1` — при активном `ufw` открыть публичные порты `x-ui`/xray |
| `LUCX_ALLOW_REBOOT` | `0` | `1` — не перехватывать перезагрузку стокового install.sh |

После merge PR #1 в `main` в URL команды замените `feat/native-vk-hash-generator` на `main` и запускайте с `LUCX_BRANCH=main`.

Ручная установка по шагам ниже — запасной вариант, если скрипт не подходит.

### Ручная установка (запасной вариант)

Полный сценарий **с нуля** на Ubuntu/Debian от `root`.
Сток `install.sh` поднимает сервис, БД и порты; **бинарник с vk-hash и «Шаблоны»** ставится только сборкой из [форка](https://github.com/samur005/lucx-ui-samur005).

Пока открыт [PR #1](https://github.com/samur005/lucx-ui-samur005/pull/1) — ветка `feat/native-vk-hash-generator`. После merge в `main` замените имя ветки на `main` во всех командах ниже.

**Требования для сборки:** Go **1.27.1+** (`go.mod`), Node.js **≥ 24** и npm **≥ 10** (`frontend/package.json` / `.nvmrc`).

### Шаг 1 — пакеты

```bash
apt update
apt install -y curl git tar ca-certificates build-essential xz-utils
```

### Шаг 2 — Go 1.27.1+

```bash
GO_VER=1.27.1
curl -fL "https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz" -o /tmp/go.tgz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=/usr/local/go/bin:$PATH' >/etc/profile.d/go.sh
export PATH=/usr/local/go/bin:$PATH
go version
```

### Шаг 3 — Node.js 24+

Официальный бинарник (путь потом используется в `export PATH`):

```bash
NODE_VER=24.20.0
curl -fL "https://nodejs.org/dist/v${NODE_VER}/node-v${NODE_VER}-linux-x64.tar.xz" -o /tmp/node.tar.xz
mkdir -p /usr/local/lib/nodejs
tar -C /usr/local/lib/nodejs -xJf /tmp/node.tar.xz
export PATH="/usr/local/lib/nodejs/node-v${NODE_VER}-linux-x64/bin:$PATH"
echo "export PATH=/usr/local/lib/nodejs/node-v${NODE_VER}-linux-x64/bin:\$PATH" >/etc/profile.d/nodejs.sh
node -v
npm -v
```

### Шаг 4 — базовый сервис панели (сток AlexeyLCP)

Нужен, чтобы появились `/usr/local/x-ui/`, systemd unit, БД `/etc/x-ui/` и порты. **Это ещё не форк.**

```bash
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
```

Сохраните из вывода скрипта: URL панели (порт и `webBasePath`), логин и пароль.
По умолчанию часто встречаются порты вроде **29830** (панель) и **2096** (подписка) — смотрите фактический вывод `install.sh`.

### Шаг 5 — клон форка с GitHub и сборка бинарника с фичами

```bash
export PATH="/usr/local/go/bin:/usr/local/lib/nodejs/node-v24.20.0-linux-x64/bin:$PATH"

git clone -b feat/native-vk-hash-generator \
  https://github.com/samur005/lucx-ui-samur005.git \
  /usr/local/src/lucx-ui-samur005
cd /usr/local/src/lucx-ui-samur005

# sanity-check фич форка
test -d internal/lucx/vkcreator
grep -n EnsureVkHashes internal/lucx/tunnel/qwdtt_inbound.go || true

cd frontend && npm ci && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui
cp -a /usr/local/x-ui/x-ui "/usr/local/x-ui/x-ui.bak-$(date +%Y%m%d%H%M)"
mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui
chmod +x /usr/local/x-ui/x-ui
systemctl start x-ui
systemctl status x-ui --no-pager
```

После merge PR #1 в `main`:

```bash
git clone -b main https://github.com/samur005/lucx-ui-samur005.git /usr/local/src/lucx-ui-samur005
# дальше те же npm ci / go build / замена бинарника
```

### Шаг 6 — firewall (если включён ufw)

Откройте порт панели и подписки из шага 4 (подставьте свои):

```bash
ufw allow 29830/tcp
ufw allow 2096/tcp
ufw reload
```

### Шаг 7 — проверка

1. Откройте URL из вывода `install.sh` (или `https://IP:ПОРТ/webBasePath/`).
2. **Туннели → qWDTT → «Генератор vk_hash»**.
3. **Inbounds → создать/редактировать → «Шаблоны»**.

---

## Установка, если панель уже стоит (только заменить бинарник форком)

Проще всего — [команда быстрой установки](#быстрая-установка-одной-командой): на существующей панели она пропускает стоковый `install.sh`, сама ставит недостающие Go/Node, собирает форк и меняет бинарник (с бэкапом). Ниже — то же вручную.

Уже есть `systemctl status x-ui` и каталог `/usr/local/x-ui` — достаточно клона и сборки (шаг 5 выше), без повторного `install.sh`.

Пока PR #1 не влит:

```bash
export PATH="/usr/local/go/bin:/usr/local/lib/nodejs/node-v24.20.0-linux-x64/bin:$PATH"
git clone -b feat/native-vk-hash-generator \
  https://github.com/samur005/lucx-ui-samur005.git \
  /usr/local/src/lucx-ui-samur005
cd /usr/local/src/lucx-ui-samur005
cd frontend && npm ci && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui
cp -a /usr/local/x-ui/x-ui "/usr/local/x-ui/x-ui.bak-$(date +%Y%m%d%H%M)"
mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui
chmod +x /usr/local/x-ui/x-ui
systemctl start x-ui
systemctl status x-ui --no-pager
```

Если Go/Node ещё не установлены — выполните шаги 2–3 из раздела «чистый VPS».


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

**Одной командой:** та же команда быстрой установки делает всю Часть 2 — `git fetch` + `reset --hard` на свежий `origin/<ветка>`, сборку frontend + Go, бэкап и замену бинарника, проверку `systemctl is-active x-ui`:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/install-fork.sh)
```

Локальные правки в `/usr/local/src/lucx-ui-samur005` скрипт убирает в `git stash` (вернуть: `git stash pop`), поэтому сначала запушьте результат Части 1 в форк. Вручную — так:

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
