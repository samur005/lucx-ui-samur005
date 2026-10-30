#!/usr/bin/env bash
# LucX-UI fork (samur005) — интерактивное меню управления панелью.
#
#   bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/lucx.sh)
#
# Неинтерактивно:  lucx.sh install|update|uninstall|gh-sync [--no-push] [--no-build]|backup-full|backup-settings
#
# Добавить пункт меню: написать функцию act_<имя>() и добавить строку в MENU
# ("ключ|Название|функция"). Ключ = аргумент CLI.
#
# Env:
#   LUCX_BRANCH       ветка форка (feat/native-vk-hash-generator)
#   LUCX_RAW_BASE     база raw-URL для скачивания install-fork.sh
#   LUCX_BACKUP_ROOT  каталог бэкапов (/root/lucx-backups)
#   LUCX_SYNC_DIR     рабочий клон для gh-sync (/usr/local/src/lucx-fork-sync)
#   LUCX_FORK_REPO    URL форка; LUCX_UPSTREAM_REPO / LUCX_UPSTREAM_BRANCH — апстрим
#   LUCX_ASSUME_YES   1 = не задавать вопросов да/нет (кроме «yes» при удалении)
#   LUCX_BACKUP_KEEP_OLD 1 = включать в архив старые x-ui.db.bak-* и x-ui.bak-* бинарники

set -euo pipefail

LUCX_BRANCH="${LUCX_BRANCH:-feat/native-vk-hash-generator}"
LUCX_FORK_REPO="${LUCX_FORK_REPO:-https://github.com/samur005/lucx-ui-samur005.git}"
LUCX_UPSTREAM_REPO="${LUCX_UPSTREAM_REPO:-https://github.com/AlexeyLCP/lucx-ui.git}"
LUCX_UPSTREAM_BRANCH="${LUCX_UPSTREAM_BRANCH:-main}"
LUCX_RAW_BASE="${LUCX_RAW_BASE:-https://raw.githubusercontent.com/samur005/lucx-ui-samur005/${LUCX_BRANCH}}"
LUCX_BACKUP_ROOT="${LUCX_BACKUP_ROOT:-/root/lucx-backups}"
LUCX_SYNC_DIR="${LUCX_SYNC_DIR:-/usr/local/src/lucx-fork-sync}"
LUCX_ASSUME_YES="${LUCX_ASSUME_YES:-0}"

XUI_DIR="/usr/local/x-ui"
XUI_BIN="${XUI_DIR}/x-ui"
XUI_ETC="/etc/x-ui"
XUI_DB_DEFAULT="${XUI_ETC}/x-ui.db"
MIN_FREE_MB=3072

# Файлы, где при конфликте слияния по умолчанию предлагается «наша» версия.
FORK_FILES=(
    internal/sub/qwdtt_app_sub.go
    internal/web/service/client_sidecar_settings.go
    scripts/install-fork.sh scripts/lucx.sh scripts/apply-vkhash.sh
    docs/FORK.md docs/FORK.en.md docs/VKHASH.md docs/WBROOM.md docs/UPDATE-VKHASH.md docs/vkhash-autogen.md
    CREDITS.md
    'internal/lucx/vkcreator/*' 'internal/lucx/wbcreator/*'
    'internal/lucx/tunnel/qwdtt*' 'internal/sub/qwdtt*'
)

# Таблица пунктов: ключ|название|функция
MENU=(
    "install|Установить панель (форк)|act_install"
    "update|Обновить панель (бэкап + сборка + проверка + откат)|act_update"
    "uninstall|Удалить панель|act_uninstall"
    "gh-sync|Обновить код на GitHub (слить апстрим в форк)|act_gh_sync"
    "backup-full|Полный бэкап|act_backup_full"
    "backup-settings|Бэкап настроек (БД + конфиги)|act_backup_settings"
)

if [[ -t 1 ]]; then
    C_R=$'\033[0;31m' C_G=$'\033[0;32m' C_Y=$'\033[0;33m' C_B=$'\033[1;34m' C_C=$'\033[0;36m' C_0=$'\033[0m'
else
    C_R="" C_G="" C_Y="" C_B="" C_C="" C_0=""
fi
step() { printf '\n%s==> %s%s\n' "$C_B" "$*" "$C_0"; }
info() { printf '    %s\n' "$*"; }
ok()   { printf '%s[OK]%s %s\n' "$C_G" "$C_0" "$*"; }
warn() { printf '%s[ВНИМАНИЕ]%s %s\n' "$C_Y" "$C_0" "$*" >&2; }
err()  { printf '%s[ОШИБКА]%s %s\n' "$C_R" "$C_0" "$*" >&2; }
die()  { err "$*"; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# ask "вопрос" [y|n] → 0 = да
ask() {
    local q="$1" def="${2:-n}" a hint="[y/N]"
    [[ "$def" == y ]] && hint="[Y/n]"
    if [[ "$LUCX_ASSUME_YES" == 1 ]]; then return 0; fi
    if [[ ! -t 0 && ! -r /dev/tty ]]; then [[ "$def" == y ]]; return; fi
    read -r -p "$q $hint: " a </dev/tty || a=""
    a="${a:-$def}"
    [[ "$a" =~ ^[YyДд] ]]
}

require_root() { [[ ${EUID:-$(id -u)} -eq 0 ]] || die "Запустите от root (sudo -i)."; }

panel_installed() { [[ -x "$XUI_BIN" ]] || [[ -f /etc/systemd/system/x-ui.service ]] || [[ -d "$XUI_ETC" ]]; }

# ---------- окружение БД ----------
xui_env_file() {
    local f
    for f in /etc/default/x-ui /etc/sysconfig/x-ui /etc/conf.d/x-ui; do
        [[ -f "$f" ]] && { echo "$f"; return; }
    done
    echo ""
}
env_val() { # env_val KEY → значение из EnvironmentFile или окружения сервиса
    local key="$1" f v=""
    f="$(xui_env_file)"
    if [[ -n "$f" ]]; then
        v="$(grep -E "^${key}=" "$f" 2>/dev/null | tail -n1 | cut -d= -f2- | sed -e 's/^["'\'']//' -e 's/["'\'']$//')" || true
    fi
    if [[ -z "$v" ]] && have systemctl; then
        v="$(systemctl show -p Environment x-ui 2>/dev/null | tr ' ' '\n' | grep -E "^(Environment=)?${key}=" | tail -n1 | sed -E "s/^(Environment=)?${key}=//")" || true
    fi
    printf '%s' "$v"
}
db_kind() {
    case "$(env_val XUI_DB_TYPE | tr '[:upper:]' '[:lower:]')" in
        postgres|postgresql|pg) echo postgres ;;
        *) echo sqlite ;;
    esac
}
db_path() {
    local d; d="$(env_val XUI_DB_FOLDER)"
    if [[ -n "$d" ]]; then echo "${d%/}/x-ui.db"; else echo "$XUI_DB_DEFAULT"; fi
}

# backup_db <dest_dir> — sqlite: .backup (или копия), postgres: pg_dump
backup_db() {
    local dest="$1" kind db
    kind="$(db_kind)"
    if [[ "$kind" == postgres ]]; then
        local dsn; dsn="$(env_val XUI_DB_DSN)"
        [[ -n "$dsn" ]] || { warn "XUI_DB_TYPE=postgres, но XUI_DB_DSN не найден — БД не сохранена"; return 1; }
        have pg_dump || { warn "pg_dump не установлен (apt install postgresql-client) — БД не сохранена"; return 1; }
        pg_dump --no-owner --format=custom --file="$dest/x-ui.pgdump" "$dsn" \
            || { warn "pg_dump завершился с ошибкой"; return 1; }
        ok "PostgreSQL: pg_dump → x-ui.pgdump"
        return 0
    fi
    db="$(db_path)"
    [[ -f "$db" ]] || { warn "БД $db не найдена"; return 1; }
    if have sqlite3; then
        sqlite3 "$db" ".backup '$dest/x-ui.db'" || { warn "sqlite3 .backup не удался"; return 1; }
        local chk; chk="$(sqlite3 "$dest/x-ui.db" 'PRAGMA integrity_check;' 2>&1 | head -n1 || true)"
        if [[ "$chk" == ok ]]; then ok "SQLite: .backup, integrity_check=ok"; else warn "integrity_check: $chk"; fi
    else
        warn "sqlite3 нет — копирую файл БД (+ -wal/-shm). Для согласованной копии: apt install sqlite3"
        cp -a "$db" "$dest/x-ui.db"
        [[ -f "$db-wal" ]] && cp -a "$db-wal" "$dest/x-ui.db-wal"
        [[ -f "$db-shm" ]] && cp -a "$db-shm" "$dest/x-ui.db-shm"
        ok "SQLite: копия файла"
    fi
}

# пути сертификатов, на которые ссылается панель (настройки + inbounds)
cert_paths() {
    local db; db="$(db_path)"
    {
        if [[ "$(db_kind)" == sqlite && -f "$db" ]] && have sqlite3; then
            sqlite3 "$db" "SELECT value FROM settings WHERE key IN ('webCertFile','webKeyFile','subCertFile','subKeyFile');" 2>/dev/null || true
            sqlite3 "$db" "SELECT stream_settings FROM inbounds;" 2>/dev/null \
                | grep -oE '"(certificateFile|keyFile)"[[:space:]]*:[[:space:]]*"[^"]+"' | sed -E 's/.*"([^"]+)"$/\1/' || true
        fi
    } | grep -E '^/' | sort -u | while read -r p; do
        if [[ -e "$p" ]]; then printf '%s\n' "$p"; fi
    done
}

# конфиги сайдкаров/туннелей (qWDTT passwords.json, wg-keys.dat, Caddyfile/yaml и т.п.)
sidecar_config_paths() {
    local d
    for d in "$XUI_DIR/bin/tunnel" "$XUI_DIR/tunnel" /var/lib/x-ui /var/lib/qwdtt /etc/tproxy-server; do
        [[ -d "$d" ]] || continue
        find "$d" -maxdepth 4 -type f \( -name 'passwords.json' -o -name 'wg-keys.dat' -o -name '*.json' -o -name '*.yaml' \
            -o -name '*.yml' -o -name '*.toml' -o -name 'Caddyfile*' -o -name '*.conf' -o -name '*.env' \) \
            -size -5M ! -name 'access.json' 2>/dev/null
    done | sort -u
}

systemd_units() {
    local u
    for u in /etc/systemd/system/x-ui.service /etc/systemd/system/x-ui.service.d \
             /etc/systemd/system/tproxy-server.service /etc/systemd/system/mtprotoproxy.service \
             /etc/systemd/system/x-ui-*.service /etc/systemd/system/*qwdtt*.service /etc/systemd/system/lucx-*.service; do
        [[ -e "$u" ]] && printf '%s\n' "$u"
    done
    xui_env_file
}

# make_archive <name> <paths...> — tar.gz в $LUCX_BACKUP_ROOT/<дата>/
make_archive() {
    local name="$1"; shift
    local stamp dir tmp arch
    stamp="$(date +%Y%m%d-%H%M%S)"
    dir="$LUCX_BACKUP_ROOT/$stamp"
    mkdir -p "$dir"; chmod 700 "$LUCX_BACKUP_ROOT" "$dir"
    tmp="$(mktemp -d)"
    mkdir -p "$tmp/db"
    backup_db "$tmp/db" || warn "БД не попала в архив"
    {
        echo "created=$(date -Is)"; echo "host=$(hostname)"; echo "db_kind=$(db_kind)"; echo "db_path=$(db_path)"
        if [[ -x "$XUI_BIN" ]]; then echo "xui_version=$("$XUI_BIN" -v 2>/dev/null | head -n1)"; fi
    } >"$tmp/MANIFEST.txt"
    local list=() p rc=0
    for p in "$@"; do [[ -e "$p" ]] && list+=("${p#/}"); done
    mapfile -t list < <(printf '%s\n' ${list[@]+"${list[@]}"} | sort -u)
    printf '/%s\n' ${list[@]+"${list[@]}"} >>"$tmp/MANIFEST.txt"
    arch="$dir/$name-$stamp.tar.gz"
    # пути в архиве: db/, MANIFEST.txt и исходные пути без ведущего «/» (etc/x-ui, usr/local/x-ui …)
    # старые копии БД/бинарников (x-ui.db.bak-*, x-ui.bak-*) не берём: LUCX_BACKUP_KEEP_OLD=1 — включить
    local ex=(--exclude='*.sock')
    if [[ "${LUCX_BACKUP_KEEP_OLD:-0}" != 1 ]]; then
        ex+=(--exclude='etc/x-ui/x-ui.db.bak*' --exclude='etc/x-ui/x-ui.db-wal' --exclude='etc/x-ui/x-ui.db-shm'
             --exclude='usr/local/x-ui/x-ui.bak*' --exclude='usr/local/x-ui/x-ui.stock*'
             --exclude='usr/local/x-ui/x-ui.nopages' --exclude='usr/local/x-ui/x-ui.new' --exclude='usr/local/x-ui/x-ui.old*')
    fi
    tar --warning=no-file-changed "${ex[@]}" -czf "$arch" -C "$tmp" db MANIFEST.txt \
        -C / ${list[@]+"${list[@]}"} || rc=$?
    rm -rf "$tmp"
    (( rc <= 1 )) || die "tar завершился с ошибкой ($rc)"
    sha256sum "$arch" | tee "$arch.sha256" >/dev/null
    chmod 600 "$arch" "$arch.sha256"
    ok "Архив: $arch"
    info "Размер: $(du -h "$arch" | cut -f1)"
    info "SHA256: $(cut -d' ' -f1 "$arch.sha256")"
}

act_backup_settings() {
    step "Бэкап настроек"
    local paths=() p
    [[ -d "$XUI_ETC" ]] && paths+=("$XUI_ETC")
    while read -r p; do [[ -n "$p" ]] && paths+=("$p"); done < <(sidecar_config_paths; systemd_units)
    make_archive settings "${paths[@]}"
}

act_backup_full() {
    step "Полный бэкап"
    local paths=() p
    for p in "$XUI_ETC" "$XUI_DIR" /var/lib/x-ui /var/lib/qwdtt /etc/tproxy-server /etc/amnezia /usr/bin/x-ui; do
        [[ -e "$p" ]] && paths+=("$p")
    done
    while read -r p; do [[ -n "$p" ]] && paths+=("$p"); done < <(systemd_units; cert_paths)
    # сертификаты-симлинки (Let's Encrypt live/ → archive/): добавить реальные файлы
    while read -r p; do
        if [[ -L "$p" ]]; then paths+=("$(readlink -f "$p")"); fi
    done < <(cert_paths)
    if [[ -d /root/.acme.sh ]]; then paths+=(/root/.acme.sh); fi
    make_archive full "${paths[@]}"
}

# ---------- установка / обновление ----------
fetch_installer() { # → путь к install-fork.sh
    local self_dir="" out
    if [[ -n "${BASH_SOURCE[0]:-}" && -f "${BASH_SOURCE[0]}" ]]; then
        self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    fi
    if [[ -n "$self_dir" && -f "$self_dir/install-fork.sh" ]]; then
        echo "$self_dir/install-fork.sh"; return
    fi
    out="$(mktemp /tmp/install-fork.XXXXXX.sh)"
    curl -fsSL --retry 3 --connect-timeout 20 "$LUCX_RAW_BASE/scripts/install-fork.sh" -o "$out" \
        || die "Не удалось скачать install-fork.sh"
    bash -n "$out" || die "Скачанный install-fork.sh не проходит bash -n"
    echo "$out"
}

act_install() {
    step "Установка панели"
    if panel_installed; then
        warn "Похоже, x-ui уже установлен ($XUI_BIN / $XUI_ETC). install-fork.sh пересоберёт и заменит бинарник."
        if ask "Сделать полный бэкап перед установкой?" y; then act_backup_full; fi
        ask "Продолжить установку?" n || { info "Отменено."; return 0; }
    fi
    local inst; inst="$(fetch_installer)"
    bash "$inst"
}

ram_mb() { awk '/MemTotal/ {printf "%d", $2/1024}' /proc/meminfo; }
swap_mb() { awk '/SwapTotal/ {printf "%d", $2/1024}' /proc/meminfo; }

preflight_update() {
    step "Проверки перед обновлением"
    local free_src free_xui ram swap
    mkdir -p /usr/local/src
    free_src="$(df -Pm /usr/local/src | awk 'NR==2{print $4}')"
    free_xui="$(df -Pm "$XUI_DIR" 2>/dev/null | awk 'NR==2{print $4}')"
    ram="$(ram_mb)"; swap="$(swap_mb)"
    info "Свободно: /usr/local/src ${free_src} МБ, $XUI_DIR ${free_xui:-?} МБ; RAM ${ram} МБ, swap ${swap} МБ"
    if (( free_src < MIN_FREE_MB )); then
        warn "Мало места (<${MIN_FREE_MB} МБ) для сборки Go+frontend"
        ask "Продолжить всё равно?" n || return 1
    fi
    if (( ram + swap < 2048 )); then
        warn "RAM+swap < 2 ГБ — сборка frontend/Go может упасть по OOM. Рекомендуется swap (fallocate -l 2G /swapfile && mkswap && swapon)."
        ask "Продолжить всё равно?" n || return 1
    fi
    ok "Проверки пройдены"
}

postcheck_update() {
    local since="$1" fail=0
    step "Проверка после обновления"
    sleep 5
    if systemctl is-active --quiet x-ui; then ok "x-ui active"; else err "x-ui не active"; fail=1; fi
    if "$XUI_BIN" -v >/dev/null 2>&1; then ok "x-ui -v: $("$XUI_BIN" -v 2>&1 | head -n1)"; else err "x-ui -v не работает"; fail=1; fi
    if journalctl -u x-ui --since "$since" --no-pager 2>/dev/null | grep -Ei 'panic|fatal' | head -n5 | grep -q .; then
        err "В журнале есть panic/fatal:"; journalctl -u x-ui --since "$since" --no-pager | grep -Ei 'panic|fatal' | head -n5 >&2
        fail=1
    else
        ok "journalctl: без panic/fatal"
    fi
    return "$fail"
}

act_update() {
    step "Обновление панели"
    panel_installed || die "Панель не установлена — используйте «Установить»."
    preflight_update || { info "Отменено."; return 1; }
    local stamp bdir since ram jobs
    stamp="$(date +%Y%m%d-%H%M%S)"
    bdir="$LUCX_BACKUP_ROOT/$stamp-pre-update"
    mkdir -p "$bdir"; chmod 700 "$LUCX_BACKUP_ROOT" "$bdir"
    backup_db "$bdir" || ask "Бэкап БД не удался. Продолжить без него?" n || return 1
    [[ -x "$XUI_BIN" ]] && cp -a "$XUI_BIN" "$bdir/x-ui.bin" && ok "Бинарник → $bdir/x-ui.bin"
    [[ -d "$XUI_ETC" ]] && cp -a "$XUI_ETC" "$bdir/etc-x-ui"
    ram="$(ram_mb)"; jobs=2; (( ram <= 2048 )) && jobs=1
    info "GOFLAGS=-p=$jobs (RAM ${ram} МБ)"
    since="$(date '+%Y-%m-%d %H:%M:%S')"
    local inst rc=0; inst="$(fetch_installer)"
    LUCX_ALLOW_REBOOT=0 LUCX_UFW_OPEN=0 GOFLAGS="-p=$jobs" nice -n 10 bash "$inst" || rc=$?
    if (( rc == 0 )) && postcheck_update "$since"; then
        ok "Обновление успешно. Бэкап: $bdir"
        return 0
    fi
    err "Обновление не прошло проверку (код $rc)."
    if [[ -f "$bdir/x-ui.bin" ]]; then
        warn "Откат бинарника из $bdir/x-ui.bin"
        systemctl stop x-ui || true
        cp -a "$bdir/x-ui.bin" "$XUI_BIN"
        systemctl start x-ui || true
        sleep 3
        if systemctl is-active --quiet x-ui; then ok "Откат выполнен, x-ui active"; else err "После отката x-ui не active — см. journalctl -u x-ui"; fi
        info "БД из бэкапа не восстанавливалась автоматически: $bdir"
    fi
    return 1
}

# ---------- удаление ----------
act_uninstall() {
    step "Удаление панели"
    panel_installed || { info "Панель не найдена."; return 0; }
    local a
    read -r -p "Удалить панель x-ui? Введите yes для подтверждения: " a </dev/tty || a=""
    [[ "$a" == yes ]] || { info "Отменено."; return 0; }
    if ask "Сделать полный бэкап перед удалением?" y; then act_backup_full; fi
    local cli=""
    for cli in /usr/bin/x-ui "$XUI_DIR/x-ui.sh"; do
        if [[ -f "$cli" ]] && grep -qE '^uninstall\(\)' "$cli"; then
            info "Запуск стокового: $cli uninstall"
            bash "$cli" uninstall </dev/tty
            return
        fi
    done
    warn "Стоковой команды uninstall не найдено — ручное удаление."
    read -r -p "Остановить сервис и удалить $XUI_DIR и $XUI_ETC? Введите yes: " a </dev/tty || a=""
    [[ "$a" == yes ]] || { info "Отменено."; return 0; }
    systemctl stop x-ui 2>/dev/null || true
    systemctl disable x-ui 2>/dev/null || true
    rm -f /etc/systemd/system/x-ui.service
    rm -rf -- /etc/systemd/system/x-ui.service.d
    systemctl daemon-reload; systemctl reset-failed 2>/dev/null || true
    rm -rf -- "$XUI_DIR" "$XUI_ETC"
    rm -f /usr/bin/x-ui
    ok "Панель удалена."
}

# ---------- синхронизация форка с апстримом ----------
is_fork_file() {
    local f="$1" pat
    for pat in "${FORK_FILES[@]}"; do
        # shellcheck disable=SC2053
        [[ "$f" == $pat ]] && return 0
    done
    return 1
}

resolve_conflicts() { # 0 = всё решено, 1 = прервано
    local files f choice auto_lock=0
    mapfile -t files < <(git diff --name-only --diff-filter=U)
    for f in "${files[@]}"; do
        case "$f" in
            frontend/package-lock.json|*/pnpm-lock.yaml|go.sum)
                info "Авто: $f → версия upstream"
                git checkout --theirs -- "$f" && git add -- "$f"
                [[ "$f" == frontend/package-lock.json ]] && auto_lock=1
                ;;
        esac
    done
    if (( auto_lock )) && have npm; then
        info "Пересборка frontend/package-lock.json (npm install --package-lock-only)"
        if (cd frontend && npm install --package-lock-only --ignore-scripts --no-audit --no-fund); then
            git add frontend/package-lock.json
        else
            warn "npm не смог пересобрать lock — оставлен upstream"
        fi
    fi
    while :; do
        mapfile -t files < <(git diff --name-only --diff-filter=U)
        (( ${#files[@]} == 0 )) && return 0
        warn "Конфликты в ${#files[@]} файлах:"
        for f in "${files[@]}"; do
            local tag=""; is_fork_file "$f" && tag=" ${C_C}(файл форка)${C_0}"
            case "$f" in docs/*|*.md|*version*|*VERSION*) tag+=" ${C_Y}(docs/версия)${C_0}";; esac
            printf '    - %s%s\n' "$f" "$tag"
        done
        echo "  1) Показать маркеры конфликтов"
        echo "  2) Открыть каждый файл в \$EDITOR (${EDITOR:-nano})"
        echo "  3) Взять «наш» (форк) для файлов форка, остальное — вручную"
        echo "  4) Взять «наш» для ВСЕХ конфликтных файлов"
        echo "  5) Взять upstream для ВСЕХ конфликтных файлов"
        echo "  6) Я исправил вручную — проверить и продолжить"
        echo "  0) Прервать (git merge --abort)"
        read -r -p "Выбор: " choice </dev/tty || choice=0
        case "$choice" in
            1) for f in "${files[@]}"; do printf '%s--- %s%s\n' "$C_B" "$f" "$C_0"; grep -nE '^(<<<<<<<|=======|>>>>>>>)' -- "$f" | head -n 30; done ;;
            2) for f in "${files[@]}"; do "${EDITOR:-nano}" "$f" </dev/tty; if ! grep -qE '^(<<<<<<<|>>>>>>>)' -- "$f"; then git add -- "$f"; fi; done ;;
            3) for f in "${files[@]}"; do is_fork_file "$f" && git checkout --ours -- "$f" && git add -- "$f" && info "ours: $f"; done ;;
            4) for f in "${files[@]}"; do git checkout --ours -- "$f" && git add -- "$f"; done ;;
            5) for f in "${files[@]}"; do git checkout --theirs -- "$f" && git add -- "$f"; done ;;
            6) for f in "${files[@]}"; do
                   if grep -qE '^(<<<<<<<|>>>>>>>)' -- "$f"; then warn "В $f ещё есть маркеры"; else git add -- "$f"; fi
               done ;;
            0) git merge --abort; warn "Слияние прервано."; return 1 ;;
            *) ;;
        esac
    done
}

git_push_safe() {
    local branch="$1"
    if have gh && gh auth status >/dev/null 2>&1; then
        gh auth setup-git >/dev/null 2>&1 || true
        git push origin "HEAD:$branch"
        return
    fi
    if git push origin "HEAD:$branch" 2>/dev/null </dev/null; then return 0; fi
    local token
    read -r -s -p "GitHub токен (не сохраняется): " token </dev/tty; echo
    [[ -n "$token" ]] || die "Токен не введён"
    local url="${LUCX_FORK_REPO#https://}"
    # токен передаётся через заголовок в окружении процесса git, не через argv и не на диск
    GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0="http.https://github.com/.extraheader" \
        GIT_CONFIG_VALUE_0="AUTHORIZATION: basic $(printf 'x-access-token:%s' "$token" | base64 | tr -d '\n')" \
        git push "https://$url" "HEAD:$branch"
    unset token
}

act_gh_sync() {
    local no_push=0 no_build=0 a
    for a in "$@"; do
        case "$a" in --no-push) no_push=1 ;; --no-build) no_build=1 ;; *) die "Неизвестный флаг: $a" ;; esac
    done
    step "Синхронизация форка с $LUCX_UPSTREAM_REPO ($LUCX_UPSTREAM_BRANCH)"
    have git || die "git не установлен"
    if [[ ! -d "$LUCX_SYNC_DIR/.git" ]]; then
        mkdir -p "$(dirname "$LUCX_SYNC_DIR")"
        git clone --branch "$LUCX_BRANCH" "$LUCX_FORK_REPO" "$LUCX_SYNC_DIR"
    fi
    cd "$LUCX_SYNC_DIR"
    [[ -z "$(git status --porcelain)" ]] || die "В $LUCX_SYNC_DIR есть незакоммиченные изменения — разберитесь вручную."
    git remote get-url upstream >/dev/null 2>&1 || git remote add upstream "$LUCX_UPSTREAM_REPO"
    git fetch origin
    git fetch upstream "$LUCX_UPSTREAM_BRANCH"
    git checkout "$LUCX_BRANCH"
    git merge --ff-only "origin/$LUCX_BRANCH" || die "Локальная ветка разошлась с origin — разберитесь вручную (без force)."
    local behind; behind="$(git rev-list --count "HEAD..upstream/$LUCX_UPSTREAM_BRANCH")"
    if (( behind == 0 )); then ok "Апстрим уже влит, нечего делать."; return 0; fi
    info "Новых коммитов в апстриме: $behind"
    git log --oneline "HEAD..upstream/$LUCX_UPSTREAM_BRANCH" | head -n 20 || true
    if ! git -c user.name="${GIT_AUTHOR_NAME:-lucx-sync}" -c user.email="${GIT_AUTHOR_EMAIL:-lucx-sync@localhost}" \
            merge --no-ff --no-edit "upstream/$LUCX_UPSTREAM_BRANCH" -m "Merge upstream $LUCX_UPSTREAM_BRANCH into $LUCX_BRANCH"; then
        resolve_conflicts || return 1
        git -c user.name="${GIT_AUTHOR_NAME:-lucx-sync}" -c user.email="${GIT_AUTHOR_EMAIL:-lucx-sync@localhost}" commit --no-edit
    fi
    step "Проверки после слияния"
    if git grep -nE '^(<<<<<<<|>>>>>>>) ' -- . ':!*.md' >/dev/null 2>&1; then
        git grep -nE '^(<<<<<<<|>>>>>>>) ' -- . ':!*.md' | head >&2
        die "Найдены маркеры конфликта — push отменён. Исправьте и закоммитьте вручную в $LUCX_SYNC_DIR"
    fi
    git diff --check HEAD^1 HEAD -- . ':!*.md' >/dev/null 2>&1 || warn "git diff --check: есть пробельные замечания (не критично)"
    ok "Маркеров конфликта нет"
    if (( no_build )); then
        info "--no-build: go build/vet пропущены"
    elif have go; then
        if ask "Запустить go build ./... и go vet ./... ?" y; then
            if go build ./... && go vet ./...; then ok "go build/vet ok"; else err "go build/vet упал — push отменён (коммит слияния остался локально)"; return 1; fi
        fi
    else
        info "Go не установлен — сборка пропущена"
    fi
    if (( no_push )); then ok "--no-push: слияние сделано локально в $LUCX_SYNC_DIR, push пропущен"; return 0; fi
    ask "Выполнить git push в $LUCX_BRANCH?" y || { info "Push пропущен."; return 0; }
    git_push_safe "$LUCX_BRANCH" && ok "Запушено (обычный push, без force)."
}

# ---------- меню ----------
menu_find() { local k="$1" row; for row in "${MENU[@]}"; do [[ "${row%%|*}" == "$k" ]] && { echo "$row"; return 0; }; done; return 1; }

run_action() { local row="$1"; shift; local fn="${row##*|}"; "$fn" "$@"; }

show_menu() {
    local i row name choice ver="не установлена"
    [[ -x "$XUI_BIN" ]] && ver="$("$XUI_BIN" -v 2>/dev/null | head -n1 || echo '?')"
    while :; do
        printf '\n%s╔══ LucX-UI (форк samur005) ══╗%s\n' "$C_B" "$C_0"
        printf '  Панель: %s%s%s  |  БД: %s\n\n' "$C_C" "$ver" "$C_0" "$(db_kind)"
        i=1
        for row in "${MENU[@]}"; do
            name="${row#*|}"; name="${name%|*}"
            printf '  %s%d)%s %s\n' "$C_G" "$i" "$C_0" "$name"; i=$((i+1))
        done
        printf '  %s0)%s Выход\n\n' "$C_R" "$C_0"
        read -r -p "Выберите пункт: " choice </dev/tty || exit 0
        [[ "$choice" == 0 || "$choice" == q ]] && exit 0
        if [[ "$choice" =~ ^[0-9]+$ ]] && (( choice >= 1 && choice <= ${#MENU[@]} )); then
            ( run_action "${MENU[choice-1]}" ) || warn "Пункт завершился с ошибкой"
            read -r -p "Enter — вернуться в меню..." _ </dev/tty || true
        else
            warn "Нет такого пункта"
        fi
    done
}

usage() {
    echo "Использование: $0 [команда]"
    local row n
    for row in "${MENU[@]}"; do n="${row#*|}"; printf '  %-16s %s\n' "${row%%|*}" "${n%|*}"; done
    echo "  gh-sync [--no-push] [--no-build]  — без push / без go build+vet"
}

main() {
    case "${1:-}" in
        -h|--help|help) usage; exit 0 ;;
    esac
    # gh-sync работает только с git-клоном и не требует root
    [[ "${1:-}" == gh-sync ]] || require_root
    if [[ $# -eq 0 ]]; then show_menu; fi
    local row; row="$(menu_find "$1")" || { usage; exit 2; }
    shift
    run_action "$row" "$@"
}

main "$@"
