#!/usr/bin/env bash
# lucx.sh — меню обслуживания панели LucX (форк samur005/lucx-ui-samur005).
#
# Запуск одной командой (работает через curl, соседние файлы не нужны):
#   bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/lucx.sh)
# Неинтерактивно:
#   bash <(curl -fsSL …/scripts/lucx.sh) <команда> [опции]     (список: help)
# После пункта «Самообновление / установить команду lucx» — просто: lucx
#
# Расширение: добавьте функцию cmd_<имя> и строку в MENU_ITEMS (ниже).
# Ответы читаются из /dev/tty, поэтому stdin может быть не терминалом.

# shellcheck disable=SC2015,SC2094  # A && B || C используется осознанно (B — вывод сообщения)
set -o pipefail

LUCX_VERSION_SCRIPT="1.0.0"
LUCX_GH_REPO="${LUCX_GH_REPO:-samur005/lucx-ui-samur005}"
LUCX_BRANCH="${LUCX_BRANCH:-feat/native-vk-hash-generator}"
LUCX_RAW="${LUCX_RAW:-https://raw.githubusercontent.com/${LUCX_GH_REPO}/${LUCX_BRANCH}}"
LUCX_UPSTREAM="${LUCX_UPSTREAM:-https://github.com/AlexeyLCP/lucx-ui.git}"
LUCX_UPSTREAM_BRANCH="${LUCX_UPSTREAM_BRANCH:-main}"
LUCX_SYNC_DIR="${LUCX_SYNC_DIR:-/usr/local/src/lucx-fork-sync}"
LUCX_SRC="${LUCX_SRC:-/usr/local/src/lucx-ui-samur005}"
XUI_DIR="${XUI_DIR:-/usr/local/x-ui}"
XUI_BIN="${XUI_DIR}/x-ui"
XUI_ETC="${XUI_ETC:-/etc/x-ui}"
XUI_DB="${XUI_DB_FOLDER:-$XUI_ETC}/x-ui.db"
TUNNEL_DIR="${XUI_DIR}/bin/tunnel"
QWDTT_PW="${TUNNEL_DIR}/qwdtt-data/passwords.json"
BACKUP_ROOT="${LUCX_BACKUP_ROOT:-/root/lucx-backups}"
TG_ENV="${XUI_ETC}/lucx-backup.env"
HOSTS_CONF="${XUI_ETC}/lucx-hosts.conf"
SYSCTL_FILE="/etc/sysctl.d/99-lucx.conf"
LUCX_LANG="${LUCX_LANG:-ru}"
LUCX_DRY_RUN="${LUCX_DRY_RUN:-0}"   # 1 = печатать меняющие команды, не выполнять
ASSUME_YES=0
NO_PUSH=0
LAST_BACKUP=""

if [[ -t 1 ]]; then
    C_R=$'\e[31m'; C_G=$'\e[32m'; C_Y=$'\e[33m'; C_B=$'\e[34m'; C_C=$'\e[36m'; C_W=$'\e[1m'; C_D=$'\e[2m'; C_0=$'\e[0m'
else
    C_R=""; C_G=""; C_Y=""; C_B=""; C_C=""; C_W=""; C_D=""; C_0=""
fi

info() { printf '%s[i]%s %s\n' "$C_B" "$C_0" "$*"; }
ok()   { printf '%s[✓]%s %s\n' "$C_G" "$C_0" "$*"; }
warn() { printf '%s[!]%s %s\n' "$C_Y" "$C_0" "$*" >&2; }
err()  { printf '%s[✗]%s %s\n' "$C_R" "$C_0" "$*" >&2; }
die()  { err "$*"; exit 1; }
hdr()  { printf '\n%s== %s ==%s\n' "$C_W$C_C" "$*" "$C_0"; }
have() { command -v "$1" >/dev/null 2>&1; }
ts()   { date +%Y%m%d-%H%M%S; }

# Выполнить меняющую команду (в режиме LUCX_DRY_RUN=1 только печать).
run() {
    if [[ "$LUCX_DRY_RUN" == 1 ]]; then
        printf '%s[dry-run]%s %s\n' "$C_D" "$C_0" "$*"
        return 0
    fi
    "$@"
}

TTY=/dev/tty
has_tty() { [[ -r $TTY && -w $TTY ]] && { : <"$TTY"; } 2>/dev/null; }

# ask "вопрос" [по-умолчанию] -> REPLY
ask() {
    local q="$1" def="${2-}"
    if ! has_tty; then REPLY="$def"; return 0; fi
    if [[ -n "$def" ]]; then printf '%s [%s]: ' "$q" "$def" >"$TTY"; else printf '%s: ' "$q" >"$TTY"; fi
    IFS= read -r REPLY <"$TTY" || REPLY=""
    [[ -z "$REPLY" ]] && REPLY="$def"
    return 0
}
ask_secret() {
    if ! has_tty; then REPLY=""; return 1; fi
    printf '%s: ' "$1" >"$TTY"
    IFS= read -rs REPLY <"$TTY" || REPLY=""
    printf '\n' >"$TTY"
}
# confirm "вопрос" -> 0 если да. --yes / LUCX_YES=1 отвечает «да».
confirm() {
    [[ "$ASSUME_YES" == 1 || "${LUCX_YES:-0}" == 1 ]] && return 0
    has_tty || { warn "Нет терминала для подтверждения: «$1» — отменено (используйте --yes)."; return 1; }
    ask "$1 (y/N)" "n"
    [[ "$REPLY" =~ ^([yYдД]|yes|да)$ ]]
}
# Жёсткое подтверждение вводом слова yes.
confirm_yes() {
    [[ "$ASSUME_YES" == 1 ]] && return 0
    has_tty || { warn "Нужно подтверждение вводом yes, терминала нет — отменено."; return 1; }
    ask "$1 — введите yes для подтверждения" ""
    [[ "$REPLY" == "yes" ]]
}
pause() { has_tty && { printf '%sEnter — назад в меню%s' "$C_D" "$C_0" >"$TTY"; read -r _ <"$TTY" || true; }; return 0; }

need_root() { [[ $EUID -eq 0 ]] || die "Нужен root (sudo -i)."; }
xui_installed() { [[ -x "$XUI_BIN" ]]; }
need_xui() { xui_installed || die "Панель x-ui не найдена ($XUI_BIN). Сначала пункт «Установить»."; }

ram_mb()  { awk '/MemTotal/{print int($2/1024)}' /proc/meminfo; }
swap_mb() { awk '/SwapTotal/{print int($2/1024)}' /proc/meminfo; }
avail_mb(){ awk '/MemAvailable/{print int($2/1024)}' /proc/meminfo; }
disk_free_mb() { df -Pm "${1:-/}" 2>/dev/null | awk 'NR==2{print $4}'; }

xui_env_file() {
    local f
    for f in /etc/default/x-ui /etc/conf.d/x-ui /etc/sysconfig/x-ui; do [[ -f $f ]] && { echo "$f"; return; }; done
    echo /etc/default/x-ui
}
# Тип БД: postgres если XUI_DB_TYPE=postgres в env-файле или окружении юнита.
db_kind() {
    local f t=""
    f=$(xui_env_file)
    [[ -r $f ]] && t=$(sed -n 's/^XUI_DB_TYPE=//p' "$f" | tr -d '"'"'" | tail -1)
    [[ -z $t ]] && have systemctl && t=$(systemctl show x-ui -p Environment 2>/dev/null | grep -o 'XUI_DB_TYPE=[^ ]*' | cut -d= -f2)
    case "${t,,}" in postgres|postgresql|pg) echo postgres ;; *) echo sqlite ;; esac
}
db_dsn() { local f; f=$(xui_env_file); [[ -r $f ]] && sed -n 's/^XUI_DB_DSN=//p' "$f" | tr -d '"'"'" | tail -1; }

# Чтение из sqlite (только чтение).
sql() {
    [[ -f "$XUI_DB" ]] || return 1
    have sqlite3 || return 1
    sqlite3 -readonly -separator '|' "$XUI_DB" "$1" 2>/dev/null
}
setting() { sql "SELECT value FROM settings WHERE key='$1' LIMIT 1;"; }

panel_port()  { local p; p=$(setting webPort); echo "${p:-2053}"; }
panel_path()  { local p; p=$(setting webBasePath); echo "${p:-/}"; }
sub_port()    { local p; p=$(setting subPort); echo "${p:-2096}"; }
sub_path()    { local p; p=$(setting subPath); echo "${p:-/sub/}"; }
panel_scheme(){ [[ -n "$(setting webCertFile)" ]] && echo https || echo http; }
sub_scheme()  { [[ -n "$(setting subCertFile)" ]] && echo https || echo http; }
ssh_port()    { local p; p=$(sshd -T 2>/dev/null | awk '/^port /{print $2; exit}'); echo "${p:-22}"; }
public_ip()   { curl -4 -fsS --max-time 6 https://ifconfig.me 2>/dev/null || curl -4 -fsS --max-time 6 https://api.ipify.org 2>/dev/null || ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}'; }
xui_version() { "${1:-$XUI_BIN}" -v 2>/dev/null | head -1; }
xray_bin()    { find "$XUI_DIR/bin" -maxdepth 1 -name 'xray-linux-*' -type f 2>/dev/null | head -1; }

# Скачать файл ветки (scripts/… и т.п.) во временный файл.
fetch_raw() {
    local rel="$1" out="$2"
    curl -fsSL --retry 3 --max-time 60 "${LUCX_RAW}/${rel}" -o "$out" || return 1
    [[ -s "$out" ]]
}

# ---------- Telegram ----------
tg_load() { TG_TOKEN=""; TG_CHAT=""; [[ -r $TG_ENV ]] && { TG_TOKEN=$(sed -n 's/^TG_TOKEN=//p' "$TG_ENV"); TG_CHAT=$(sed -n 's/^TG_CHAT=//p' "$TG_ENV"); }; [[ -n $TG_TOKEN && -n $TG_CHAT ]]; }
tg_setup() {
    tg_load && { info "Telegram уже настроен ($TG_ENV)."; confirm "Перенастроить?" || return 0; }
    ask_secret "Токен бота Telegram (от @BotFather)" || return 1
    local tok="$REPLY"
    ask "chat_id (ваш id или id группы)" ""
    local chat="$REPLY"
    [[ -n $tok && -n $chat ]] || { warn "Пусто — Telegram не настроен."; return 1; }
    run install -m 600 /dev/null "$TG_ENV"
    [[ "$LUCX_DRY_RUN" == 1 ]] || printf 'TG_TOKEN=%s\nTG_CHAT=%s\n' "$tok" "$chat" >"$TG_ENV"
    ok "Сохранено в $TG_ENV (chmod 600)."
}
tg_send() { tg_load || return 1; curl -fsS --max-time 20 "https://api.telegram.org/bot${TG_TOKEN}/sendMessage" -d chat_id="$TG_CHAT" --data-urlencode text="$1" >/dev/null; }
tg_doc()  { tg_load || return 1; curl -fsS --max-time 300 "https://api.telegram.org/bot${TG_TOKEN}/sendDocument" -F chat_id="$TG_CHAT" -F document=@"$1" -F caption="${2:-}" >/dev/null; }

# ---------- Проверки ----------
check_resources() {
    local ram swap disk
    ram=$(ram_mb); swap=$(swap_mb); disk=$(disk_free_mb /)
    info "RAM: ${ram} МБ (доступно $(avail_mb)), swap: ${swap} МБ, свободно на /: ${disk} МБ"
    local bad=0
    (( disk < 2048 )) && { err "Мало места (<2 ГБ) для сборки."; bad=1; }
    (( ram + swap < 2048 )) && { warn "RAM+swap < 2 ГБ — сборка может упасть по OOM; создайте swap (пункт «Swap»)."; }
    return $bad
}
goflags_by_ram() { if (( $(ram_mb) <= 2048 )); then echo "-p=1"; else echo "-p=2"; fi; }

verify_panel() {
    local i
    for i in 1 2 3 4 5 6 7 8 9 10; do systemctl is-active --quiet x-ui && break; sleep 3; done
    systemctl is-active --quiet x-ui || { err "x-ui не active"; return 1; }
    local v; v=$(xui_version) || true
    [[ -n $v ]] || { err "x-ui -v не отвечает"; return 1; }
    sleep 5
    if journalctl -u x-ui --since "-2min" --no-pager 2>/dev/null | grep -Eiq 'panic|fatal'; then
        err "В журнале x-ui есть panic/fatal:"; journalctl -u x-ui --since "-2min" --no-pager | grep -Ei 'panic|fatal' | tail -5
        return 1
    fi
    ok "Панель работает, версия: $v"
}

# ================= ПУНКТЫ =================

cmd_prepare() {
    hdr "С чего начать: подготовка сервера"
    local -a RES=()
    mark() { RES+=("$1|$2"); if [[ $1 == ok ]]; then ok "$2"; else warn "$2"; fi; }
    # IP и домен
    local ip; ip=$(public_ip)
    if [[ -n $ip ]]; then mark ok "Внешний IPv4: $ip"; else mark bad "Не удалось определить внешний IP"; fi
    ask "Домен для панели/подписки (Enter — пропустить)" "${LUCX_DOMAIN:-}"
    local dom="$REPLY"
    if [[ -n $dom ]]; then
        local a aaaa
        if have dig; then a=$(dig +short A "$dom" | tr '\n' ' '); aaaa=$(dig +short AAAA "$dom" | tr '\n' ' ')
        else a=$(getent ahostsv4 "$dom" | awk '{print $1}' | sort -u | tr '\n' ' '); aaaa=$(getent ahostsv6 "$dom" 2>/dev/null | awk '{print $1}' | sort -u | tr '\n' ' '); fi
        info "A: ${a:-нет}  AAAA: ${aaaa:-нет}"
        if [[ " $a " == *" $ip "* ]]; then mark ok "Домен $dom указывает на этот сервер"
        else
            mark bad "Домен $dom НЕ указывает на $ip"
            echo "  → Создайте у регистратора/в Cloudflare A-запись: $dom → $ip (TTL 300). Подождите 5–30 мин."
            if [[ $a =~ (^| )(104\.(1[6-9]|2[0-9]|3[01])|172\.6[4-7]|188\.114|162\.15[89]|141\.101|108\.162|190\.93|197\.234|198\.41)\. ]]; then
                echo "  → IP домена — Cloudflare (оранжевое облако): для панели, Reality и qWDTT включите режим DNS only (серое облако)."
            fi
        fi
        [[ -n $aaaa ]] && warn "Есть AAAA-запись: убедитесь, что IPv6 сервера совпадает, иначе SSL/клиенты могут ходить мимо."
    else
        echo "  → Без домена SSL выпускается на IP (короткий срок) — домен рекомендуется."
    fi
    # ОС
    local os arch
    # shellcheck source=/dev/null
    os=$(. /etc/os-release 2>/dev/null && echo "$ID $VERSION_ID"); arch=$(uname -m)
    case "$arch" in x86_64|aarch64|arm64) mark ok "ОС: $os, архитектура $arch" ;; *) mark bad "Архитектура $arch может не поддерживаться" ;; esac
    [[ $EUID -eq 0 ]] && mark ok "Запущено от root" || mark bad "Не root"
    if have timedatectl && timedatectl show -p NTPSynchronized --value 2>/dev/null | grep -q yes; then mark ok "Время синхронизировано (NTP)"
    else mark bad "Время не синхронизировано — важно для TLS/Reality: timedatectl set-ntp true"; fi
    # Порты
    local pp; pp=$(panel_port)
    local p
    for p in 80 443 "$pp"; do
        local who; who=$(ss -Hltnp "sport = :$p" 2>/dev/null | grep -o 'users:(("[^"]*' | head -1 | cut -d'"' -f2)
        if [[ -z $who ]]; then mark ok "Порт $p свободен"
        elif [[ $p == "$pp" && $who == x-ui ]]; then mark ok "Порт панели $p занят x-ui (норма)"
        else mark bad "Порт $p занят: $who"; [[ $p == 80 ]] && echo "  → Порт 80 нужен для выпуска SSL (acme standalone)."; fi
    done
    # Ресурсы
    local ram swap; ram=$(ram_mb); swap=$(swap_mb)
    if (( ram < 2048 && swap < 1024 )); then mark bad "RAM ${ram} МБ, swap ${swap} МБ — рекомендуется swap 2 ГБ (пункт «Swap»)"
    else mark ok "RAM ${ram} МБ, swap ${swap} МБ"; fi
    local df; df=$(disk_free_mb /)
    (( df > 3072 )) && mark ok "Свободно на диске: ${df} МБ" || mark bad "Мало места: ${df} МБ (нужно ≥3 ГБ)"
    # GitHub
    if curl -fsS --max-time 10 -o /dev/null "${LUCX_RAW}/scripts/install-fork.sh"; then mark ok "GitHub/raw доступен"; else mark bad "Нет доступа к raw.githubusercontent.com"; fi
    # UFW
    if have ufw && ufw status 2>/dev/null | grep -q 'Status: active'; then mark ok "UFW активен"; else mark bad "UFW не активен (см. пункт «Firewall»)"; fi

    hdr "Сводка"
    local r
    for r in "${RES[@]}"; do if [[ ${r%%|*} == ok ]]; then printf ' %s✓%s %s\n' "$C_G" "$C_0" "${r#*|}"; else printf ' %s✗%s %s\n' "$C_R" "$C_0" "${r#*|}"; fi; done
    cat <<TXT

${C_W}Рекомендуемый порядок:${C_0}
 1) DNS: A-запись домена → IP сервера (DNS only)   2) Подготовка: swap, время, пакеты, UFW
 3) Установить панель                               4) SSL на домен
 5) Сменить логин/пароль/путь панели                6) BBR и тюнинг сети
 7) Бэкап по расписанию                             8) Мониторинг в Telegram
${C_W}Узел (нода):${C_0} установите форк на узле так же, затем в панели МАСТЕРА:
 «Узлы» → «Добавить» → адрес/порт/путь панели узла и API-токен узла
 (на узле: x-ui setting -getApiToken -tokenScope node-sync).
TXT
    if [[ $EUID -eq 0 ]] && have apt-get && confirm "Обновить пакеты системы (apt update && apt upgrade)?"; then
        run apt-get update && DEBIAN_FRONTEND=noninteractive run apt-get -y upgrade
    fi
    if have ufw && confirm "Открыть в UFW порты SSH($(ssh_port)), 80, 443, панели($pp)?"; then
        for p in "$(ssh_port)" 80 443 "$pp"; do run ufw allow "$p/tcp"; done
    fi
    if ! xui_installed && confirm "Перейти к установке панели?"; then cmd_install; fi
}

cmd_install() {
    need_root
    hdr "Установка форка LucX"
    if xui_installed; then
        warn "x-ui уже установлен: $(xui_version)"
        confirm "Сделать полный бэкап перед переустановкой?" && cmd_backup_full
        confirm "Продолжить установку поверх?" || return 0
    fi
    check_resources || confirm "Ресурсов мало. Всё равно продолжить?" || return 1
    local tmp; tmp=$(mktemp /tmp/install-fork.XXXXXX.sh)
    fetch_raw scripts/install-fork.sh "$tmp" || die "Не удалось скачать install-fork.sh"
    info "Запуск install-fork.sh (ветка $LUCX_BRANCH)…"
    GOFLAGS="$(goflags_by_ram)" LUCX_BRANCH="$LUCX_BRANCH" run nice -n 10 bash "$tmp"
    local rc=$?
    rm -f "$tmp"
    (( rc == 0 )) && ok "Установка завершена. Дальше: SSL, смена доступа, BBR, бэкапы." || err "install-fork.sh завершился с кодом $rc"
    return $rc
}

backup_db_to() {   # $1 = каталог назначения
    local dst="$1"
    if [[ $(db_kind) == postgres ]]; then
        have pg_dump || { warn "PostgreSQL, но pg_dump не найден — дамп БД пропущен"; return 1; }
        pg_dump --dbname="$(db_dsn)" -Fc -f "$dst/x-ui.pgdump" && ok "pg_dump → $dst/x-ui.pgdump"
    elif [[ -f $XUI_DB ]]; then
        if have sqlite3; then sqlite3 "$XUI_DB" ".backup '$dst/x-ui.db'" || cp -a "$XUI_DB" "$dst/x-ui.db"
        else cp -a "$XUI_DB" "$dst/x-ui.db"; fi
        ok "БД → $dst/x-ui.db"
    fi
}

cmd_update() {
    need_root; need_xui
    hdr "Обновление панели"
    check_resources || confirm "Ресурсов мало. Продолжить?" || return 1
    local old_v bak d
    old_v=$(xui_version)
    info "Текущая версия: $old_v"
    d="$BACKUP_ROOT/pre-update-$(ts)"; run mkdir -p "$d"
    [[ $LUCX_DRY_RUN == 1 ]] || backup_db_to "$d"
    bak="$XUI_DIR/x-ui.bak-$(ts)"
    run cp -a "$XUI_BIN" "$bak" && ok "Бинарник → $bak"
    local tmp; tmp=$(mktemp /tmp/install-fork.XXXXXX.sh)
    fetch_raw scripts/install-fork.sh "$tmp" || die "Не удалось скачать install-fork.sh"
    local gf; gf=$(goflags_by_ram)
    info "Сборка: GOFLAGS=$gf, nice 10, LUCX_ALLOW_REBOOT=0 LUCX_UFW_OPEN=0"
    LUCX_ALLOW_REBOOT=0 LUCX_UFW_OPEN=0 GOFLAGS="$gf" LUCX_BRANCH="$LUCX_BRANCH" run nice -n 10 bash "$tmp"
    local rc=$?; rm -f "$tmp"
    [[ $LUCX_DRY_RUN == 1 ]] && return 0
    if (( rc == 0 )) && verify_panel; then
        ok "Обновлено: $old_v → $(xui_version)"; return 0
    fi
    err "Обновление не прошло проверку — откат бинарника из $bak"
    systemctl stop x-ui; cp -a "$bak" "$XUI_BIN"; systemctl start x-ui
    verify_panel && warn "Откат выполнен, работает прежняя версия $old_v. БД-бэкап: $d" || err "После отката панель не поднялась! Бэкап БД: $d"
    return 1
}

cmd_uninstall() {
    need_root
    hdr "Удаление панели"
    xui_installed || { warn "x-ui не установлен."; return 0; }
    confirm_yes "Удалить панель x-ui и Xray" || { info "Отменено."; return 0; }
    confirm "Сделать полный бэкап перед удалением? (рекомендуется)" && cmd_backup_full
    if [[ -f $XUI_DIR/x-ui.sh ]] && grep -q '"uninstall")' "$XUI_DIR/x-ui.sh"; then
        info "Стоковое удаление: x-ui uninstall"
        if has_tty; then run x-ui uninstall <"$TTY"; else run x-ui uninstall; fi
    else
        warn "x-ui.sh без uninstall — ручное удаление."
        confirm_yes "Удалить $XUI_DIR, $XUI_ETC, юнит x-ui" || return 0
        run systemctl disable --now x-ui
        run rm -rf /etc/systemd/system/x-ui.service /etc/systemd/system/x-ui.service.d "$XUI_DIR" "$XUI_ETC" /usr/bin/x-ui
        run systemctl daemon-reload
    fi
}

cmd_rollback() {
    need_root; need_xui
    hdr "Откат на предыдущую версию"
    local -a f=()
    mapfile -t f < <(find "$XUI_DIR" -maxdepth 1 -type f -name 'x-ui.bak-*' -printf '%T@ %p\n' 2>/dev/null | sort -rn | cut -d' ' -f2-)
    (( ${#f[@]} )) || { warn "Нет файлов $XUI_DIR/x-ui.bak-*"; return 0; }
    info "Текущая: $(xui_version)"
    local i
    for i in "${!f[@]}"; do printf ' %2d) %-55s %s\n' "$((i+1))" "${f[$i]##*/}" "$(timeout 5 "${f[$i]}" -v 2>/dev/null | head -1)"; done
    ask "Номер версии для отката (0 — отмена)" "0"
    [[ $REPLY =~ ^[0-9]+$ ]] && (( REPLY >= 1 && REPLY <= ${#f[@]} )) || return 0
    local sel="${f[$((REPLY-1))]}" keep
    confirm "Заменить текущий бинарник на ${sel##*/} и перезапустить?" || return 0
    keep="$XUI_DIR/x-ui.bak-prerollback-$(ts)"
    run cp -a "$XUI_BIN" "$keep"
    run systemctl stop x-ui; run cp -a "$sel" "$XUI_BIN"; run systemctl start x-ui
    [[ $LUCX_DRY_RUN == 1 ]] && return 0
    if verify_panel; then ok "Откат выполнен. Прежний бинарник: $keep"
    else err "Не поднялась — возвращаю как было"; systemctl stop x-ui; cp -a "$keep" "$XUI_BIN"; systemctl start x-ui; verify_panel; fi
}

cmd_preflight() {
    hdr "Проверка перед обновлением"
    check_resources || true
    if have go; then info "Go: $(go version | awk '{print $3}')"; else info "Go: не установлен (install-fork.sh поставит)"; fi
    if have node; then info "Node: $(node -v)"; else info "Node: не установлен"; fi
    if curl -fsS --max-time 10 -o /dev/null https://github.com; then ok "github.com доступен"; else err "github.com недоступен"; fi
    local inst="-" br up
    xui_installed && inst=$(xui_version)
    br=$(curl -fsSL --max-time 15 "${LUCX_RAW}/internal/config/version" 2>/dev/null | tr -d '[:space:]')
    up=$(curl -fsSL --max-time 15 "https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/${LUCX_UPSTREAM_BRANCH}/internal/config/version" 2>/dev/null | tr -d '[:space:]')
    info "Установлено: ${inst:--}"
    info "В ветке форка ($LUCX_BRANCH): ${br:-?}"
    info "В upstream AlexeyLCP/lucx-ui: ${up:-?}"
    local sha
    sha=$(curl -fsSL --max-time 15 "https://api.github.com/repos/${LUCX_GH_REPO}/commits/${LUCX_BRANCH}" 2>/dev/null | sed -n 's/^  "sha": "\([0-9a-f]\{7\}\).*/\1/p' | head -1)
    [[ -n $sha ]] && info "Последний коммит ветки: $sha"
    if [[ -n $br && $inst == *"$br"* ]]; then ok "Установлена актуальная версия ветки"; elif [[ -n $br ]]; then warn "Доступно обновление: $br"; fi
    [[ -n $up && -n $br && $br != *"${up%%-*}"* ]] && warn "Upstream ушёл вперёд ($up) — пункт «Обновить код на GitHub»."
    return 0
}

hosts_list() { [[ -f $HOSTS_CONF ]] && grep -Ev '^\s*(#|$)' "$HOSTS_CONF"; }
cmd_update_all() {
    need_root
    hdr "Обновить все серверы"
    if [[ ! -f $HOSTS_CONF ]]; then
        info "Создаю шаблон $HOSTS_CONF"
        run install -m 600 /dev/null "$HOSTS_CONF"
        [[ $LUCX_DRY_RUN == 1 ]] || cat >"$HOSTS_CONF" <<'H'
# роль  хост          порт  пользователь  auth(key|pass)
# node  1.2.3.4       22    root          key
# master local
H
        info "Заполните файл и запустите снова. Пароли не храните — auth=pass спросит при запуске."
        return 0
    fi
    local -a nodes=() masters=()
    local role h p u a
    while read -r role h p u a; do
        case "$role" in node) nodes+=("$h|${p:-22}|${u:-root}|${a:-key}") ;; master) masters+=("$h|${p:-22}|${u:-root}|${a:-key}") ;; esac
    done < <(hosts_list)
    info "Узлов: ${#nodes[@]}, мастеров: ${#masters[@]}. Порядок: сначала узлы, потом мастер."
    confirm "Запустить обновление по очереди (стоп на первой ошибке)?" || return 0
    local e
    for e in "${nodes[@]}" "${masters[@]}"; do
        IFS='|' read -r h p u a <<<"$e"
        hdr "Сервер $h"
        if [[ $h == local ]]; then cmd_update || { err "Сбой на $h — стоп"; return 1; }; continue; fi
        local -a SSH=(ssh -p "$p" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15)
        if [[ $a == pass ]]; then
            have sshpass || { err "Нужен sshpass (apt install sshpass) или ключи"; return 1; }
            ask_secret "Пароль $u@$h" || return 1
            export SSHPASS="$REPLY"; SSH=(sshpass -e "${SSH[@]}")
        fi
        run "${SSH[@]}" "$u@$h" "bash -c 'bash <(curl -fsSL ${LUCX_RAW}/scripts/lucx.sh) update --yes'" </dev/null || { unset SSHPASS; err "Сбой на $h — стоп"; return 1; }
        unset SSHPASS
        ok "$h обновлён"
    done
}

# ---------- Бэкапы ----------
sidecar_units() { systemctl list-unit-files --no-legend 2>/dev/null | awk '{print $1}' | grep -E '^(x-ui|lucx-|tproxy-server|mtprotoproxy|awg-quick@)' ; }

make_backup() {   # $1 = full|settings
    need_root
    local kind="$1" dir stamp arch
    stamp=$(ts); dir="$BACKUP_ROOT/$stamp"
    mkdir -p "$dir/stage/stagedb" || return 1
    chmod 700 "$BACKUP_ROOT"
    backup_db_to "$dir/stage/stagedb"
    local -a paths=() excl=(--exclude='etc/x-ui/x-ui.db' --exclude='etc/x-ui/x-ui.db-wal' --exclude='etc/x-ui/x-ui.db-shm' --exclude='etc/x-ui/x-ui.db.bak*'
        --exclude='usr/local/x-ui/x-ui.bak-*' --exclude='usr/local/x-ui/x-ui.stock*' --exclude='usr/local/x-ui/x-ui.nopages' --exclude='*.sock')
    local p
    for p in "$XUI_ETC" "$(xui_env_file)"; do [[ -e $p ]] && paths+=("${p#/}"); done
    if [[ $kind == settings ]]; then
        for p in "$TUNNEL_DIR"/qwdtt-data "$TUNNEL_DIR"/*.json "$TUNNEL_DIR"/*.caddyfile "$XUI_DIR"/bin/config.json; do [[ -e $p ]] && paths+=("${p#/}"); done
        excl+=(--exclude='*-data/caddy' --exclude='*.log')
    else
        paths+=("${XUI_DIR#/}")
        for p in /etc/systemd/system/x-ui.service /etc/systemd/system/x-ui.service.d /etc/systemd/system/lucx-*.service \
                 /etc/systemd/system/tproxy-server.service /etc/systemd/system/mtprotoproxy.service /etc/amnezia /etc/wireguard \
                 /root/.acme.sh /root/cert /etc/cron.d/lucx-backup /etc/cron.d/lucx-monitor "$SYSCTL_FILE" /usr/bin/x-ui; do
            [[ -e $p ]] && paths+=("${p#/}")
        done
    fi
    arch="$dir/lucx-${kind}-$(hostname -s)-$stamp.tar.gz"
    tar -czf "$arch" "${excl[@]}" -C / "${paths[@]}" -C "$dir/stage" --transform 's,^stagedb,etc/x-ui,' stagedb 2>"$dir/tar.err"
    local rc=$?
    rm -rf "$dir/stage"
    if (( rc > 1 )); then err "tar: $(tail -3 "$dir/tar.err")"; return 1; fi
    [[ -s $dir/tar.err ]] || rm -f "$dir/tar.err"
    chmod 600 "$arch"
    sha256sum "$arch" | awk '{print $1}' >"$arch.sha256"
    LAST_BACKUP="$arch"
    ok "Архив: $arch"
    info "Размер: $(du -h "$arch" | cut -f1), sha256: $(cat "$arch.sha256")"
}
cmd_backup_full()     { hdr "Полный бэкап"; make_backup full; post_backup; }
cmd_backup_settings() { hdr "Бэкап настроек"; make_backup settings; post_backup; }
post_backup() {
    [[ -n $LAST_BACKUP ]] || return 1
    [[ -n ${OPT_ROTATE:-} ]] && rotate_backups "$OPT_ROTATE"
    if [[ ${OPT_TG:-0} == 1 ]]; then
        local sz; sz=$(stat -c %s "$LAST_BACKUP")
        if (( sz < 49000000 )); then tg_doc "$LAST_BACKUP" "LucX бэкап $(hostname) $(date '+%F %T')" && ok "Отправлено в Telegram" || warn "Telegram: не отправлено"
        else tg_send "LucX бэкап $(hostname): $LAST_BACKUP ($(du -h "$LAST_BACKUP" | cut -f1)) — больше 50 МБ, лежит на сервере"; fi
    fi
    return 0
}
rotate_backups() {
    local keep="$1" d
    mapfile -t d < <(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -name '20*' | sort -r)
    local i
    for (( i=keep; i<${#d[@]}; i++ )); do run rm -rf "${d[$i]}" && info "Ротация: удалён ${d[$i]}"; done
}

list_archives() { find "$BACKUP_ROOT" -name 'lucx-*.tar.gz' -type f -printf '%T@ %p\n' 2>/dev/null | sort -rn | cut -d' ' -f2-; }
cmd_restore() {
    need_root
    hdr "Восстановление из бэкапа"
    local -a a=(); mapfile -t a < <(list_archives)
    local sel="${OPT_FILE:-}"
    if [[ -z $sel ]]; then
        (( ${#a[@]} )) || { warn "Нет архивов в $BACKUP_ROOT"; return 0; }
        local i; for i in "${!a[@]}"; do printf ' %2d) %s  %s\n' "$((i+1))" "${a[$i]#"$BACKUP_ROOT"/}" "$(du -h "${a[$i]}" | cut -f1)"; done
        ask "Номер архива (0 — отмена)" "0"
        [[ $REPLY =~ ^[0-9]+$ ]] && (( REPLY >= 1 && REPLY <= ${#a[@]} )) || return 0
        sel="${a[$((REPLY-1))]}"
    fi
    [[ -f $sel ]] || die "Нет файла $sel"
    if [[ -f $sel.sha256 ]] && [[ $(sha256sum "$sel" | awk '{print $1}') != "$(cat "$sel.sha256")" ]]; then die "sha256 не совпадает — архив повреждён"; fi
    tar -tzf "$sel" >/dev/null || die "Архив не читается"
    info "Содержимое (верхний уровень): $(tar -tzf "$sel" | cut -d/ -f1-3 | sort -u | head -15 | tr '\n' ' ')"
    confirm_yes "Восстановить $(basename "$sel") поверх текущих файлов" || return 0
    info "Сначала бэкап текущего состояния…"
    [[ $LUCX_DRY_RUN == 1 ]] || make_backup settings || die "Не удалось сделать предварительный бэкап"
    run systemctl stop x-ui
    run tar -xzf "$sel" -C / --exclude='etc/x-ui/x-ui.pgdump'
    run rm -f "$XUI_DB-wal" "$XUI_DB-shm"
    if tar -tzf "$sel" | grep -q 'x-ui.pgdump$' && [[ $(db_kind) == postgres ]]; then
        local tmpd; tmpd=$(mktemp -d)
        tar -xzf "$sel" -C "$tmpd" etc/x-ui/x-ui.pgdump && confirm "Восстановить PostgreSQL дамп (pg_restore --clean)?" && run pg_restore --clean --if-exists --dbname="$(db_dsn)" "$tmpd/etc/x-ui/x-ui.pgdump"
        rm -rf "$tmpd"
    fi
    run systemctl daemon-reload
    run systemctl start x-ui
    [[ $LUCX_DRY_RUN == 1 ]] || verify_panel || err "После восстановления проверка не прошла. Предыдущее состояние: $LAST_BACKUP"
}

cmd_backup_cron() {
    need_root
    hdr "Бэкап по расписанию"
    local cf=/etc/cron.d/lucx-backup
    [[ -f $cf ]] && { info "Сейчас: $(grep -v '^#' "$cf")"; }
    echo " 1) ежедневно  2) еженедельно  3) выключить  0) назад"
    ask "Выбор" "0"
    local when
    case "$REPLY" in 1) when="30 3 * * *" ;; 2) when="30 3 * * 0" ;; 3) run rm -f "$cf"; ok "Расписание выключено"; return 0 ;; *) return 0 ;; esac
    ask "Тип: settings (маленький) или full" "settings"; local kind="$REPLY"; [[ $kind == full ]] || kind=settings
    ask "Сколько копий хранить" "7"; local keep="$REPLY"; [[ $keep =~ ^[0-9]+$ ]] || keep=7
    local tg=""
    if confirm "Отправлять архив в Telegram?"; then tg_setup && tg=" --telegram"; fi
    install_cmd quiet || return 1
    run install -m 644 /dev/null "$cf"
    [[ $LUCX_DRY_RUN == 1 ]] || printf '# LucX: бэкап по расписанию\nSHELL=/bin/bash\n%s root /usr/local/bin/lucx backup-%s --rotate %s%s >>/var/log/lucx-backup.log 2>&1\n' "$when" "$kind" "$keep" "$tg" >"$cf"
    ok "Расписание: $when, backup-$kind, хранить $keep${tg:+, Telegram}"
}

cmd_migrate() {
    need_root; need_xui
    hdr "Миграция на новый сервер"
    ask "Новый хост (IP/домен)" ""; local h="$REPLY"; [[ -n $h ]] || return 0
    ask "SSH-порт" "22"; local p="$REPLY"
    ask "Пользователь" "root"; local u="$REPLY"
    local -a SSH=(ssh -p "$p" -o StrictHostKeyChecking=accept-new) SCP=(scp -P "$p" -o StrictHostKeyChecking=accept-new)
    if confirm "Вход по паролю (иначе ssh-ключ)?"; then
        have sshpass || die "Нужен sshpass: apt install sshpass"
        ask_secret "Пароль $u@$h" || return 1; export SSHPASS="$REPLY"
        SSH=(sshpass -e "${SSH[@]}"); SCP=(sshpass -e "${SCP[@]}")
    fi
    confirm_yes "Полный бэкап → $h, установка форка там, восстановление" || { unset SSHPASS; return 0; }
    make_backup full || { unset SSHPASS; return 1; }
    local bf="$LAST_BACKUP" rf
    rf="/root/$(basename "$bf")"
    run "${SSH[@]}" "$u@$h" "mkdir -p /root/lucx-backups" &&
    run "${SCP[@]}" "$bf" "$bf.sha256" "$u@$h:/root/" &&
    run "${SSH[@]}" "$u@$h" "bash -c 'bash <(curl -fsSL ${LUCX_RAW}/scripts/lucx.sh) install --yes && bash <(curl -fsSL ${LUCX_RAW}/scripts/lucx.sh) restore --file $rf --yes && bash <(curl -fsSL ${LUCX_RAW}/scripts/lucx.sh) status'" </dev/null
    local rc=$?; unset SSHPASS
    (( rc == 0 )) && ok "Миграция завершена. Перенаправьте DNS домена на $h." || err "Миграция прервана (код $rc). Текущий сервер не изменён."
}

# ---------- Обслуживание ----------
cmd_status() {
    hdr "Статус"
    if ! xui_installed; then warn "x-ui не установлен"; return 0; fi
    info "Версия: $(xui_version)"
    info "Сервис x-ui: $(systemctl is-active x-ui 2>/dev/null), с $(systemctl show x-ui -p ActiveEnterTimestamp --value 2>/dev/null)"
    info "Uptime системы: $(uptime -p 2>/dev/null)"
    info "БД: $(db_kind) $( [[ $(db_kind) == sqlite ]] && du -h "$XUI_DB" 2>/dev/null | cut -f1)"
    info "Панель: $(panel_scheme)://$(public_ip):$(panel_port)$(panel_path)"
    info "Подписка: $(sub_scheme)://…:$(sub_port)$(sub_path)"
    local cert; cert=$(setting webCertFile)
    if [[ -n $cert && -r $cert ]]; then info "SSL панели: $(openssl x509 -enddate -noout -in "$cert" 2>/dev/null | cut -d= -f2) ($cert)"; else info "SSL панели: не задан"; fi
    local x; x=$(xray_bin); [[ -n $x ]] && info "Xray: $("$x" version 2>/dev/null | head -1 | awk '{print $2}'), процесс: $(pgrep -f "$x" >/dev/null && echo работает || echo нет)"
    local n; n=$(sql "SELECT count(*) FROM nodes;")
    if [[ -n $n && $n != 0 ]]; then
        info "Узлы ($n):"
        sql "SELECT name,address,port,status,latency_ms,panel_version,datetime(last_heartbeat,'unixepoch') FROM nodes ORDER BY id;" | \
            awk -F'|' '{printf "   %-18s %-22s %-6s %-8s %5sms %-22s %s\n",$1,$2,$3,$4,$5,$6,$7}'
    fi
    info "Сайдкары:"
    local pat
    for pat in qwdtt olcrtc caddy-naive anytls mieru csqtt mtg tuic hysteria; do
        local pids; pids=$(pgrep -f "bin/${pat}" | tr '\n' ' ')
        [[ -n $pids ]] && printf '   %-12s pid %s\n' "$pat" "$pids"
    done
    sidecar_units | grep -v '^x-ui' | while read -r un; do printf '   %-28s %s\n' "$un" "$(systemctl is-active "$un" 2>/dev/null)"; done
    info "RAM: $(free -m | awk '/Mem/{print $3"/"$2" МБ"}'), swap: $(free -m | awk '/Swap/{print $3"/"$2" МБ"}'), диск /: $(df -h / | awk 'NR==2{print $3"/"$2" ("$5")"}')"
    info "Нагрузка: $(cut -d' ' -f1-3 /proc/loadavg)"
}

cmd_logs() {
    hdr "Логи"
    local n="${OPT_N:-100}" mode="${OPT_MODE:-}"
    if [[ -z $mode ]] && has_tty; then
        echo " 1) x-ui последние строки  2) только panic/fatal/error  3) follow (Ctrl+C выход)  4) сайдкары"
        ask "Выбор" "1"; mode="$REPLY"; ask "Сколько строк" "$n"; n="$REPLY"
    fi
    case "$mode" in
        2|errors) journalctl -u x-ui -n "$((n*20))" --no-pager | grep -Ei 'panic|fatal|error' | tail -n "$n" ;;
        3|follow) journalctl -u x-ui -f ;;
        4|sidecars) journalctl -u x-ui -n "$((n*10))" --no-pager | grep -Ei 'qwdtt|olcrtc|naive|anytls|mieru|sidecar|tunnel' | tail -n "$n"
                    local u; for u in $(sidecar_units | grep -v '^x-ui\|@'); do echo "--- $u"; journalctl -u "$u" -n 20 --no-pager; done ;;
        *) journalctl -u x-ui -n "$n" --no-pager ;;
    esac
}

cmd_restart() {
    need_root
    hdr "Перезапуск"
    local what="${OPT_WHAT:-}"
    if [[ -z $what ]]; then echo " 1) панель x-ui  2) только Xray  3) сайдкары (юниты lucx-*)  0) назад"; ask "Выбор" "0"; what="$REPLY"; fi
    case "$what" in
        1|panel) warn "Рестарт x-ui рвёт клиентов qWDTT примерно на 70 с."; confirm "Перезапустить x-ui?" && run systemctl restart x-ui && { [[ $LUCX_DRY_RUN == 1 ]] || verify_panel; } ;;
        2|xray) confirm "Перезапустить Xray (через x-ui restart-xray / API)?" || return 0
            if "$XUI_BIN" help 2>&1 | grep -q restart-xray; then run "$XUI_BIN" restart-xray
            else run api_call POST /panel/api/server/restartXrayService && ok "Xray перезапущен"; fi ;;
        3|sidecars) local u; for u in $(sidecar_units | grep -v '^x-ui\|@'); do confirm "Перезапустить $u?" && run systemctl restart "$u"; done
            info "qWDTT/olcRTC запускаются панелью — их перезапускает рестарт x-ui." ;;
    esac
}

# API-токен для локальных вызовов (имя lucx-cli, остальные токены не трогает).
api_token() { "$XUI_BIN" setting -getApiToken -tokenName lucx-cli 2>/dev/null | grep -Eo '[A-Za-z0-9._-]{20,}' | tail -1; }
api_call() {   # METHOD PATH [data]
    local tok base; tok=$(api_token) || true
    [[ -n $tok ]] || { err "Не получил API-токен"; return 1; }
    base="$(panel_scheme)://127.0.0.1:$(panel_port)$(panel_path)"; base="${base%/}"
    curl -ksS --max-time 300 -X "$1" -H "Authorization: Bearer $tok" -H 'Accept: application/json' ${3:+-H 'Content-Type: application/json' --data-binary "$3"} "$base$2"
}

cmd_reset_access() {
    need_root; need_xui
    hdr "Сброс доступа к панели"
    "$XUI_BIN" setting -show 2>/dev/null | grep -Ev 'password|token' || true
    echo " 1) логин и пароль  2) путь панели (webBasePath)  3) порт панели  4) сбросить 2FA  0) назад"
    ask "Выбор" "0"
    case "$REPLY" in
        1) ask "Новый логин" "admin"; local u="$REPLY"; ask_secret "Новый пароль (пусто — сгенерировать)"; local pw="$REPLY"
           [[ -z $pw ]] && pw=$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 16) && info "Сгенерирован пароль: $pw"
           confirm "Установить логин $u и новый пароль?" && run "$XUI_BIN" setting -username "$u" -password "$pw" ;;
        2) ask "Новый путь (например /$(tr -dc 'a-z0-9' </dev/urandom | head -c 12)/)" ""; [[ -n $REPLY ]] && confirm "Сменить путь на $REPLY?" && run "$XUI_BIN" setting -webBasePath "$REPLY" ;;
        3) ask "Новый порт" ""; [[ $REPLY =~ ^[0-9]+$ ]] && confirm "Сменить порт на $REPLY (не забудьте UFW)?" && run "$XUI_BIN" setting -port "$REPLY" ;;
        4) confirm "Сбросить двухфакторную аутентификацию?" && run "$XUI_BIN" setting -resetTwoFactor ;;
        *) return 0 ;;
    esac
    confirm "Перезапустить x-ui, чтобы применить?" && run systemctl restart x-ui
}

cmd_ssl() {
    need_root; need_xui
    hdr "SSL-сертификат"
    local acme=/root/.acme.sh/acme.sh
    [[ -x $acme ]] && { info "Сертификаты acme.sh:"; "$acme" --list 2>/dev/null; }
    "$XUI_BIN" setting -getCert 2>/dev/null || true
    echo " 1) выпустить/настроить через стоковое меню x-ui (SSL Certificate Management)"
    echo " 2) продлить все сертификаты acme.sh сейчас   0) назад"
    ask "Выбор" "0"
    case "$REPLY" in
        1) warn "Нужен свободный порт 80 (Caddy не трогаем — если он держит 80, выпуск standalone не пройдёт)."
           info "Откроется стоковое меню x-ui: выберите пункт «SSL Certificate Management»."
           confirm "Открыть меню x-ui?" && { if has_tty; then run x-ui <"$TTY"; else run x-ui; fi; } ;;
        2) [[ -x $acme ]] || die "acme.sh не установлен"; confirm "Продлить (acme.sh --cron)?" && run "$acme" --cron --home /root/.acme.sh && confirm "Перезапустить x-ui?" && run systemctl restart x-ui ;;
    esac
}

cmd_geo() {
    need_root; need_xui
    hdr "Обновить геобазы"
    confirm "Обновить geoip/geosite (стоковый x-ui update-all-geofiles) и перезапустить Xray?" || return 0
    local d; d="$BACKUP_ROOT/geo-$(ts)"; run mkdir -p "$d"; run cp -a "$XUI_DIR"/bin/geo*.dat "$d/"
    if grep -q '"update-all-geofiles")' "$XUI_DIR/x-ui.sh" 2>/dev/null; then run x-ui update-all-geofiles
    else
        local f; for f in geoip.dat geosite.dat; do
            run curl -fsSL -o "$XUI_DIR/bin/$f.new" "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/$f" && run mv "$XUI_DIR/bin/$f.new" "$XUI_DIR/bin/$f"
        done
        run systemctl restart x-ui
    fi
    ok "Геобазы обновлены (старые: $d)"
}

cmd_xray_core() {
    need_root; need_xui
    hdr "Обновить ядро Xray"
    local x; x=$(xray_bin); [[ -n $x ]] && info "Текущий Xray: $("$x" version 2>/dev/null | head -1)"
    local vers; vers=$(curl -fsSL --max-time 15 https://api.github.com/repos/XTLS/Xray-core/releases?per_page=8 2>/dev/null | sed -n 's/.*"tag_name": "\(v[^"]*\)".*/\1/p' | tr '\n' ' ')
    info "Доступные: ${vers:-?}"
    ask "Версия (например v25.10.15, 0 — отмена)" "0"; [[ $REPLY == v* ]] || return 0
    local v="$REPLY"
    confirm "Установить Xray $v через API панели (installXray)?" || return 0
    [[ -n $x ]] && run cp -a "$x" "$x.bak-$(ts)"
    run api_call POST "/panel/api/server/installXray/$v" && ok "Запрошено. Проверьте статус через минуту."
}

cmd_cleanup() {
    need_root
    hdr "Очистка"
    ask "Сколько последних x-ui.bak-* оставить" "3"; local k="$REPLY"; [[ $k =~ ^[0-9]+$ ]] || k=3
    ask "Сколько последних бэкапов в $BACKUP_ROOT оставить" "10"; local kb="$REPLY"; [[ $kb =~ ^[0-9]+$ ]] || kb=10
    local -a del=(); local f
    mapfile -t f < <(find "$XUI_DIR" -maxdepth 1 -type f -name 'x-ui.bak-*' -printf '%T@ %p\n' 2>/dev/null | sort -rn | cut -d' ' -f2- | tail -n +"$((k+1))")
    del+=("${f[@]}")
    mapfile -t f < <(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort -r | tail -n +"$((kb+1))")
    del+=("${f[@]}")
    for f in /root/.cache/go-build /root/go/pkg/mod /root/.npm/_cacache "$LUCX_SRC/.install-fork" /tmp/install-fork.*; do [[ -e $f ]] && del+=("$f"); done
    (( ${#del[@]} )) || { ok "Нечего чистить"; return 0; }
    printf '   %s\n' "${del[@]}"
    info "Освободится: $(du -csh "${del[@]}" 2>/dev/null | tail -1 | cut -f1)"
    confirm "Удалить перечисленное?" || return 0
    if [[ -d /root/go/pkg/mod ]]; then run chmod -R u+w /root/go/pkg/mod 2>/dev/null; fi
    run rm -rf "${del[@]}" && ok "Очищено"
}

cmd_monitor() {
    need_root
    hdr "Мониторинг с уведомлением в Telegram"
    local cf=/etc/cron.d/lucx-monitor
    [[ -f $cf ]] && info "Сейчас: $(grep -v '^#' "$cf" | tail -1)"
    echo " 1) включить  2) выключить  3) проверить сейчас  0) назад"
    ask "Выбор" "0"
    case "$REPLY" in
        1) tg_setup || tg_load || { err "Нужен Telegram"; return 1; }
           ask "Интервал, минут" "5"; local m="$REPLY"; [[ $m =~ ^[0-9]+$ ]] || m=5
           install_cmd quiet || return 1
           run install -m 644 /dev/null "$cf"
           [[ $LUCX_DRY_RUN == 1 ]] || printf '# LucX мониторинг\nSHELL=/bin/bash\n*/%s * * * * root /usr/local/bin/lucx monitor-check >/dev/null 2>&1\n' "$m" >"$cf"
           ok "Мониторинг каждые $m мин." ;;
        2) run rm -f "$cf"; ok "Выключено" ;;
        3) cmd_monitor_check; cat /var/lib/lucx/monitor.state 2>/dev/null ;;
    esac
}
cmd_monitor_check() {
    local st=/var/lib/lucx/monitor.state prev cur=""
    mkdir -p /var/lib/lucx
    prev=$(cat "$st" 2>/dev/null)
    systemctl is-active --quiet x-ui || cur+="x-ui DOWN; "
    local name addr port s
    while IFS='|' read -r name addr port s; do
        [[ -z $name ]] && continue
        [[ $s == online ]] && continue
        timeout 5 bash -c ">/dev/tcp/$addr/$port" 2>/dev/null || cur+="узел $name ($addr:$port) $s; "
    done < <(sql "SELECT name,address,port,status FROM nodes WHERE enable=1;")
    if [[ $cur != "$prev" ]]; then
        if [[ -n $cur ]]; then tg_send "⚠️ LucX $(hostname): $cur"; else tg_send "✅ LucX $(hostname): всё восстановилось"; fi
        printf '%s' "$cur" >"$st"
    fi
    [[ -z $cur ]] && echo "OK" || echo "$cur"
}

# ---------- Сеть и безопасность ----------
inbound_ports() { sql "SELECT port,protocol,remark,enable FROM inbounds ORDER BY port;"; }
cmd_ufw() {
    hdr "Firewall (UFW)"
    have ufw || { warn "ufw не установлен (apt install ufw)"; return 0; }
    ufw status numbered 2>/dev/null | head -40
    info "Панель: $(panel_port)/tcp, подписка: $(sub_port)/tcp, SSH: $(ssh_port)/tcp"
    info "Инбаунды (порт|протокол|имя|вкл):"; inbound_ports | sed 's/^/   /'
    [[ ${OPT_SHOW:-0} == 1 ]] && return 0
    echo " 1) открыть порты панели, подписки и всех инбаундов  2) открыть порт  3) закрыть порт  4) включить UFW  0) назад"
    ask "Выбор" "0"
    local sp; sp=$(ssh_port)
    case "$REPLY" in
        1) confirm "Открыть $(panel_port), $(sub_port) и порты инбаундов (tcp+udp)?" || return 0
           run ufw allow "$sp/tcp"; run ufw allow "$(panel_port)/tcp"; run ufw allow "$(sub_port)/tcp"
           local p; for p in $(inbound_ports | cut -d'|' -f1 | sort -u); do run ufw allow "$p"; done ;;
        2) ask "Порт[/tcp|/udp]" ""; [[ -n $REPLY ]] && confirm "Открыть $REPLY?" && run ufw allow "$REPLY" ;;
        3) ask "Порт[/tcp|/udp]" ""; local p="$REPLY"; [[ -n $p ]] || return 0
           if [[ ${p%%/*} == "$sp" ]]; then warn "Это SSH-порт! Закрыв его, вы потеряете доступ."; confirm_yes "Закрыть SSH-порт $p" || return 0; fi
           confirm "Закрыть $p?" && run ufw delete allow "$p" ;;
        4) run ufw allow "$sp/tcp"; confirm "Включить UFW (SSH $sp уже разрешён)?" && run ufw --force enable ;;
    esac
}

cmd_bbr() {
    hdr "BBR и тюнинг сети"
    info "Сейчас: congestion=$(sysctl -n net.ipv4.tcp_congestion_control 2>/dev/null), qdisc=$(sysctl -n net.core.default_qdisc 2>/dev/null), файлов=$(ulimit -n)"
    [[ -f $SYSCTL_FILE ]] && info "Файл $SYSCTL_FILE есть"
    [[ ${OPT_SHOW:-0} == 1 ]] && return 0
    need_root
    echo " 1) включить BBR+fq и тюнинг  2) откатить (удалить $SYSCTL_FILE)  0) назад"
    ask "Выбор" "0"
    case "$REPLY" in
        1) confirm "Записать $SYSCTL_FILE и применить?" || return 0
           [[ -f $SYSCTL_FILE ]] && run cp -a "$SYSCTL_FILE" "$SYSCTL_FILE.bak-$(ts)"
           [[ $LUCX_DRY_RUN == 1 ]] || cat >"$SYSCTL_FILE" <<'S'
# LucX: BBR + тюнинг для VPN (откат: удалить файл и sysctl --system)
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
net.core.rmem_max = 67108864
net.core.wmem_max = 67108864
net.ipv4.tcp_rmem = 4096 87380 67108864
net.ipv4.tcp_wmem = 4096 65536 67108864
net.ipv4.udp_rmem_min = 16384
net.ipv4.udp_wmem_min = 16384
net.core.netdev_max_backlog = 16384
net.ipv4.tcp_fastopen = 3
net.ipv4.tcp_mtu_probing = 1
net.ipv4.tcp_slow_start_after_idle = 0
fs.file-max = 1048576
S
           run sysctl --system >/dev/null
           if [[ ! -f /etc/security/limits.d/99-lucx.conf ]]; then
               [[ $LUCX_DRY_RUN == 1 ]] || printf '* soft nofile 1048576\n* hard nofile 1048576\nroot soft nofile 1048576\nroot hard nofile 1048576\n' >/etc/security/limits.d/99-lucx.conf
           fi
           ok "Применено: $(sysctl -n net.ipv4.tcp_congestion_control)" ;;
        2) confirm "Удалить $SYSCTL_FILE и limits.d/99-lucx.conf?" && run rm -f "$SYSCTL_FILE" /etc/security/limits.d/99-lucx.conf && run sysctl --system >/dev/null && ok "Откат (BBR останется до перезагрузки, если ядро уже переключено)" ;;
    esac
}

cmd_swap() {
    hdr "Swap"
    swapon --show 2>/dev/null; free -m | grep -i swap
    [[ ${OPT_SHOW:-0} == 1 ]] && return 0
    need_root
    ask "Размер swapfile в ГБ (0 — отмена)" "2"; local g="$REPLY"
    [[ $g =~ ^[0-9]+$ ]] && (( g > 0 )) || return 0
    (( $(disk_free_mb /) > g*1024 + 1024 )) || die "Мало места на диске"
    confirm "Создать/пересоздать /swapfile на ${g} ГБ?" || return 0
    if swapon --show=NAME --noheadings | grep -qx /swapfile; then run swapoff /swapfile; fi
    run fallocate -l "${g}G" /swapfile || run dd if=/dev/zero of=/swapfile bs=1M count=$((g*1024))
    run chmod 600 /swapfile; run mkswap /swapfile; run swapon /swapfile
    grep -q '^/swapfile' /etc/fstab || { run cp -a /etc/fstab "/etc/fstab.bak-$(ts)"; [[ $LUCX_DRY_RUN == 1 ]] || echo '/swapfile none swap sw 0 0' >>/etc/fstab; }
    ok "Swap: $(swap_mb) МБ"
}

cmd_fail2ban() {
    need_root
    hdr "Fail2ban"
    have fail2ban-client && fail2ban-client status 2>/dev/null
    confirm "Установить/настроить fail2ban для SSH и входа в панель?" || return 0
    have fail2ban-client || run apt-get install -y fail2ban || die "Не удалось установить fail2ban"
    local jl=/etc/fail2ban/jail.d/lucx.conf flt=/etc/fail2ban/filter.d/lucx-panel.conf
    [[ -f $jl ]] && run cp -a "$jl" "$jl.bak-$(ts)"
    # x-ui пишет «failed login: username=…, IP="1.2.3.4"» в журнал systemd.
    [[ $LUCX_DRY_RUN == 1 ]] || {
        cat >"$flt" <<'F'
[Definition]
failregex = failed login: username=.*, IP="<HOST>"
journalmatch = _SYSTEMD_UNIT=x-ui.service
F
        cat >"$jl" <<J
[sshd]
enabled = true
port = $(ssh_port)
backend = systemd
maxretry = 5
bantime = 1h

[lucx-panel]
enabled = true
filter = lucx-panel
backend = systemd
port = $(panel_port)
maxretry = 5
findtime = 10m
bantime = 1h
J
    }
    run systemctl enable --now fail2ban; run fail2ban-client reload
    ok "Jail sshd и lucx-panel (по журналу x-ui «failed login») активны"
}

cmd_ssh_harden() {
    need_root
    hdr "Смена порта SSH и вход только по ключу"
    local cur; cur=$(ssh_port)
    info "Текущий порт: $cur"
    local ak=/root/.ssh/authorized_keys
    if [[ ! -s $ak ]]; then err "Нет $ak — вход по ключу не настроен. Сначала добавьте ключ (ssh-copy-id)."; return 1; fi
    ok "authorized_keys: $(grep -c '^ssh-\|^ecdsa-' "$ak") ключ(ей)"
    ask "Новый порт SSH (Enter — оставить $cur)" "$cur"; local np="$REPLY"
    [[ $np =~ ^[0-9]+$ ]] && (( np > 0 && np < 65536 )) || die "Неверный порт"
    local nopw=0; confirm "Отключить вход по паролю (только ключ)?" && nopw=1
    confirm_yes "Применить: порт $np (старый $cur пока остаётся), пароль: $([[ $nopw == 1 ]] && echo выкл || echo без изменений)" || return 0
    local dd=/etc/ssh/sshd_config.d f=/etc/ssh/sshd_config.d/99-lucx.conf
    run mkdir -p "$dd"; run cp -a /etc/ssh/sshd_config "/etc/ssh/sshd_config.bak-$(ts)"
    grep -q '^Include /etc/ssh/sshd_config.d' /etc/ssh/sshd_config || warn "sshd_config не подключает sshd_config.d — проверьте вручную"
    have ufw && ufw status | grep -q 'Status: active' && run ufw allow "$np/tcp"
    [[ $LUCX_DRY_RUN == 1 ]] || { { echo "Port $cur"; [[ $np != "$cur" ]] && echo "Port $np"; [[ $nopw == 1 ]] && printf 'PasswordAuthentication no\nKbdInteractiveAuthentication no\nPermitRootLogin prohibit-password\n'; } >"$f"; }
    if ! sshd -t; then err "sshd -t не прошёл — возвращаю"; run rm -f "$f"; return 1; fi
    run systemctl restart ssh 2>/dev/null || run systemctl restart sshd
    [[ $np == "$cur" ]] && { ok "Готово"; return 0; }
    warn "Откройте НОВОЕ окно и проверьте: ssh -p $np root@$(public_ip). Старый порт $cur пока работает."
    if confirm "Новый порт $np работает — закрыть старый $cur?"; then
        [[ $LUCX_DRY_RUN == 1 ]] || sed -i "/^Port $cur\$/d" "$f"
        if sshd -t; then run systemctl restart ssh 2>/dev/null || run systemctl restart sshd; else err "sshd -t не прошёл — старый порт не закрыт"; return 1; fi
        have ufw && run ufw delete allow "$cur/tcp"
        ok "SSH теперь только на $np"
    else info "Старый порт $cur оставлен. Закрыть позже: повторите пункт."; fi
}

cmd_latency() {
    hdr "Скорость и задержка до узлов, проверка портов"
    local name addr port
    while IFS='|' read -r name addr port; do
        [[ -z $name ]] && continue
        local ms t0 t1
        ms=$(ping -c 3 -W 2 "$addr" 2>/dev/null | awk -F/ '/rtt|round-trip/{print $5" мс"}')
        t0=$(date +%s%N)
        if timeout 5 bash -c ">/dev/tcp/$addr/$port" 2>/dev/null; then t1=$(date +%s%N); printf '  %-18s %-22s ping %-10s tcp:%s %s (%d мс)\n' "$name" "$addr" "${ms:-нет}" "$port" "${C_G}открыт${C_0}" $(((t1-t0)/1000000))
        else printf '  %-18s %-22s ping %-10s tcp:%s %s\n' "$name" "$addr" "${ms:-нет}" "$port" "${C_R}закрыт/фильтр${C_0}"; fi
    done < <(sql "SELECT name,address,port FROM nodes ORDER BY id;")
    local ip; ip=$(public_ip)
    info "Порты инбаундов этого сервера ($ip), проверка извне через check-host недоступна — локальная проверка слушания:"
    local p proto rem en
    while IFS='|' read -r p proto rem en; do
        local l; if ss -Hlntu "sport = :$p" 2>/dev/null | grep -q .; then l="${C_G}слушает${C_0}"; else l="${C_Y}не слушает${C_0}"; fi
        printf '   %-6s %-12s %-24s %s\n' "$p" "$proto" "${rem:0:24}" "$l"
    done < <(inbound_ports)
    if have speedtest || have speedtest-cli; then confirm "Запустить speedtest?" && { speedtest 2>/dev/null || speedtest-cli --simple; }; fi
}

# ---------- Клиенты и инбаунды ----------
cmd_qwdtt_clients() {
    hdr "Клиенты qWDTT"
    [[ -r $QWDTT_PW ]] || { warn "Нет $QWDTT_PW"; return 0; }
    have jq || die "Нужен jq (apt install jq)"
    info "Личные пароли (сами пароли не показываются):"
    jq -r '.passwords | to_entries[] | .value | [(.label//"-"), ((.device_ids//[])|length|tostring)+"/"+((.max_devices//1)|tostring), (if (.expires_at//0)==0 then "∞" else (.expires_at|todate) end), (((.down_bytes//0)+(.up_bytes//0))/1048576|floor|tostring)+" МБ"] | @tsv' "$QWDTT_PW" | \
        awk -F'\t' 'BEGIN{printf "   %-40s %-8s %-22s %s\n","метка","устр.","истекает","трафик"}{printf "   %-40s %-8s %-22s %s\n",$1,$2,$3,$4}'
    info "Устройств всего: $(jq '.devices|length' "$QWDTT_PW")"
    local n; n=$(jq '[.passwords|to_entries[]|select((.value.label//"")|test("zz_test|lucx-deploy-probe"))]|length' "$QWDTT_PW")
    info "Тестовых записей (zz_test*, lucx-deploy-probe*): $n"
    [[ ${OPT_SHOW:-0} == 1 || $n == 0 ]] && return 0
    need_root
    confirm "Удалить $n тестовых записей (с бэкапом) и отправить SIGHUP qWDTT?" || return 0
    local b; b="$QWDTT_PW.bak-$(ts)"; run cp -a "$QWDTT_PW" "$b"
    local tmp; tmp=$(mktemp)
    jq '(.passwords) |= with_entries(select((.value.label//"")|test("zz_test|lucx-deploy-probe")|not))' "$QWDTT_PW" >"$tmp" && run install -m 600 "$tmp" "$QWDTT_PW"
    rm -f "$tmp"
    local pid; pid=$(pgrep -f 'bin/qwdtt-linux' | head -1)
    [[ -n $pid ]] && run kill -HUP "$pid" && ok "Удалено, SIGHUP → $pid. Бэкап: $b"
    warn "Если тестовые записи созданы панелью, удалите и клиентов в панели, иначе панель восстановит их при синхронизации."
}

cmd_sub_check() {
    hdr "Проверка подписки"
    local q="${OPT_ARG:-}"
    [[ -z $q ]] && { ask "subId или email клиента" ""; q="$REPLY"; }
    [[ -n $q ]] || return 0
    local sid
    sid=$(sql "SELECT json_extract(c.value,'\$.subId') FROM inbounds i, json_each(json_extract(i.settings,'\$.clients')) c WHERE json_valid(i.settings) AND json_extract(c.value,'\$.email')='${q//\'/}' LIMIT 1;")
    [[ -n $sid ]] || sid="$q"
    info "subId: ${sid:-?}"
    sql "SELECT email,up,down,total,expiry_time,enable FROM client_traffics WHERE email IN (SELECT json_extract(c.value,'\$.email') FROM inbounds i, json_each(json_extract(i.settings,'\$.clients')) c WHERE json_valid(i.settings) AND json_extract(c.value,'\$.subId')='${sid//\'/}');" | \
        while IFS='|' read -r e u d t x en; do
            printf '   %-30s трафик %s МБ / %s  истекает %s  %s\n' "$e" $(((u+d)/1048576)) "$([[ $t == 0 ]] && echo ∞ || echo "$((t/1048576)) МБ")" "$([[ $x == 0 ]] && echo никогда || date -d "@$((x/1000))" '+%F %R')" "$([[ $en == 1 ]] && echo вкл || echo выкл)"
        done
    [[ -n $sid ]] || return 0
    local url; url="$(sub_scheme)://127.0.0.1:$(sub_port)$(sub_path)$sid"
    local ua
    for ua in "qWDTT-Subscription/1.0" "v2rayN/7.0"; do
        info "UA $ua → $url"
        local hdrs body; body=$(mktemp); hdrs=$(curl -ksS --max-time 15 -A "$ua" -D - -o "$body" "$url" 2>&1)
        echo "$hdrs" | grep -Ei '^(HTTP|subscription-userinfo|profile-title|content-type)' | sed 's/^/   /'
        local plain; plain=$(base64 -d "$body" 2>/dev/null || cat "$body")
        printf '   узлов/строк: %s\n' "$(printf '%s\n' "$plain" | grep -c '://')"
        printf '%s\n' "$plain" | grep -o '^[a-z0-9]*://[^?#]*' | sed -E 's#(://)[^@]*@#\1***@#' | head -10 | sed 's/^/     /'
        rm -f "$body"
    done
}

cmd_inbounds_export() {
    need_xui
    hdr "Экспорт инбаундов в JSON"
    local out="${OPT_FILE:-$BACKUP_ROOT/inbounds-$(ts).json}"
    mkdir -p "$(dirname "$out")"
    if have sqlite3 && [[ $(db_kind) == sqlite ]]; then
        sqlite3 -readonly -json "$XUI_DB" "SELECT remark,enable,listen,port,protocol,settings,stream_settings,tag,sniffing FROM inbounds ORDER BY id;" >"$out" || return 1
    else api_call GET /panel/api/inbounds/list >"$out" || return 1; fi
    chmod 600 "$out"; ok "Экспорт: $out ($(du -h "$out" | cut -f1))"
    warn "Файл содержит ключи и UUID клиентов — храните как секрет."
}
cmd_inbounds_import() {
    need_root; need_xui
    hdr "Импорт инбаундов из JSON"
    have jq || die "Нужен jq"
    local f="${OPT_FILE:-}"; [[ -z $f ]] && { ask "Путь к JSON (из экспорта)" ""; f="$REPLY"; }
    [[ -r $f ]] || die "Нет файла $f"
    local -a busy=(); local p t
    while IFS=$'\t' read -r p t; do
        [[ -n $(sql "SELECT id FROM inbounds WHERE port=$p OR tag='${t//\'/}';") ]] && busy+=("$p/$t")
    done < <(jq -r '(if type=="object" then .obj else . end)[] | [.port, .tag] | @tsv' "$f")
    (( ${#busy[@]} )) && warn "Конфликты порт/тег (будут пропущены панелью): ${busy[*]}"
    confirm "Сделать бэкап настроек и импортировать через API панели?" || return 0
    make_backup settings || return 1
    local item
    while IFS= read -r item; do
        [[ $LUCX_DRY_RUN == 1 ]] && { echo "[dry-run] add $(jq -r .remark <<<"$item")"; continue; }
        api_call POST /panel/api/inbounds/add "$item" | jq -r '"   " + (.msg // "?") + " " + ((.success|tostring) // "")'
    done < <(jq -c '(if type=="object" then .obj else . end)[] | del(.id) | .up=0 | .down=0 | .total=(.total//0) | .expiryTime=(.expiryTime//0) | .streamSettings=(.streamSettings // .stream_settings) | del(.stream_settings) | .enable=(.enable==1 or .enable==true)' "$f")
}

# ---------- GitHub ----------
FORK_OURS_PATTERNS=(
    'internal/sub/qwdtt_app_sub*.go' 'internal/web/service/client_sidecar_settings*.go' 'internal/web/service/client_inbound_apply*.go'
    'scripts/install-fork.sh' 'scripts/lucx.sh' 'scripts/apply-vkhash.sh' 'docs/FORK*.md' 'CREDITS.md'
    'internal/lucx/vkcreator/*' 'internal/lucx/wbcreator/*' 'internal/lucx/tunnel/qwdtt_users*.go'
)
is_fork_file() { local f="$1" p; for p in "${FORK_OURS_PATTERNS[@]}"; do # shellcheck disable=SC2053
    [[ $f == $p ]] && return 0; done; return 1; }

cmd_gh_sync() {
    hdr "Синхронизация форка с AlexeyLCP/lucx-ui ($LUCX_UPSTREAM_BRANCH)"
    have git || die "Нужен git"
    local d="$LUCX_SYNC_DIR"
    if [[ ! -d $d/.git ]]; then
        run git clone --branch "$LUCX_BRANCH" "https://github.com/${LUCX_GH_REPO}.git" "$d" || die "clone не удался"
    fi
    cd "$d" || die "нет $d"
    [[ -z $(git status --porcelain) ]] || die "В $d есть незакоммиченные изменения — разберите их вручную."
    git remote get-url upstream >/dev/null 2>&1 || git remote add upstream "$LUCX_UPSTREAM"
    git fetch origin "$LUCX_BRANCH" && git fetch upstream "$LUCX_UPSTREAM_BRANCH" || die "fetch не удался"
    git checkout -q "$LUCX_BRANCH" && git merge -q --ff-only "origin/$LUCX_BRANCH" || die "Локальная ветка разошлась с origin — разберите вручную"
    local behind; behind=$(git rev-list --count "HEAD..upstream/$LUCX_UPSTREAM_BRANCH")
    info "Новых коммитов upstream: $behind"
    (( behind > 0 )) || { ok "Форк уже содержит upstream/$LUCX_UPSTREAM_BRANCH"; return 0; }
    git -c user.name="${GIT_AUTHOR_NAME:-lucx-sync}" -c user.email="${GIT_AUTHOR_EMAIL:-lucx-sync@localhost}" \
        merge --no-ff --no-edit "upstream/$LUCX_UPSTREAM_BRANCH" -m "merge: upstream AlexeyLCP/lucx-ui $LUCX_UPSTREAM_BRANCH into fork" && conf_done=1 || conf_done=0
    if (( conf_done == 0 )); then
        resolve_conflicts || { git merge --abort; err "Слияние прервано (git merge --abort). Форк не изменён."; return 1; }
        git -c user.name="${GIT_AUTHOR_NAME:-lucx-sync}" -c user.email="${GIT_AUTHOR_EMAIL:-lucx-sync@localhost}" commit -q --no-edit || die "commit не удался"
    fi
    # Проверки
    if git grep -nE '^(<<<<<<<|>>>>>>>) ' HEAD -- . ':!*.md' >/dev/null 2>&1; then err "Остались маркеры конфликтов:"; git grep -nE '^(<<<<<<<|>>>>>>>) ' HEAD -- . ':!*.md' | head; return 1; fi
    git diff --check "HEAD^1" HEAD -- '*.go' '*.sh' >/dev/null 2>&1 || warn "git diff --check: есть пробельные замечания (не критично)"
    ok "Маркеров конфликтов нет"
    if have go && (( $(avail_mb) > 1500 )) && [[ ${LUCX_SKIP_BUILD:-0} != 1 ]]; then
        info "go build ./... && go vet (GOFLAGS=$(goflags_by_ram))"
        GOFLAGS="$(goflags_by_ram)" nice -n 10 go build ./... && GOFLAGS="$(goflags_by_ram)" nice -n 10 go vet ./... || { err "Сборка/vet не прошли — push не делаю. Коммит слияния в $d."; return 1; }
    else info "go build пропущен (нет Go, мало RAM или LUCX_SKIP_BUILD=1)"; fi
    git log --oneline -1
    if [[ $NO_PUSH == 1 ]]; then ok "--no-push: слияние готово в $d, push не выполнен"; return 0; fi
    confirm "Сделать git push в origin/$LUCX_BRANCH (обычный, без force)?" || return 0
    if have gh && gh auth status >/dev/null 2>&1; then
        gh auth setup-git >/dev/null 2>&1; git push origin "HEAD:$LUCX_BRANCH"
    else
        ask_secret "GitHub токен (не сохраняется)" || die "Нет токена"
        # Токен живёт только в окружении этого git push и не печатается.
        # shellcheck disable=SC2016
        LUCX_TOK="$REPLY" git -c credential.helper= -c 'credential.helper=!f(){ echo username=x-access-token; echo "password=$LUCX_TOK"; };f' \
            push origin "HEAD:$LUCX_BRANCH" </dev/null
        REPLY=""
    fi
}

resolve_conflicts() {
    local -a files=(); mapfile -t files < <(git diff --name-only --diff-filter=U)
    local f rest=()
    for f in "${files[@]}"; do
        case "$f" in
            *package-lock.json|*pnpm-lock.yaml|*yarn.lock|*go.sum)
                git checkout --theirs -- "$f" && git add -- "$f" && ok "Авто: $f → upstream"
                if [[ $f == *package-lock.json ]] && have npm; then (cd "$(dirname "$f")" && npm install --package-lock-only --ignore-scripts >/dev/null 2>&1) && git add -- "$f" && info "  перегенерирован npm"; fi ;;
            *) rest+=("$f") ;;
        esac
    done
    (( ${#rest[@]} )) || return 0
    warn "Конфликты, требующие решения:"
    for f in "${rest[@]}"; do printf '   %s %s\n' "$f" "$(is_fork_file "$f" && echo "${C_Y}(файл форка)${C_0}")"; done
    has_tty || { err "Нет терминала — решить нельзя."; return 1; }
    for f in "${rest[@]}"; do
        local def="2"; is_fork_file "$f" && def="1"
        while :; do
            ask "$f: 1) наш  2) upstream  3) редактор  0) прервать" "$def"
            case "$REPLY" in
                1) git checkout --ours -- "$f" && git add -- "$f"; break ;;
                2) git checkout --theirs -- "$f" && git add -- "$f"; break ;;
                3) "${EDITOR:-nano}" "$f" <"$TTY" >"$TTY"; if grep -qE '^(<<<<<<<|=======|>>>>>>>)( |$)' "$f"; then warn "Маркеры остались"; else git add -- "$f"; break; fi ;;
                0) return 1 ;;
            esac
        done
    done
    [[ -z $(git diff --name-only --diff-filter=U) ]]
}

# ---------- Роль, узлы ----------
# master: в БД есть узлы; node: есть API-токен scope=node-sync (мастер ходит сюда); иначе single.
detect_role() {
    local n t
    n=$(sql "SELECT count(*) FROM nodes;"); t=$(sql "SELECT count(*) FROM api_tokens WHERE scope='node-sync';")
    if [[ ${n:-0} -gt 0 ]]; then echo master; elif [[ ${t:-0} -gt 0 ]]; then echo node; else echo single; fi
}
cmd_role() {
    hdr "Роль сервера"
    local r; r=$(detect_role)
    case "$r" in
        master) ok "Мастер: узлов в БД — $(sql 'SELECT count(*) FROM nodes;')" ;;
        node)   ok "Узел: есть API-токен node-sync (мастер подключается сюда)" ;;
        *)      info "Одиночный сервер (узлов нет, токена node-sync нет) — можно сделать мастером или узлом" ;;
    esac
    echo "Меню показывает пункты для роли «$r»; пункт 90 в меню — показать все."
}
cmd_node_link() {
    hdr "Подключение узла к мастеру"
    local r; r=$(detect_role)
    if [[ $r == master ]]; then
        info "Проверка связи мастер → узлы (адрес:порт панели узла):"
        local name sch addr port bp st
        while IFS='|' read -r name sch addr port bp st; do
            [[ -z $name ]] && continue
            local code; code=$(curl -ks -o /dev/null -w '%{http_code}' --max-time 8 "${sch:-https}://$addr:$port${bp:-/}" 2>/dev/null)
            printf '   %-18s %s://%s:%s%s  HTTP %s  статус в БД: %s\n' "$name" "${sch:-https}" "$addr" "$port" "${bp:-/}" "${code:-нет}" "$st"
        done < <(sql "SELECT name,scheme,address,port,base_path,status FROM nodes ORDER BY id;")
    fi
    cat <<TXT

${C_W}Как подключить узел:${C_0}
 1. На УЗЛЕ установите форк (пункт «Установить»), откройте порт панели узла в UFW
    (лучше только для IP мастера: ufw allow from <IP мастера> to any port $(panel_port) proto tcp).
 2. На УЗЛЕ выпустите токен: ${C_C}x-ui setting -getApiToken -tokenName master -tokenScope node-sync${C_0}
    (показывается один раз; повторный вызов перевыпускает токен с этим именем).
 3. В панели МАСТЕРА: «Узлы» → «Добавить»: схема (https при SSL), адрес узла, порт панели узла
    ($(panel_port) здесь), путь панели узла ($(panel_path) здесь), API-токен из шага 2.
TXT
    if [[ $r != master ]]; then
        ask "IP/домен мастера для проверки связи (Enter — пропустить)" ""
        [[ -n $REPLY ]] || return 0
        local m="$REPLY"
        ping -c 2 -W 2 "$m" >/dev/null 2>&1 && ok "ping $m — есть" || warn "ping $m — нет (может быть закрыт ICMP)"
        info "Мастер должен достучаться до этого узла: $(public_ip):$(panel_port) — проверьте, что порт открыт в UFW для IP мастера."
    fi
}

# ---------- Массовые действия с клиентами (через API панели) ----------
# Выбор клиентов: по id инбаунда и/или маске email (glob). Срок подписки панель
# пересчитывает сама (bulkAdjust трогает и clients, и client_traffics.expiry_time).
select_emails() {
    ask "ID инбаунда (Enter — все)" "${OPT_INBOUND:-}"; local ib="$REPLY"
    ask "Маска email (glob, например user_*; Enter — все)" "${OPT_MASK:-*}"; local mask="$REPLY"
    local glob; glob=${mask//\'/}
    local where="json_valid(i.settings)"; [[ $ib =~ ^[0-9]+$ ]] && where+=" AND i.id=$ib"
    mapfile -t SEL_EMAILS < <(sql "SELECT DISTINCT json_extract(c.value,'\$.email') FROM inbounds i, json_each(json_extract(i.settings,'\$.clients')) c WHERE $where AND json_extract(c.value,'\$.email') GLOB '$glob' ORDER BY 1;")
    info "Выбрано клиентов: ${#SEL_EMAILS[@]}"
    (( ${#SEL_EMAILS[@]} )) && printf '   %s\n' "${SEL_EMAILS[@]:0:15}" && (( ${#SEL_EMAILS[@]} > 15 )) && echo "   …"
    (( ${#SEL_EMAILS[@]} ))
}
emails_json() { printf '%s\n' "${SEL_EMAILS[@]}" | jq -R . | jq -sc '{emails: .}'; }
cmd_clients_bulk() {
    need_root; need_xui; have jq || die "Нужен jq"
    hdr "Массовые действия с клиентами"
    echo " 1) продлить срок на N дней  2) сбросить трафик  3) включить  4) отключить  0) назад"
    ask "Выбор" "0"; local act="$REPLY"; [[ $act =~ ^[1-4]$ ]] || return 0
    local -a SEL_EMAILS=(); select_emails || return 0
    local body path days
    body=$(emails_json)
    case "$act" in
        1) ask "На сколько дней продлить" "30"; days="$REPLY"; [[ $days =~ ^-?[0-9]+$ ]] || return 0
           body=$(jq -c --argjson d "$days" '. + {addDays: $d}' <<<"$body"); path=/panel/api/clients/bulkAdjust ;;
        2) path=/panel/api/clients/bulkResetTraffic ;;
        3) path=/panel/api/clients/bulkEnable ;;
        4) path=/panel/api/clients/bulkDisable ;;
    esac
    confirm "Применить к ${#SEL_EMAILS[@]} клиентам ($path)?" || return 0
    [[ $LUCX_DRY_RUN == 1 ]] || make_backup settings || return 1
    if [[ $LUCX_DRY_RUN == 1 ]]; then echo "[dry-run] POST $path $body"; return 0; fi
    api_call POST "$path" "$body" | jq -c '{success, msg}'
}

cmd_traffic_report() {
    hdr "Отчёт по трафику"
    local now; now=$(( $(date +%s) * 1000 ))
    info "Топ-15 клиентов по трафику:"
    sql "SELECT email,(up+down)/1048576,total/1048576 FROM client_traffics ORDER BY up+down DESC LIMIT 15;" | \
        awk -F'|' '{printf "   %-34s %8s МБ  лимит %s\n",$1,$2,($3==0?"∞":$3" МБ")}'
    info "Срок истекает в ближайшие ${OPT_DAYS:-7} дней:"
    sql "SELECT email,datetime(expiry_time/1000,'unixepoch','localtime') FROM client_traffics WHERE expiry_time>0 AND expiry_time BETWEEN $now AND $now+${OPT_DAYS:-7}*86400000 ORDER BY expiry_time;" | sed 's/|/  до /; s/^/   /'
    info "Уже истекли: $(sql "SELECT count(*) FROM client_traffics WHERE expiry_time>0 AND expiry_time<$now;")"
    info "Израсходовано ≥90% лимита:"
    sql "SELECT email,(up+down)*100/total FROM client_traffics WHERE total>0 AND (up+down)*10>=total*9 ORDER BY 2 DESC;" | sed 's/|/  /; s/$/%/; s/^/   /'
}

cmd_hwid_reset() {
    need_root; need_xui
    hdr "Сброс HWID / устройств клиента"
    ask "Email клиента" "${OPT_ARG:-}"; local e="$REPLY"; [[ -n $e ]] || return 0
    local sid; sid=$(sql "SELECT json_extract(c.value,'\$.subId') FROM inbounds i, json_each(json_extract(i.settings,'\$.clients')) c WHERE json_valid(i.settings) AND json_extract(c.value,'\$.email')='${e//\'/}' LIMIT 1;")
    info "subId: ${sid:-?}; устройства (client_hwids):"
    sql "SELECT id,device_os,device_model,datetime(last_seen,'unixepoch','localtime') FROM client_hwids WHERE sub_id='${sid//\'/}';" | sed 's/^/   /'
    confirm "Удалить все HWID клиента $e (через API панели)?" || return 0
    if [[ $LUCX_DRY_RUN == 1 ]]; then echo "[dry-run] DELETE /panel/api/clients/hwids/$e"; return 0; fi
    api_call DELETE "/panel/api/clients/hwids/$(jq -rn --arg s "$e" '$s|@uri')" | jq -c '{success, msg}' 2>/dev/null
    info "Устройства qWDTT (личные пароли) — отдельно, см. пункт «Клиенты qWDTT»."
}

reality_inbounds() { sql "SELECT id,port,remark,stream_settings FROM inbounds WHERE json_valid(stream_settings) AND json_extract(stream_settings,'\$.security')='reality';"; }
cmd_reality_check() {
    hdr "Проверка Reality-маскировки"
    local id port rem ss
    while IFS='|' read -r id port rem ss; do
        [[ -z $id ]] && continue
        local dest sni host dport out
        dest=$(jq -r '.realitySettings | (.target // .dest // "")' <<<"$ss"); sni=$(jq -r '.realitySettings.serverNames[0] // ""' <<<"$ss")
        host=${dest%:*}; dport=${dest##*:}; [[ $dport == "$dest" ]] && dport=443
        printf '\n   #%s порт %s «%s»: донор %s, SNI %s\n' "$id" "$port" "$rem" "$dest" "$sni"
        out=$(timeout 10 openssl s_client -connect "$host:$dport" -servername "${sni:-$host}" -tls1_3 -alpn h2 </dev/null 2>&1)
        grep -q 'Protocol *: TLSv1.3\|New, TLSv1.3' <<<"$out" && ok "TLS 1.3" || err "TLS 1.3 нет / донор недоступен"
        grep -q 'ALPN protocol: h2' <<<"$out" && ok "h2 (ALPN)" || warn "h2 не согласован"
        local code; code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 8 --resolve "${sni:-$host}:$dport:$(getent ahostsv4 "$host" | awk 'NR==1{print $1}')" "https://${sni:-$host}:$dport/")
        info "HTTPS-ответ донора: ${code:-нет}"
    done < <(reality_inbounds)
}

cmd_reality_rotate() {
    need_root; need_xui; have jq || die "Нужен jq"
    hdr "Ротация ключей Reality и shortId"
    local x; x=$(xray_bin); [[ -n $x ]] || die "Нет бинарника Xray"
    reality_inbounds | cut -d'|' -f1-3 | sed 's/^/   /'
    ask "ID инбаунда (0 — отмена)" "0"; local id="$REPLY"; [[ $id =~ ^[1-9][0-9]*$ ]] || return 0
    warn "После ротации ВСЕ клиенты этого инбаунда должны обновить подписку, иначе отвалятся."
    confirm_yes "Сгенерировать новые privateKey/publicKey/shortIds для инбаунда #$id" || return 0
    local kp priv pub sid
    kp=$("$x" x25519); priv=$(awk -F': ' '/^Private/{print $2}' <<<"$kp"); pub=$(awk -F': ' '/^Public|^Password/{print $2; exit}' <<<"$kp")
    sid=$(openssl rand -hex 8)
    [[ -n $priv && -n $pub ]] || die "Не разобрал вывод xray x25519"
    [[ $LUCX_DRY_RUN == 1 ]] && { echo "[dry-run] update inbound #$id: новая пара ключей, shortIds=[$sid]"; return 0; }
    make_backup settings || return 1
    local cur; cur=$(api_call GET "/panel/api/inbounds/get/$id") || return 1
    local upd
    upd=$(jq -c --arg p "$priv" --arg k "$pub" --arg s "$sid" '.obj | .streamSettings = (.streamSettings|fromjson
            | .realitySettings.privateKey=$p | .realitySettings.settings.publicKey=$k | .realitySettings.shortIds=[$s] | tojson)' <<<"$cur") || die "Не разобрал инбаунд"
    api_call POST "/panel/api/inbounds/update/$id" "$upd" | jq -c '{success, msg}'
    ok "Новый shortId: $sid. Бэкап до изменения: $LAST_BACKUP"
}

cmd_db_migrate() {
    need_root; need_xui
    hdr "Перенос SQLite ↔ PostgreSQL"
    info "Сейчас: $(db_kind). Используется штатная команда панели: x-ui migrate-db"
    "$XUI_BIN" migrate-db -h 2>&1 | sed 's/^/   /' | head -12
    echo " 1) SQLite → PostgreSQL (копия в --dsn)  2) SQLite → текстовый дамп (.dump)  0) назад"
    ask "Выбор" "0"
    case "$REPLY" in
        1) [[ $(db_kind) == sqlite ]] || die "Панель уже на PostgreSQL"
           ask_secret "DSN PostgreSQL (postgres://user:pass@host:5432/db?sslmode=disable)" || return 1; local dsn="$REPLY"
           confirm_yes "Скопировать $XUI_DB в PostgreSQL" || return 0
           make_backup settings || return 1
           run systemctl stop x-ui
           run "$XUI_BIN" migrate-db -dsn "$dsn"; local rc=$?
           run systemctl start x-ui
           (( rc == 0 )) && cat <<TXT
Данные скопированы. Чтобы панель работала на PostgreSQL, пропишите в $(xui_env_file):
  XUI_DB_TYPE=postgres
  XUI_DB_DSN=<ваш DSN>
и перезапустите x-ui (пункт «Перезапуск»). Обратно: штатного переноса PostgreSQL → SQLite в панели нет —
используйте бэкап SQLite (он сделан выше) и уберите XUI_DB_TYPE.
TXT
           ;;
        2) local out; out="$BACKUP_ROOT/x-ui-$(ts).dump"; run mkdir -p "$BACKUP_ROOT"; run "$XUI_BIN" migrate-db -dump "$out" && ok "Дамп: $out" ;;
    esac
}

cmd_cloud_sync() {
    need_root
    hdr "Синхронизация бэкапов: Яндекс Диск / S3"
    local f; f=$(list_archives | head -1); [[ -n ${OPT_FILE:-} ]] && f="$OPT_FILE"
    [[ -n $f ]] || { warn "Нет архивов"; return 0; }
    local YA="" S3=""
    [[ -r $TG_ENV ]] && { YA=$(sed -n 's/^YADISK_TOKEN=//p' "$TG_ENV"); S3=$(sed -n 's/^S3_TARGET=//p' "$TG_ENV"); }
    echo " 1) отправить последний архив ($(basename "$f"))  2) настроить Яндекс Диск  3) настроить S3 (rclone/aws)  0) назад"
    local ch="${OPT_MODE:-}"; [[ -n $ch ]] || { ask "Выбор" "0"; ch="$REPLY"; }
    case "$ch" in
        2) ask_secret "OAuth-токен Яндекс Диска (oauth.yandex.ru, доступ к WebDAV)" || return 1
           env_set YADISK_TOKEN "$REPLY"; ok "Сохранено в $TG_ENV" ;;
        3) ask "Цель: rclone-remote:путь (например s3:bucket/lucx) или s3://bucket/путь для aws-cli" ""; env_set S3_TARGET "$REPLY"; ok "Сохранено" ;;
        1|send)
           if [[ -n $YA ]]; then
               run curl -fsS -X MKCOL -H "Authorization: OAuth $YA" https://webdav.yandex.ru/lucx-backups/ -o /dev/null || true
               run curl -fsS -T "$f" -H "Authorization: OAuth $YA" "https://webdav.yandex.ru/lucx-backups/$(basename "$f")" && ok "Яндекс Диск: /lucx-backups/$(basename "$f")"
           fi
           if [[ -n $S3 ]]; then
               if [[ $S3 == s3://* ]] && have aws; then run aws s3 cp "$f" "${S3%/}/" && ok "S3: $S3"
               elif have rclone; then run rclone copy "$f" "$S3" && ok "rclone: $S3"
               else warn "Нет rclone/aws-cli"; fi
           fi
           [[ -z $YA && -z $S3 ]] && warn "Ничего не настроено (пункты 2/3)" ;;
    esac
}
env_set() {   # ключ значение -> lucx-backup.env (600), значение не печатается
    [[ $LUCX_DRY_RUN == 1 ]] && { echo "[dry-run] записать $1 в $TG_ENV"; return 0; }
    [[ -f $TG_ENV ]] || install -m 600 /dev/null "$TG_ENV"
    chmod 600 "$TG_ENV"
    local tmp; tmp=$(mktemp); grep -v "^$1=" "$TG_ENV" >"$tmp"; printf '%s=%s\n' "$1" "$2" >>"$tmp"; install -m 600 "$tmp" "$TG_ENV"; rm -f "$tmp"
}

cmd_toggle_dry() { if [[ $LUCX_DRY_RUN == 1 ]]; then LUCX_DRY_RUN=0; ok "Dry-run выключен"; else LUCX_DRY_RUN=1; ok "Dry-run включён: меняющие команды только печатаются"; fi; }
cmd_toggle_lang() { if [[ $LUCX_LANG == en ]]; then LUCX_LANG=ru; else LUCX_LANG=en; fi; ok "Language/язык: $LUCX_LANG"; }

# ---------- Скрипт ----------
install_cmd() {
    local q="${1:-}" tmp; tmp=$(mktemp)
    fetch_raw scripts/lucx.sh "$tmp" || { rm -f "$tmp"; err "Не удалось скачать lucx.sh"; return 1; }
    bash -n "$tmp" || { rm -f "$tmp"; err "Скачанный lucx.sh с ошибкой синтаксиса"; return 1; }
    run install -m 755 "$tmp" /usr/local/bin/lucx; rm -f "$tmp"
    [[ -n $q ]] || ok "Установлено: /usr/local/bin/lucx — дальше запускайте просто: lucx"
}
cmd_self_update() { need_root; hdr "Самообновление скрипта"; install_cmd; }

cmd_help() {
    echo "LucX меню v$LUCX_VERSION_SCRIPT. Использование: lucx [команда] [опции]"
    echo
    local it n=0
    for it in "${MENU_ITEMS[@]}"; do
        IFS='|' read -r sec key _ title desc _ te de <<<"$it"
        [[ $LUCX_LANG == en ]] && { title="$te"; desc="$de"; }
        n=$((n+1)); printf '  %-17s %2d. %s — %s\n' "$key" "$n" "$title" "$desc"
    done
    cat <<'H'
  help                  эта справка
Опции: --yes (подтверждать всё), --no-push (gh-sync), --dry-run (печатать меняющие команды),
       --rotate N, --telegram (бэкапы), --file PATH (restore/import/export), --show (ufw/bbr/swap/qwdtt-clients без изменений),
       -n N, --mode errors|follow|sidecars (logs), --what panel|xray|sidecars (restart), --arg X (sub-check/hwid-reset),
       --inbound ID, --mask GLOB (clients-bulk), --days N (traffic-report), --lang ru|en, --all
Журнал действий: /var/log/lucx.log
H
}

# ================= ТАБЛИЦА МЕНЮ =================
# раздел|команда|функция|название|пояснение   (номера — по порядку, сквозные)
# раздел|команда|функция|название|пояснение|роль(all/master/node)|title EN|description EN
MENU_ITEMS=(
    "Начало|prepare|cmd_prepare|С чего начать / подготовка сервера|домен→IP, время, порты, RAM, GitHub; чек-лист ✓/✗|all|Getting started / server prep|domain→IP, time, ports, RAM, GitHub; ✓/✗ checklist"
    "Начало|role|cmd_role|Роль сервера|мастер / узел / одиночный (по БД)|all|Server role|master / node / single (from DB)"
    "Установка|install|cmd_install|Установить|сборка и установка форка (install-fork.sh)|all|Install|build and install the fork (install-fork.sh)"
    "Установка|update|cmd_update|Обновить|бэкап БД и бинарника, сборка, проверка, авто-откат|all|Update|DB+binary backup, build, verify, auto-rollback"
    "Установка|uninstall|cmd_uninstall|Удалить|стоковый x-ui uninstall, перед этим полный бэкап|all|Uninstall|stock x-ui uninstall after a full backup"
    "Установка|rollback|cmd_rollback|Откат на предыдущую версию|выбор из x-ui.bak-*, проверка, возврат при сбое|all|Roll back version|pick x-ui.bak-*, verify, revert on failure"
    "Установка|preflight|cmd_preflight|Проверка перед обновлением|диск, RAM, Go/Node, GitHub, новые версии|all|Pre-update check|disk, RAM, Go/Node, GitHub, new versions"
    "Установка|update-all|cmd_update_all|Обновить все серверы|по списку lucx-hosts.conf: узлы, затем мастер|master|Update all servers|from lucx-hosts.conf: nodes, then master"
    "Бэкапы|backup-full|cmd_backup_full|Полный бэкап|БД, /etc/x-ui, /usr/local/x-ui, сайдкары, юниты, сертификаты|all|Full backup|DB, /etc/x-ui, /usr/local/x-ui, sidecars, units, certs"
    "Бэкапы|backup-settings|cmd_backup_settings|Бэкап настроек|БД, /etc/x-ui, passwords.json, конфиги сайдкаров|all|Settings backup|DB, /etc/x-ui, passwords.json, sidecar configs"
    "Бэкапы|restore|cmd_restore|Восстановить из бэкапа|выбор архива, бэкап текущего, рестарт, проверка|all|Restore backup|pick archive, backup current, restart, verify"
    "Бэкапы|backup-cron|cmd_backup_cron|Бэкап по расписанию|cron, ротация, отправка в Telegram|all|Scheduled backups|cron, rotation, Telegram upload"
    "Бэкапы|cloud-sync|cmd_cloud_sync|Бэкапы в облако|Яндекс Диск (WebDAV) / S3 (rclone, aws-cli)|all|Cloud backup sync|Yandex Disk (WebDAV) / S3 (rclone, aws-cli)"
    "Бэкапы|migrate|cmd_migrate|Миграция на новый сервер|бэкап → scp → установка → восстановление|all|Migrate to new server|backup → scp → install → restore"
    "Бэкапы|db-migrate|cmd_db_migrate|SQLite ↔ PostgreSQL|штатный x-ui migrate-db (обратно — из бэкапа)|all|SQLite ↔ PostgreSQL|stock x-ui migrate-db (back: from backup)"
    "Обслуживание|status|cmd_status|Статус|версия, сервис, порты, SSL, узлы, сайдкары, RAM/диск|all|Status|version, service, ports, SSL, nodes, sidecars, RAM/disk"
    "Обслуживание|logs|cmd_logs|Логи|x-ui и сайдкары, фильтр ошибок, follow|all|Logs|x-ui and sidecars, error filter, follow"
    "Обслуживание|restart|cmd_restart|Перезапуск|панель / Xray / сайдкары|all|Restart|panel / Xray / sidecars"
    "Обслуживание|reset-access|cmd_reset_access|Сброс доступа|логин, пароль, путь, порт, 2FA|all|Reset access|username, password, path, port, 2FA"
    "Обслуживание|ssl|cmd_ssl|SSL|выпуск (стоковое меню x-ui/acme.sh) и продление|all|SSL|issue (stock x-ui menu/acme.sh) and renew"
    "Обслуживание|geo|cmd_geo|Обновить геобазы|geoip/geosite без пересборки панели|all|Update geo files|geoip/geosite without rebuilding"
    "Обслуживание|xray-core|cmd_xray_core|Обновить ядро Xray|выбор версии, через API панели|all|Update Xray core|pick version, via panel API"
    "Обслуживание|cleanup|cmd_cleanup|Очистка|старые x-ui.bak, бэкапы, кэш Go/npm|all|Cleanup|old x-ui.bak, backups, Go/npm cache"
    "Обслуживание|monitor|cmd_monitor|Мониторинг в Telegram|x-ui и узлы, уведомление о сбое/восстановлении|all|Telegram monitoring|x-ui and nodes, alert on failure/recovery"
    "Сеть и безопасность|ufw|cmd_ufw|Firewall (UFW)|показать/открыть/закрыть порты панели и инбаундов|all|Firewall (UFW)|show/open/close panel and inbound ports"
    "Сеть и безопасность|bbr|cmd_bbr|BBR и тюнинг сети|BBR, fq, лимиты, sysctl (с откатом)|all|BBR & network tuning|BBR, fq, limits, sysctl (revertable)"
    "Сеть и безопасность|swap|cmd_swap|Swap|показать, создать или увеличить swapfile|all|Swap|show, create or grow swapfile"
    "Сеть и безопасность|fail2ban|cmd_fail2ban|Fail2ban|защита SSH и входа в панель|all|Fail2ban|protect SSH and panel login"
    "Сеть и безопасность|ssh-harden|cmd_ssh_harden|Порт SSH и вход по ключу|с проверками, старый порт до подтверждения|all|SSH port & key-only login|with checks, old port kept until confirmed"
    "Сеть и безопасность|latency|cmd_latency|Задержка до узлов и порты|ping/tcp к узлам, слушают ли инбаунды|all|Node latency & ports|ping/tcp to nodes, listening inbounds"
    "Сеть и безопасность|reality-check|cmd_reality_check|Проверка Reality-маскировки|донор доступен, TLS 1.3, h2|all|Reality camouflage check|donor reachable, TLS 1.3, h2"
    "Сеть и безопасность|reality-rotate|cmd_reality_rotate|Ротация ключей Reality|новые ключи и shortId (клиентам обновить подписку)|all|Rotate Reality keys|new keys and shortId (clients must refresh)"
    "Клиенты|qwdtt-clients|cmd_qwdtt_clients|Клиенты qWDTT|список устройств, удаление тестовых записей|all|qWDTT clients|devices list, remove test entries"
    "Клиенты|sub-check|cmd_sub_check|Проверка подписки|по subId/email: узлы, срок, трафик|all|Subscription check|by subId/email: nodes, expiry, traffic"
    "Клиенты|traffic-report|cmd_traffic_report|Отчёт по трафику|топ клиентов, скоро истекают, ≥90% лимита|all|Traffic report|top clients, expiring soon, ≥90% quota"
    "Клиенты|clients-bulk|cmd_clients_bulk|Массовые действия с клиентами|продлить, сбросить трафик, вкл/выкл (по инбаунду/маске)|all|Bulk client actions|extend, reset traffic, enable/disable"
    "Клиенты|hwid-reset|cmd_hwid_reset|Сброс HWID клиента|удалить привязанные устройства (client_hwids)|all|Reset client HWID|remove bound devices (client_hwids)"
    "Клиенты|inbounds-export|cmd_inbounds_export|Экспорт инбаундов|в JSON-файл|all|Export inbounds|to JSON file"
    "Клиенты|inbounds-import|cmd_inbounds_import|Импорт инбаундов|из JSON через API, с бэкапом|all|Import inbounds|from JSON via API, with backup"
    "Узлы|node-link|cmd_node_link|Подключение узла к мастеру|проверка связи и что ввести в панели мастера|all|Link node to master|connectivity check and what to enter on master"
    "GitHub|gh-sync|cmd_gh_sync|Обновить код на GitHub|слить upstream AlexeyLCP/lucx-ui в форк, конфликты, push|all|Sync code on GitHub|merge upstream AlexeyLCP/lucx-ui, conflicts, push"
    "Скрипт|self-update|cmd_self_update|Самообновление / команда lucx|скачать свежий lucx.sh в /usr/local/bin/lucx|all|Self-update / lucx command|install fresh lucx.sh as /usr/local/bin/lucx"
)

SHOW_ALL="${LUCX_SHOW_ALL:-0}"
item_visible() { [[ $SHOW_ALL == 1 || $1 == all || $1 == "$ROLE" || ( $1 == master && $ROLE == single ) ]]; }
show_menu() {
    local it n=0 last="" sec key fn title desc role te de
    ROLE=$(detect_role 2>/dev/null); ROLE=${ROLE:-single}
    if [[ $LUCX_LANG == en ]]; then
        printf '\n%s LucX maintenance menu %s %s(branch %s, script v%s, role: %s)%s\n' "$C_W$C_C" "$C_0" "$C_D" "$LUCX_BRANCH" "$LUCX_VERSION_SCRIPT" "$ROLE" "$C_0"
    else
        printf '\n%s LucX — панель обслуживания %s %s(ветка %s, скрипт v%s, роль: %s)%s\n' "$C_W$C_C" "$C_0" "$C_D" "$LUCX_BRANCH" "$LUCX_VERSION_SCRIPT" "$ROLE" "$C_0"
    fi
    xui_installed && printf ' %sx-ui: %s, %s%s\n' "$C_D" "$(xui_version)" "$(systemctl is-active x-ui 2>/dev/null)" "$C_0"
    [[ $LUCX_DRY_RUN == 1 ]] && printf ' %s[DRY-RUN] ничего не меняется / nothing is changed%s\n' "$C_Y" "$C_0"
    for it in "${MENU_ITEMS[@]}"; do
        IFS='|' read -r sec key fn title desc role te de <<<"$it"
        n=$((n+1))
        item_visible "$role" || continue
        [[ $LUCX_LANG == en ]] && { title="$te"; desc="$de"; sec="$(sec_en "$sec")"; }
        [[ $sec != "$last" ]] && { printf '\n %s%s%s\n' "$C_Y" "$sec" "$C_0"; last="$sec"; }
        printf '  %s%2d%s. %-36s %s%s%s\n' "$C_G" "$n" "$C_0" "$title" "$C_D" "$desc" "$C_0"
    done
    if [[ $LUCX_LANG == en ]]; then
        printf '\n  90. show all items   91. dry-run on/off   92. язык RU/EN\n  %s 0%s. Exit\n' "$C_R" "$C_0"
    else
        printf '\n  90. показать все пункты   91. dry-run вкл/выкл   92. language RU/EN\n  %s 0%s. Выход\n' "$C_R" "$C_0"
    fi
}
sec_en() { case "$1" in Начало) echo Start ;; Установка) echo Install ;; Бэкапы) echo Backups ;; Обслуживание) echo Maintenance ;; "Сеть и безопасность") echo "Network & security" ;; Клиенты) echo Clients ;; Узлы) echo Nodes ;; Скрипт) echo Script ;; *) echo "$1" ;; esac; }

# Журнал действий: что, когда, кто, результат. Аргументы с секретами не пишутся.
log_action() {
    local who="${SUDO_USER:-$(who am i 2>/dev/null | awk '{print $1}')}"
    { printf '%s user=%s(%s) cmd=%s dry=%s rc=%s\n' "$(date '+%F %T%z')" "${who:-?}" "$(id -un)" "$1" "$LUCX_DRY_RUN" "$2" >>/var/log/lucx.log; } 2>/dev/null || true
}

dispatch() {   # по номеру или команде
    local want="$1" it n=0
    for it in "${MENU_ITEMS[@]}"; do
        IFS='|' read -r _ key fn _ <<<"$it"; n=$((n+1))
        if [[ $want == "$key" || $want == "$n" ]]; then "$fn"; local rc=$?; log_action "$key" "$rc"; return $rc; fi
    done
    case "$want" in
        monitor-check) cmd_monitor_check ;;
        90|show-all) [[ $SHOW_ALL == 1 ]] && SHOW_ALL=0 || SHOW_ALL=1 ;;
        91|dry-run) cmd_toggle_dry ;;
        92|lang) cmd_toggle_lang ;;
        help|-h|--help) cmd_help ;;
        *) err "Неизвестная команда: $want (см. help)"; return 2 ;;
    esac
}

main() {
    local cmd=""
    while (( $# )); do
        case "$1" in
            --yes|-y) ASSUME_YES=1 ;;
            --no-push) NO_PUSH=1 ;;
            --dry-run) LUCX_DRY_RUN=1 ;;
            --telegram) OPT_TG=1 ;;
            --show) OPT_SHOW=1 ;;
            --rotate) OPT_ROTATE="$2"; shift ;;
            --file) OPT_FILE="$2"; shift ;;
            -n) OPT_N="$2"; shift ;;
            --mode) OPT_MODE="$2"; shift ;;
            --what) OPT_WHAT="$2"; shift ;;
            --arg) OPT_ARG="$2"; shift ;;
            --inbound) OPT_INBOUND="$2"; shift ;;
            --mask) OPT_MASK="$2"; shift ;;
            --days) OPT_DAYS="$2"; shift ;;
            --lang) LUCX_LANG="$2"; shift ;;
            --all) SHOW_ALL=1 ;;
            *) [[ -z $cmd ]] && cmd="$1" || OPT_ARG="$1" ;;
        esac
        shift
    done
    case "$cmd" in help|-h|--help|"") ;; *) [[ $EUID -eq 0 || $cmd == gh-sync ]] || die "Нужен root (sudo -i)." ;; esac
    if [[ -n $cmd ]]; then dispatch "$cmd"; exit $?; fi
    [[ $EUID -eq 0 ]] || die "Нужен root (sudo -i)."
    has_tty || { cmd_help; exit 0; }
    while :; do
        show_menu
        ask $'\nВыберите пункт' ""
        [[ $REPLY == 0 ]] && { echo "Выход."; exit 0; }
        [[ -z $REPLY ]] && continue
        case "$REPLY" in 90|91|92) dispatch "$REPLY"; continue ;; esac
        ( dispatch "$REPLY" ) || warn "Пункт завершился с ошибкой."
        pause
    done
}

main "$@"
