# Генератор комнат WB Stream для olcRTC

Форк samur005 · English summary — [ниже](#english-summary)

Туннель **olcRTC** с провайдером **WB Stream** (`provider: wbstream`) заходит в видеокомнату
[stream.wb.ru](https://stream.wb.ru) как тихий участник. olcrtc подключается к комнате как гость,
а **создать** комнату гостю WB не даёт (`400 Guests are not allowed to create room`). Раньше ID комнаты
приходилось создавать руками на сайте и вставлять в инбаунд. Теперь панель создаёт комнату сама
по сохранённой сессии stream.wb.ru и записывает ID прямо в настройки инбаунда olcRTC.

> Это **не** WB-режим qWDTT/WDTT. Клиент qWDTT (SpaceNeuroX) WB не поддерживает. Генератор нужен
> только туннелю olcRTC; клиенты — olcbox / owenclave.

## Как пользоваться

### 1. Сохранить сессию WB (один раз)

**Настройки панели → Ядра → «Открыть конфиги туннелей»** (страница `/tunnels`) → карточка **olcRTC** →
блок **«Генератор комнат WB Stream»**:

1. Вставьте в поле **«Сессия stream.wb.ru (Cookie или Bearer-токен)»** то, что скопировали из браузера
   (см. [где взять cookies](#где-взять-cookies-stream-wb-ru)).
2. **«Сохранить сессию»**. Справа появится зелёный тег **«Сессия WB сохранена»**, ниже —
   имена сохранённых cookies (значения панель обратно не показывает).
3. **«Очистить»** удаляет сессию из панели.

### 2. Создать комнату

**Вариант А — из карточки olcRTC (сразу в инбаунд).**
В списке **«Куда записать ID комнаты»** выберите инбаунд olcRTC с провайдером WB Stream
(по умолчанию выбран первый такой) → **«Создать комнату»**. Панель:

- создаёт комнату на stream.wb.ru;
- пишет её ID в `settings.roomId` выбранного инбаунда (в БД, не в памяти);
- в течение ~10 секунд reconcile-задача перезапускает процесс olcrtc с новой комнатой.

Ссылка `olcrtc://…` содержит ID комнаты, поэтому **клиентам нужно выдать новую ссылку/QR**
(подписка обновится сама). Инбаунды с другим провайдером и инбаунды на удалённых нодах
в списке неактивны — для нод используйте вариант Б.

Пункт **«В поле „URL / ID комнаты“ формы ниже»** только подставляет ID (и провайдер WB Stream)
в старую форму карточки — затем нажмите **«Сохранить»**.

**Вариант Б — из формы инбаунда.**
**Инбаунды → создать/редактировать → протокол olcRTC → Провайдер: WB Stream** → под полем
**«Room ID / URL»** кнопка **«Создать комнату»**: ID подставляется в поле, затем сохраните инбаунд.

**Вариант В — автоматически.** Если провайдер WB Stream, поле комнаты пустое и сессия WB сохранена,
комната создаётся сама при сохранении инбаунда. Ошибка WB сохранение не блокирует (инбаунд просто
останется без комнаты, как раньше; причина — в логе панели). Вставленная ссылка
`https://stream.wb.ru/room/<id>` или `wbstream://<id>` при сохранении сводится к голому `<id>`.

Последние 10 созданных комнат видны внизу блока (ID, время, в какой инбаунд записана).

## Где взять cookies stream.wb.ru

1. Откройте https://stream.wb.ru в Chrome/Edge/Firefox и войдите (телефон + код из SMS).
   Проверьте, что вручную можете создать встречу — значит, аккаунт годится.
2. **F12 → Network (Сеть)** → обновите страницу (**F5**) → в фильтре введите `slide`.
3. Выберите запрос **`slide-v3`** (хост `auth-stream.wb.ru`) → **Headers → Request Headers** →
   строка **`Cookie`** → скопируйте значение целиком (ПКМ → Copy value).
   Там должна быть cookie **`wbx-refresh`** — она HttpOnly, поэтому через `document.cookie` её не достать.
4. Вставьте в панель и нажмите «Сохранить сессию».

Какие cookies важны: **`wbx-refresh`** (обязательна для автопродления), а также `x_wbaas_token`,
`_wbauid`, `wbx-validation-key` — можно вставлять весь заголовок, лишние cookies не мешают.

Другие принимаемые форматы (можно по одному на строку, вперемешку):

| Формат | Пример |
| --- | --- |
| Заголовок Cookie | `Cookie: wbx-refresh=…; _wbauid=…` или без `Cookie:` |
| JSON-экспорт расширения (Cookie-Editor и т.п.) | `[{"name":"wbx-refresh","value":"…"}, …]` |
| Bearer-токен | `Authorization: Bearer eyJ…`, `Bearer eyJ…` или голый JWT |

Bearer берётся так: Network → любой запрос к `stream.wb.ru/api-room/…` → Request Headers →
`Authorization`. **Только токен продлить нельзя**: когда WB его отзовёт, придётся вставить заново.
С `wbx-refresh` панель сама получает свежий токен (`auth-stream.wb.ru/v2/auth/slide-v3`) и сохраняет
обновлённые cookies, если WB их ротирует.

### Советы

- После копирования **не выходите из аккаунта** в этом браузере (выход отзывает сессию).
- Браузер и панель делят одну сессию. Если WB ротирует `wbx-refresh`, копия в браузере или в панели
  может устареть. Надёжнее всего — войти в отдельном профиле/окне инкогнито, скопировать cookies
  и закрыть окно, не выходя из аккаунта.
- Тег **«Сессия WB устарела»** + текст ошибки = WB отклонил сессию: войдите заново и вставьте свежие cookies.

## Как это устроено

| Что | Где |
| --- | --- |
| Пакет | `internal/lucx/wbcreator/` (`api.go` — HTTP к WB, `store.go` — хранение, `creator.go` — логика) |
| Хранение | таблица settings: `lucxWbCookies` (сессия), `lucxWbState` (deviceId, последняя ошибка, история комнат) |
| API (под авторизацией панели) | `GET /panel/api/tunnel/wb/status`, `POST /panel/api/tunnel/wb/cookies`, `POST /panel/api/tunnel/wb/cookies/clear`, `POST /panel/api/tunnel/wb/create` (`{"apply":true,"inbound_id":N}`) |
| Автосоздание при сохранении | `internal/web/service/olcrtc_wbroom.go` (`ensureOlcrtcWbRoom`, вызывается перед нормализацией olcRTC) |
| UI | `frontend/src/pages/tunnels/OlcrtcWbPanel.tsx` (в `OlcrtcCard.tsx`), кнопка в `frontend/src/pages/inbounds/form/protocols/olcrtc.tsx` |
| i18n | `pages.tunnels.olcrtc.wb.*`, `pages.inbounds.form.olcrtcWb*` (ru-RU, en-US) |

Запросы к WB: `POST https://auth-stream.wb.ru/v2/auth/slide-v3` (Cookie → bearer; «unauthorized» WB
отдаёт с HTTP 200 и `"result":12`) и `POST https://stream.wb.ru/api-room/api/v2/room`
(`ROOM_TYPE_ALL_ON_SCREEN`, `ROOM_PRIVACY_FREE`). Статус сессии WB не опрашивает (slide-v3 ротирует
cookies) — проверка происходит при создании комнаты.

## Ограничения

- Нужен живой аккаунт stream.wb.ru. WB может поменять API, добавить капчу/антибот (`x_wbaas_token`)
  или отозвать сессию — тогда генератор вернёт ошибку, и сессию надо обновить.
- Время жизни пустой комнаты WB не документировано. Пока olcrtc запущен, он сидит в комнате;
  если комната «умерла», создайте новую и раздайте клиентам новую ссылку.
- Смена комнаты = новая ссылка `olcrtc://` для клиентов.
- Для инбаундов на удалённых нодах — только через форму инбаунда (вариант Б/В).

## Происхождение кода

`ParseRoomID` и формат запроса создания комнаты адаптированы из
[kulikov0/whitelist-bypass](https://github.com/kulikov0/whitelist-bypass) `relay/wbstream/api.go`
(**MIT**). Полный текст лицензии — в [CREDITS.md](../CREDITS.md). GPL-код WDTT/PWDTT здесь не используется.

---

## English summary

The fork can create **WB Stream** (stream.wb.ru) rooms for the **olcRTC** tunnel (`provider: wbstream`).
WB guests cannot create rooms, so the panel stores a logged-in stream.wb.ru session and creates rooms with it.

- **Session:** Settings → Cores → "Open tunnel configs" (`/tunnels`) → olcRTC card → **WB Stream room generator**.
  Paste the `Cookie` request header of the `slide-v3` request (DevTools → Network, host `auth-stream.wb.ru`;
  it must contain the HttpOnly `wbx-refresh` cookie), or a JSON cookie export, or `Authorization: Bearer …`
  (token-only mode cannot be renewed). Values are never echoed back by the API.
- **Create room:** pick an olcRTC inbound with provider WB Stream → **Create room**: the room ID is written into
  that inbound's `settings.roomId` in the DB; the reconcile job restarts olcrtc within ~10 s. Or use the
  **Create room** button under the Room ID field in the olcRTC inbound form, or leave the field empty — a room is
  auto-created on save when a session is stored (failures never block the save).
- Clients need the new `olcrtc://` link after a room change.
- API: `/panel/api/tunnel/wb/{status,cookies,cookies/clear,create}` (panel auth). Package `internal/lucx/wbcreator/`.
- Room API adapted from kulikov0/whitelist-bypass (MIT); see [CREDITS.md](../CREDITS.md).
