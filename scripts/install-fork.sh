#!/usr/bin/env bash
# LucX-UI fork (samur005) — one-command install / update.
#
#   bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/install-fork.sh)
#
# Fresh VPS: installs build deps, Go, Node, builds the fork (frontend + Go),
# runs the stock AlexeyLCP install.sh (service, DB, ports), then swaps in the
# fork binary. Existing panel: skips the stock installer and just rebuilds and
# swaps the binary (= update to the latest fork build). Re-run to update.
#
# Env overrides:
#   LUCX_FORK_REPO     git URL of the fork
#   LUCX_BRANCH        branch to build
#   LUCX_SRC           source checkout directory
#   GO_VER             minimum/installed Go version (raised to go.mod if higher)
#   NODE_VER           Node version to install when no suitable Node (>= 22.12) exists
#   LUCX_SKIP_SERVICE  1 = build only (no stock install, no systemd, no binary swap,
#                      no /etc/profile.d writes; apt packages and missing Go/Node are still installed)
#   LUCX_UFW_OPEN      1 = if ufw is active, open the public ports x-ui/xray listen on
#   LUCX_ALLOW_REBOOT  1 = let the stock installer reboot immediately (default: defer)
#
# Whole script is wrapped in main() so it is fully read before running
# (safe for `curl | bash` too).

set -euo pipefail

LUCX_FORK_REPO="${LUCX_FORK_REPO:-https://github.com/samur005/lucx-ui-samur005.git}"
LUCX_BRANCH="${LUCX_BRANCH:-feat/native-vk-hash-generator}"
LUCX_SRC="${LUCX_SRC:-/usr/local/src/lucx-ui-samur005}"
GO_VER="${GO_VER:-1.27.1}"
NODE_VER="${NODE_VER:-24.20.0}"
LUCX_SKIP_SERVICE="${LUCX_SKIP_SERVICE:-0}"
LUCX_UFW_OPEN="${LUCX_UFW_OPEN:-0}"
LUCX_ALLOW_REBOOT="${LUCX_ALLOW_REBOOT:-0}"

NODE_MIN="22.12.0"
XUI_DIR="/usr/local/x-ui"
XUI_BIN="${XUI_DIR}/x-ui"
NODE_BASE="/usr/local/lib/nodejs"
STOCK_INSTALL_URL="https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh"
ONE_LINER="bash <(curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/install-fork.sh)"
BUILD_DIR="${LUCX_SRC}/.install-fork"
BUILD_OUT="${BUILD_DIR}/x-ui"

CURL=(curl -fL --retry 3 --retry-delay 3 --connect-timeout 20)

if [[ -t 1 ]]; then
    C_R=$'\033[0;31m' C_G=$'\033[0;32m' C_Y=$'\033[0;33m' C_B=$'\033[1;34m' C_0=$'\033[0m'
else
    C_R="" C_G="" C_Y="" C_B="" C_0=""
fi

step() { printf '\n%s==> %s%s\n' "$C_B" "$*" "$C_0"; }
info() { printf '    %s\n' "$*"; }
ok()   { printf '%s[OK]%s %s\n' "$C_G" "$C_0" "$*"; }
warn() { printf '%s[ВНИМАНИЕ]%s %s\n' "$C_Y" "$C_0" "$*" >&2; }
die()  { printf '%s[ОШИБКА]%s %s\n' "$C_R" "$C_0" "$*" >&2; exit 1; }

# ver_ge A B  -> true if version A >= B
ver_ge() {
    [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]
}

# Normalise "1.28" -> "1.28.0" (Go release tarballs since 1.21 have a patch number).
norm_go_ver() {
    local v="$1"
    if [[ "$v" =~ ^[0-9]+\.[0-9]+$ ]]; then v="${v}.0"; fi
    printf '%s' "$v"
}

TMPD=""
cleanup() { if [[ -n "$TMPD" && -d "$TMPD" ]]; then rm -rf "$TMPD"; fi; }

git_src() { git -C "$LUCX_SRC" -c safe.directory="$LUCX_SRC" "$@"; }

# Write /etc/profile.d/<name> unless some profile.d script already puts <dir> on PATH.
# Skipped in build-only mode (LUCX_SKIP_SERVICE=1) to leave the system config alone.
write_profile() {
    local name="$1" dir="$2"
    if [[ "$LUCX_SKIP_SERVICE" == "1" ]]; then return 0; fi
    if grep -lsF "$dir" /etc/profile.d/*.sh >/dev/null 2>&1; then
        info "PATH для ${dir} уже прописан в /etc/profile.d — не трогаю"
        return 0
    fi
    echo "export PATH=${dir}:\$PATH" >"/etc/profile.d/${name}"
    info "Записан /etc/profile.d/${name}"
}

have_unit() {
    [[ -f /etc/systemd/system/x-ui.service ]] || systemctl cat x-ui.service >/dev/null 2>&1
}

check_env() {
    step "Проверка системы"
    [[ ${EUID:-$(id -u)} -eq 0 ]] || die "Запустите скрипт от root (sudo -i, затем повторите команду)."
    command -v apt-get >/dev/null 2>&1 || die "Нужен Ubuntu/Debian (apt-get не найден)."
    if [[ -r /etc/os-release ]]; then
        # shellcheck disable=SC1091
        . /etc/os-release
        case " ${ID:-} ${ID_LIKE:-} " in
            *" debian "* | *" ubuntu "*) info "ОС: ${PRETTY_NAME:-$ID}" ;;
            *) die "Поддерживаются только Ubuntu/Debian (обнаружено: ${PRETTY_NAME:-unknown})." ;;
        esac
    else
        die "Не найден /etc/os-release — не могу определить ОС."
    fi

    case "$(uname -m)" in
        x86_64 | amd64) GO_ARCH=amd64 NODE_ARCH=x64 ;;
        aarch64 | arm64) GO_ARCH=arm64 NODE_ARCH=arm64 ;;
        *) die "Архитектура $(uname -m) не поддерживается скриптом (нужна x86_64 или aarch64)." ;;
    esac
    info "Архитектура: $(uname -m) (go: ${GO_ARCH}, node: ${NODE_ARCH})"

    if [[ "$LUCX_SKIP_SERVICE" != "1" && ! -d /run/systemd/system ]]; then
        die "systemd не запущен. Для одной только сборки используйте LUCX_SKIP_SERVICE=1."
    fi

    : "${HOME:=/root}"
    export HOME

    local mem_kb swap_kb
    mem_kb=$(awk '/^MemTotal:/ {print $2}' /proc/meminfo 2>/dev/null || echo 0)
    swap_kb=$(awk '/^SwapTotal:/ {print $2}' /proc/meminfo 2>/dev/null || echo 0)
    if (( (mem_kb + swap_kb) < 1900000 )); then
        warn "RAM+swap меньше ~2 ГБ — сборка frontend/Go может упасть по памяти (OOM)."
        warn "Если так случится, добавьте swap (например, 2 ГБ) и запустите команду ещё раз."
    fi

    local free_kb
    free_kb=$(df -Pk /usr/local 2>/dev/null | awk 'NR==2 {print $4}')
    if [[ -n "$free_kb" ]] && (( free_kb < 3000000 )); then
        warn "Свободно меньше 3 ГБ в /usr/local — сборке нужно ~2–3 ГБ (node_modules, кэш Go)."
    fi
}

install_packages() {
    step "Установка пакетов (curl git tar build-essential xz-utils ...)"
    export DEBIAN_FRONTEND=noninteractive
    # Lock timeout: fresh VPS often runs unattended-upgrades on first boot.
    apt-get -o DPkg::Lock::Timeout=300 update -qq
    apt-get -o DPkg::Lock::Timeout=300 install -y -qq \
        curl git tar ca-certificates build-essential xz-utils iproute2 >/dev/null
    ok "Пакеты установлены"
}

sync_source() {
    step "Исходники форка: ${LUCX_FORK_REPO} (ветка ${LUCX_BRANCH}) -> ${LUCX_SRC}"
    if [[ -d "$LUCX_SRC/.git" ]]; then
        info "Каталог уже есть — обновляю до origin/${LUCX_BRANCH}"
        git_src remote set-url origin "$LUCX_FORK_REPO"
        if [[ -n "$(git_src status --porcelain --untracked-files=no)" ]]; then
            local msg
            msg="install-fork $(date '+%Y-%m-%d %H:%M:%S')"
            warn "Есть локальные изменения — сохраняю их в git stash (\"${msg}\")."
            git_src -c user.name=install-fork -c user.email=install-fork@localhost \
                stash push -m "$msg" >/dev/null
            info "Вернуть их: cd ${LUCX_SRC} && git stash list && git stash pop"
        fi
        git_src fetch origin
        git_src fetch origin "+refs/heads/${LUCX_BRANCH}:refs/remotes/origin/${LUCX_BRANCH}"
        git_src checkout -f -B "$LUCX_BRANCH" "origin/${LUCX_BRANCH}"
        git_src reset --hard "origin/${LUCX_BRANCH}"
    elif [[ -e "$LUCX_SRC" ]]; then
        die "${LUCX_SRC} существует, но это не git-репозиторий. Уберите его или задайте другой LUCX_SRC."
    else
        mkdir -p "$(dirname "$LUCX_SRC")"
        git clone --branch "$LUCX_BRANCH" "$LUCX_FORK_REPO" "$LUCX_SRC"
    fi
    # Keep our build output out of `git status`.
    grep -qxF '/.install-fork/' "$LUCX_SRC/.git/info/exclude" 2>/dev/null \
        || echo '/.install-fork/' >>"$LUCX_SRC/.git/info/exclude"
    SRC_COMMIT=$(git_src rev-parse --short=12 HEAD)
    ok "Исходники на коммите ${SRC_COMMIT}: $(git_src log -1 --format=%s)"

    # Fork feature sanity-check: refuse to build a stock-like binary.
    local miss=0
    if [[ -d "$LUCX_SRC/internal/lucx/vkcreator" ]]; then
        ok "Найден internal/lucx/vkcreator (native vk-hash)"
    else
        warn "Нет каталога internal/lucx/vkcreator"; miss=1
    fi
    if grep -rqs EnsureVkHashes "$LUCX_SRC/internal/lucx/tunnel"; then
        ok "Найден хук EnsureVkHashes"
    else
        warn "Не найден EnsureVkHashes в internal/lucx/tunnel"; miss=1
    fi
    (( miss == 0 )) || die "В ветке ${LUCX_BRANCH} нет фич форка — проверьте LUCX_BRANCH/LUCX_FORK_REPO. Ничего не изменено."
}

ensure_go() {
    local need gomod
    need="$(norm_go_ver "$GO_VER")"
    gomod=$(awk '$1 == "go" {print $2; exit}' "$LUCX_SRC/go.mod" 2>/dev/null || true)
    if [[ -n "$gomod" ]]; then
        gomod="$(norm_go_ver "$gomod")"
        if ! ver_ge "$need" "$gomod"; then
            info "go.mod требует Go ${gomod} — это больше GO_VER=${GO_VER}, ставлю ${gomod}"
            need="$gomod"
        fi
    fi
    step "Go >= ${need}"

    local cur=""
    if [[ -x /usr/local/go/bin/go ]]; then
        cur=$(/usr/local/go/bin/go env GOVERSION 2>/dev/null || true)
        cur="${cur#go}"
    fi
    if [[ -n "$cur" ]] && ver_ge "$cur" "$need"; then
        ok "Go ${cur} уже установлен в /usr/local/go — пропускаю"
    else
        if [[ -n "$cur" ]]; then info "Найден Go ${cur} (< ${need}) — заменяю"; fi
        local file="go${need}.linux-${GO_ARCH}.tar.gz"
        info "Скачиваю ${file}"
        "${CURL[@]}" -sS -o "$TMPD/$file" "https://go.dev/dl/${file}" \
            || die "Не удалось скачать Go (${file})."
        local want=""
        want=$("${CURL[@]}" -sS "https://dl.google.com/go/${file}.sha256" 2>/dev/null | tr -d '[:space:]' || true)
        if [[ "$want" =~ ^[0-9a-f]{64}$ ]]; then
            echo "${want}  $TMPD/$file" | sha256sum -c --quiet - || die "Контрольная сумма Go не совпала."
            ok "sha256 Go проверена"
        else
            warn "Не удалось получить sha256 для ${file} — продолжаю без проверки."
        fi
        rm -rf /usr/local/go
        tar -C /usr/local -xzf "$TMPD/$file"
        rm -f "$TMPD/$file"
        ok "Установлен $(/usr/local/go/bin/go version)"
    fi
    write_profile go.sh /usr/local/go/bin
    export PATH="/usr/local/go/bin:$PATH"
}

# Prints "<version> <bin dir>" of the best Node >= NODE_MIN with npm next to it.
find_node() {
    local cands=() c v best_v="" best_d=""
    c=$(command -v node 2>/dev/null || true)
    if [[ -n "$c" ]]; then cands+=("$c"); fi
    for c in "$NODE_BASE"/node-v*/bin/node; do
        if [[ -x "$c" ]]; then cands+=("$c"); fi
    done
    for c in "${cands[@]+"${cands[@]}"}"; do
        [[ -x "$(dirname "$c")/npm" ]] || continue
        v=$("$c" -v 2>/dev/null || true)
        v="${v#v}"
        [[ "$v" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || continue
        ver_ge "$v" "$NODE_MIN" || continue
        if [[ -z "$best_v" ]] || ! ver_ge "$best_v" "$v"; then
            best_v="$v" best_d="$(dirname "$c")"
        fi
    done
    if [[ -n "$best_v" ]]; then printf '%s %s\n' "$best_v" "$best_d"; fi
}

ensure_node() {
    step "Node.js >= ${NODE_MIN} (frontend просит >= 24; 22.12+ работает с предупреждениями)"
    local found nv nd
    found=$(find_node)
    if [[ -n "$found" ]]; then
        read -r nv nd <<<"$found"
        ok "Node ${nv} уже есть (${nd}) — пропускаю установку"
    else
        local name="node-v${NODE_VER}-linux-${NODE_ARCH}"
        info "Подходящий Node не найден — ставлю ${name} в ${NODE_BASE}"
        "${CURL[@]}" -sS -o "$TMPD/${name}.tar.xz" "https://nodejs.org/dist/v${NODE_VER}/${name}.tar.xz" \
            || die "Не удалось скачать Node ${NODE_VER}."
        local want=""
        want=$("${CURL[@]}" -sS "https://nodejs.org/dist/v${NODE_VER}/SHASUMS256.txt" 2>/dev/null \
            | awk -v f="${name}.tar.xz" '$2 == f {print $1}' || true)
        if [[ "$want" =~ ^[0-9a-f]{64}$ ]]; then
            echo "${want}  $TMPD/${name}.tar.xz" | sha256sum -c --quiet - || die "Контрольная сумма Node не совпала."
            ok "sha256 Node проверена"
        else
            warn "Не удалось получить SHASUMS256 для Node — продолжаю без проверки."
        fi
        mkdir -p "$NODE_BASE"
        rm -rf "${NODE_BASE:?}/${name}"
        tar -C "$NODE_BASE" -xJf "$TMPD/${name}.tar.xz"
        rm -f "$TMPD/${name}.tar.xz"
        nd="${NODE_BASE}/${name}/bin"
        nv="$("$nd/node" -v)"
        nv="${nv#v}"
        ok "Установлен Node ${nv}"
    fi
    if [[ "$nd" == "$NODE_BASE"/* ]]; then
        write_profile nodejs.sh "$nd"
    fi
    export PATH="${nd}:$PATH"
    info "node $(node -v), npm $(npm -v)"
}

build_fork() {
    step "Сборка frontend (npm) — может занять несколько минут"
    mkdir -p "$BUILD_DIR"
    rm -f "$BUILD_OUT"
    local lock_before
    lock_before=$(git_src status --porcelain -- frontend/package-lock.json)
    (
        cd "$LUCX_SRC/frontend"
        # npm ci keeps package-lock.json untouched; fall back to npm install if the lock is out of sync.
        if ! npm ci --no-audit --no-fund; then
            warn "npm ci не прошёл — пробую npm install"
            npm install --no-audit --no-fund
        fi
        npm run build
    ) || die "Сборка frontend не удалась. Работающая панель НЕ тронута."
    # npm install may rewrite package-lock.json; restore it so the next run
    # does not see "local changes" and stash them.
    if [[ -z "$lock_before" && -n "$(git_src status --porcelain -- frontend/package-lock.json)" ]]; then
        git_src checkout -- frontend/package-lock.json
    fi
    ok "Frontend собран"

    step "Сборка Go-бинарника — может занять несколько минут"
    (
        cd "$LUCX_SRC"
        CGO_ENABLED=1 go build -ldflags "-w -s" -o "$BUILD_OUT" .
    ) || { rm -f "$BUILD_OUT"; die "go build не удался. Работающая панель НЕ тронута."; }
    [[ -s "$BUILD_OUT" ]] || die "Бинарник не создан. Работающая панель НЕ тронута."
    chmod 0755 "$BUILD_OUT"
    grep -aqF 'lucx/vkcreator' "$BUILD_OUT" \
        || die "В собранном бинарнике нет vkcreator — что-то не так со сборкой. Панель НЕ тронута."
    NEW_VERSION=$("$BUILD_OUT" -v 2>/dev/null | head -n1 || true)
    ok "Собран ${BUILD_OUT} (версия: ${NEW_VERSION:-?}, коммит ${SRC_COMMIT})"
}

run_stock_installer() {
    if [[ -x "$XUI_BIN" ]] && have_unit; then
        step "Панель уже установлена (${XUI_BIN} + x-ui.service) — стоковый install.sh пропускаю"
        return 0
    fi
    step "Базовая установка панели: стоковый install.sh AlexeyLCP"
    info "Он создаст сервис x-ui, БД /etc/x-ui и порты, задаст вопросы (порт, SSL и т.д.)"
    info "и напечатает URL панели, логин и пароль — СОХРАНИТЕ их."
    info "Стоковый бинарник сразу после этого будет заменён собранным бинарником форка."
    "${CURL[@]}" -sS -o "$TMPD/stock-install.sh" "$STOCK_INSTALL_URL" \
        || die "Не удалось скачать стоковый install.sh."

    local path_prefix=""
    if [[ "$LUCX_ALLOW_REBOOT" != "1" ]]; then
        # The stock installer may reboot 10s after an AWG kernel upgrade.
        # Shadow `reboot`/`shutdown` so it happens after the fork binary is in place.
        mkdir -p "$TMPD/shim"
        local s
        for s in reboot shutdown; do
            cat >"$TMPD/shim/$s" <<EOF_SHIM
#!/bin/sh
echo "[install-fork] ${s} отложен: сначала поставлю бинарник форка, потом попрошу перезагрузить." >&2
touch "$TMPD/reboot-needed"
exit 0
EOF_SHIM
            chmod +x "$TMPD/shim/$s"
        done
        path_prefix="$TMPD/shim:"
    fi

    # stdin is inherited, so the stock prompts still talk to the terminal.
    PATH="${path_prefix}${PATH}" bash "$TMPD/stock-install.sh" \
        || die "Стоковый install.sh завершился с ошибкой. Исправьте причину и запустите команду ещё раз."

    if [[ -f "$TMPD/reboot-needed" ]]; then
        REBOOT_NEEDED=1
    fi
    if ! [[ -x "$XUI_BIN" ]] || ! have_unit; then
        die "После install.sh не найден ${XUI_BIN} или x-ui.service. Запустите команду ещё раз."
    fi
    ok "Стоковая панель установлена"
}

wait_active() {
    local i st
    for i in $(seq 1 20); do
        st=$(systemctl is-active x-ui 2>/dev/null || true)
        if [[ "$st" == "active" && $i -ge 3 ]]; then return 0; fi
        if [[ "$st" == "failed" ]]; then return 1; fi
        sleep 1
    done
    [[ "$(systemctl is-active x-ui 2>/dev/null || true)" == "active" ]]
}

swap_binary() {
    step "Замена бинарника панели на сборку форка"
    install -m 0755 "$BUILD_OUT" "${XUI_BIN}.new"
    BACKUP="${XUI_BIN}.bak-$(date +%Y%m%d%H%M%S)"
    systemctl stop x-ui
    if ! cp -a "$XUI_BIN" "$BACKUP"; then
        rm -f "${XUI_BIN}.new"; systemctl start x-ui || true
        die "Не удалось сделать бэкап бинарника (место на диске?). Панель запущена на старом бинарнике."
    fi
    if ! mv -f "${XUI_BIN}.new" "$XUI_BIN"; then
        systemctl start x-ui || true
        die "Не удалось заменить бинарник. Панель запущена на старом бинарнике."
    fi
    chmod +x "$XUI_BIN"
    info "Бэкап старого бинарника: ${BACKUP}"
    systemctl start x-ui || true
    if wait_active; then
        ok "systemctl is-active x-ui: active"
    else
        warn "x-ui не запустился с новым бинарником — откатываю на ${BACKUP}"
        journalctl -u x-ui -n 30 --no-pager 2>/dev/null || true
        systemctl stop x-ui 2>/dev/null || true
        cp -a "$BACKUP" "$XUI_BIN"
        systemctl start x-ui || true
        if wait_active; then
            die "Откат выполнен, панель работает на старом бинарнике. Смотрите лог выше."
        fi
        die "Откат выполнен, но x-ui всё равно не активен. Проверьте: journalctl -u x-ui -n 100"
    fi
}

# Prints listening TCP sockets owned by x-ui / xray: "<proc> <addr:port>".
xui_sockets() {
    ss -Htlnp 2>/dev/null | awk '
        /users:\(\("x-ui"/ {print "x-ui", $4; next}
        /users:\(\("xray/  {print "xray", $4}'
}

report_ports() {
    step "Порты, которые слушает панель"
    sleep 2
    local socks
    socks=$(xui_sockets | sort -u || true)
    if [[ -z "$socks" ]]; then
        warn "Не вижу слушающих сокетов x-ui (ss -tlnp). Проверьте: systemctl status x-ui"
        return 0
    fi
    printf '%s\n' "$socks" | sed 's/^/    /'

    # Public (non-loopback) ports only.
    PUBLIC_PORTS=$(printf '%s\n' "$socks" | awk '{print $2}' \
        | grep -vE '^(127\.|\[::1\]|localhost)' | sed -E 's/.*:([0-9]+)$/\1/' | sort -un | tr '\n' ' ' || true)

    if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q '^Status: active'; then
        step "ufw активен"
        local p missing=()
        for p in $PUBLIC_PORTS; do
            if ufw status 2>/dev/null | grep -qE "^${p}(/tcp)?[[:space:]]"; then
                info "порт ${p}/tcp уже разрешён"
            else
                missing+=("$p")
            fi
        done
        if (( ${#missing[@]} == 0 )); then
            ok "Все публичные порты панели уже открыты в ufw"
        elif [[ "$LUCX_UFW_OPEN" == "1" ]]; then
            for p in "${missing[@]}"; do ufw allow "${p}/tcp" >/dev/null && ok "ufw allow ${p}/tcp"; done
        else
            warn "В ufw не открыты порты: ${missing[*]}"
            info "Откройте нужные (панель, подписка, инбаунды) вручную, например:"
            for p in "${missing[@]}"; do info "  ufw allow ${p}/tcp"; done
            info "или перезапустите скрипт с LUCX_UFW_OPEN=1 — он откроет все эти порты сам."
        fi
    fi
}

final_message() {
    local ver
    ver=$("$XUI_BIN" -v 2>/dev/null | head -n1 || true)
    printf '\n%s============================================================%s\n' "$C_G" "$C_0"
    ok "Готово: панель работает на бинарнике форка (${ver:-версия ?}, коммит ${SRC_COMMIT})"
    cat <<EOF

Где проверить фичи форка (в веб-панели):
  • Туннели → карточка qWDTT → блок «Генератор vk_hash»
  • Инбаунды → создать/редактировать → кнопка «Шаблоны»

URL панели, логин и пароль: вывод стокового install.sh выше,
  файл /etc/x-ui/install-result.env (если есть) или команда: x-ui settings

${C_Y}ВАЖНО: никогда не обновляйте панель кнопкой Update в панели, командой
«x-ui update» или стоковым install.sh — они поставят бинарник апстрима БЕЗ фич форка.${C_0}
Для обновления до свежей сборки форка просто запустите ту же команду ещё раз:
  ${ONE_LINER}

Бэкап предыдущего бинарника: ${BACKUP:-нет}
Исходники: ${LUCX_SRC} (ветка ${LUCX_BRANCH})
EOF
    if [[ "${REBOOT_NEEDED:-0}" == "1" ]]; then
        printf '\n%s' "$C_Y"
        cat <<'EOF'
Стоковый установщик поставил новое ядро для AmneziaWG и хотел перезагрузить сервер.
Перезагрузка была отложена, чтобы сначала поставить бинарник форка.
Перезагрузите сервер сейчас:  reboot
EOF
        printf '%s' "$C_0"
    fi
}

main() {
    printf '%sLucX-UI fork installer%s\n' "$C_B" "$C_0"
    info "Репозиторий: ${LUCX_FORK_REPO}"
    info "Ветка:       ${LUCX_BRANCH}"
    info "Исходники:   ${LUCX_SRC}"
    info "Go >= ${GO_VER}, Node (если нужен) ${NODE_VER}"
    if [[ "$LUCX_SKIP_SERVICE" == "1" ]]; then info "Режим LUCX_SKIP_SERVICE=1: только сборка, сервис не трогаю"; fi
    info "Порядок: пакеты → клон → Go → Node → сборка (5–15 мин) → [стоковый install.sh] → замена бинарника"

    check_env
    TMPD=$(mktemp -d /tmp/lucx-fork.XXXXXX)
    trap cleanup EXIT
    SRC_COMMIT="?"
    REBOOT_NEEDED=0
    BACKUP=""

    install_packages
    sync_source
    ensure_go
    ensure_node
    build_fork

    if [[ "$LUCX_SKIP_SERVICE" == "1" ]]; then
        step "LUCX_SKIP_SERVICE=1 — готово, сервис и ${XUI_BIN} не тронуты"
        info "Собранный бинарник: ${BUILD_OUT}"
        return 0
    fi

    run_stock_installer
    swap_binary
    report_ports
    final_message
}

main "$@"
