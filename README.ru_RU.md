<!-- LUCX-HOOK: Russian fork landing. Keep in sync with docs/FORK.md. -->
# LucX-UI — форк samur005

Репозиторий: [samur005/lucx-ui-samur005](https://github.com/samur005/lucx-ui-samur005)  
Апстрим: [AlexeyLCP/lucx-ui](https://github.com/AlexeyLCP/lucx-ui)

Это форк панели LucX с двумя практическими доработками поверх апстрима. Общая документация панели (быстрый старт, Docker, лицензия) — в корневом [README.md](README.md). Ниже — только то, что добавляет **этот** форк.

## Что добавлено

1. **Native генератор vk_hash для qWDTT** — cookies VK в панели → живой звонок → заполнение `VkHashes` (Туннели → qWDTT → **«Генератор vk_hash»**). Запасные пути: `LUCX_VK_HASH` и опциональный внешний WDTT (`LUCX_WDTT_*`). Код: `internal/lucx/vkcreator/`, API `/panel/api/tunnel/vk/*`.
2. **Шаблоны inbound** — кнопка **«Шаблоны»** в модалке создания/редактирования inbound с пресетами VLESS (XHTTP/gRPC/WS/HTTPUpgrade/KCP + Reality/TLS).

## Установка с фичами форка

Стоковый `install.sh` / `x-ui update` / Update в панели качают **бинарник апстрима без этих фич**. Нужна сборка из этого репозитория.

Пока открыт [PR #1](https://github.com/samur005/lucx-ui-samur005/pull/1), клонируйте ветку `feat/native-vk-hash-generator` (после merge — `main`):

```bash
git clone -b feat/native-vk-hash-generator https://github.com/samur005/lucx-ui-samur005.git /usr/local/src/lucx-ui-samur005
cd /usr/local/src/lucx-ui-samur005
cd frontend && npm ci && npm run build && cd ..
go build -o /usr/local/x-ui/x-ui.new .
systemctl stop x-ui && mv /usr/local/x-ui/x-ui.new /usr/local/x-ui/x-ui && systemctl start x-ui
```

Go — версия из `go.mod`; Node — из `frontend/package.json` / `.nvmrc`. Если панели ещё нет — сначала стоковый `install.sh`, затем замена бинарника.

**Полная инструкция (установка, env, обновление без затирания):** → **[docs/FORK.md](docs/FORK.md)**  
English summary: [docs/FORK.en.md](docs/FORK.en.md)

## Обновления (когда вышел релиз AlexeyLCP)

Канонический плейбук из двух частей — в **[docs/FORK.md](docs/FORK.md)** (раздел «Когда у AlexeyLCP вышел новый релиз»):

1. **Часть 1** — `git merge upstream/main` на ветке форка, сохранить vk-hash + шаблоны; push. Sync fork → Update OK; Discard запрещён.
2. **Часть 2** — на VPS: `npm` build + `go build`, бэкап старого бинарника, замена `/usr/local/x-ui/x-ui`, проверка портов и UI (Туннели → qWDTT → генератор vk_hash; Inbounds → Шаблоны).

Не используйте Update в панели / `x-ui update` / стоковый `install.sh` для обновления форка.

## Документация по vk-hash

- [docs/VKHASH.md](docs/VKHASH.md) — подробно  
- [docs/vkhash-autogen.md](docs/vkhash-autogen.md) — кратко  
- [CREDITS.md](CREDITS.md) — provenance WDTT  

[English upstream README](docs/readme/README.en_US.md) · [Русский README апстрима](README.md)
<!-- END LUCX-HOOK -->
