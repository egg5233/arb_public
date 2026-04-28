#!/usr/bin/env bash
set -Eeuo pipefail

# Arb Bot one-click installer for Ubuntu/Debian VPS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/egg5233/arb_public/main/install.sh | bash
#
# Useful overrides:
#   ARB_DIR=/root/arb
#   ARB_REPO_URL=https://github.com/egg5233/arb_public.git
#   ARB_BRANCH=main
#   ARB_GO_VERSION=1.26.0
#   ARB_START=1          # start/restart arb after install
#   ARB_FORCE_UPDATE=1   # discard local repo edits and match origin/branch
#   ARB_OVERWRITE_SERVICE=1
#   ARB_SKIP_START_VALIDATION=1
#   ARB_INSTALL_CHROME=1 # install Chromium for optional CoinGlass scraper

RED=$'\033[0;31m'
GREEN=$'\033[0;32m'
YELLOW=$'\033[1;33m'
BLUE=$'\033[0;34m'
BOLD=$'\033[1m'
NC=$'\033[0m'

info() { echo "${BLUE}[i]${NC} $*"; }
ok() { echo "${GREEN}[ok]${NC} $*"; }
warn() { echo "${YELLOW}[!]${NC} $*"; }
fail() { echo "${RED}[x]${NC} $*" >&2; exit 1; }
step() { echo; echo "${BOLD}${BLUE}==> $*${NC}"; }

ARB_DIR="${ARB_DIR:-/root/arb}"
ARB_REPO_URL="${ARB_REPO_URL:-https://github.com/egg5233/arb_public.git}"
ARB_BRANCH="${ARB_BRANCH:-main}"
ARB_GO_VERSION="${ARB_GO_VERSION:-}"
ARB_START="${ARB_START:-0}"
ARB_FORCE_UPDATE="${ARB_FORCE_UPDATE:-0}"
ARB_OVERWRITE_SERVICE="${ARB_OVERWRITE_SERVICE:-0}"
ARB_SKIP_START_VALIDATION="${ARB_SKIP_START_VALIDATION:-0}"
ARB_INSTALL_CHROME="${ARB_INSTALL_CHROME:-0}"
SERVICE_NAME="${SERVICE_NAME:-arb}"
ARB_REDIS_PASS_EFFECTIVE=""
REDIS_CONFIG_CHANGED=0

require_root() {
  if [ "${EUID:-$(id -u)}" -ne 0 ]; then
    fail "Run as root: sudo bash install.sh"
  fi
}

require_apt() {
  command -v apt-get >/dev/null 2>&1 || fail "This installer supports Ubuntu/Debian with apt-get."
}

validate_overrides() {
  case "$SERVICE_NAME" in
    *[!A-Za-z0-9_.@-]*|'') fail "Unsafe SERVICE_NAME: $SERVICE_NAME" ;;
  esac
  case "$ARB_DIR" in
    /*) ;;
    *) fail "ARB_DIR must be an absolute path" ;;
  esac
  case "$ARB_DIR" in
    *[[:space:]]*) fail "ARB_DIR must not contain whitespace: $ARB_DIR" ;;
  esac
  case "$ARB_DIR" in
    *[!A-Za-z0-9._+@=/-]*) fail "ARB_DIR contains characters unsafe for this installer/systemd unit: $ARB_DIR" ;;
  esac
}

guard_active_service_update() {
  if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null && [ "$ARB_START" != "1" ]; then
    fail "$SERVICE_NAME is active. Updating dependencies, Redis, repo files, or the binary can disrupt the running bot. Rerun with ARB_START=1 for an intentional update/restart, or stop the service first."
  fi
}

service_path() {
  echo "/etc/systemd/system/${SERVICE_NAME}.service"
}

unit_value() {
  local key="$1"
  local path
  path="$(service_path)"
  [ -f "$path" ] || return 0
  awk -F= -v key="$key" '$1 == key { sub(/^[^=]*=/, ""); print; exit }' "$path"
}

unit_config_file() {
  local path
  path="$(service_path)"
  [ -f "$path" ] || return 0
  awk '$0 ~ /^Environment=/ {
    line = $0
    sub(/^Environment=/, "", line)
    n = split(line, parts, /[[:space:]]+/)
    for (i = 1; i <= n; i++) {
      gsub(/^"|"$/, "", parts[i])
      if (parts[i] ~ /^CONFIG_FILE=/) {
        sub(/^CONFIG_FILE=/, "", parts[i])
        print parts[i]
        exit
      }
    }
  }' "$path"
}

validate_existing_service_alignment() {
  local path
  path="$(service_path)"
  if [ ! -f "$path" ] || [ "$ARB_OVERWRITE_SERVICE" = "1" ]; then
    return
  fi

  local wd
  local exec_start
  local config_file
  wd="$(unit_value "WorkingDirectory")"
  exec_start="$(unit_value "ExecStart")"
  config_file="$(unit_config_file)"

  if [ "$wd" != "$ARB_DIR" ] || [ "$exec_start" != "$ARB_DIR/arb" ]; then
    fail "Existing $path does not match ARB_DIR=$ARB_DIR (WorkingDirectory=$wd ExecStart=$exec_start). Set ARB_DIR to the service path, SERVICE_NAME to a new service, or ARB_OVERWRITE_SERVICE=1."
  fi
  if [ -n "$config_file" ] && [ "$config_file" != "$ARB_DIR/config.json" ]; then
    fail "Existing $path uses CONFIG_FILE=$config_file, not $ARB_DIR/config.json. Set matching ARB_DIR or ARB_OVERWRITE_SERVICE=1."
  fi
}

version_ge() {
  # Returns 0 when $1 >= $2.
  [ "$(printf '%s\n' "$2" "$1" | sort -V | head -n1)" = "$2" ]
}

go_version_number() {
  go version 2>/dev/null | awk '{print $3}' | sed 's/^go//'
}

go_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) fail "Unsupported CPU architecture for Go install: $(uname -m)" ;;
  esac
}

latest_go_version() {
  if [ -n "$ARB_GO_VERSION" ]; then
    echo "$ARB_GO_VERSION"
    return
  fi

  local latest
  latest="$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1 | sed 's/^go//')" || true
  if [ -z "$latest" ]; then
    latest="1.26.0"
  fi
  echo "$latest"
}

install_go() {
  step "Install Go"

  local current=""
  if command -v go >/dev/null 2>&1; then
    current="$(go_version_number)"
  fi

  if [ -n "$current" ] && version_ge "$current" "1.26"; then
    ok "Go $current is already installed"
    return
  fi

  local desired
  desired="$(latest_go_version)"
  if ! version_ge "$desired" "1.26"; then
    desired="1.26.0"
  fi

  info "Installing Go $desired ..."
  local arch
  arch="$(go_arch)"
  curl -fsSL "https://go.dev/dl/go${desired}.linux-${arch}.tar.gz" -o /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm -f /tmp/go.tar.gz

  if ! grep -q '/usr/local/go/bin' /etc/profile.d/arb-go.sh 2>/dev/null; then
    echo 'export PATH="/usr/local/go/bin:$PATH"' >/etc/profile.d/arb-go.sh
  fi
  export PATH="/usr/local/go/bin:$PATH"
  ok "Go $(go_version_number) installed"
}

install_node() {
  step "Install Node.js 22"

  if command -v node >/dev/null 2>&1; then
    local major
    major="$(node -v | sed 's/^v//' | cut -d. -f1)"
    if [ "$major" -ge 22 ] 2>/dev/null; then
      ok "Node.js $(node -v) is already installed"
      ok "npm $(npm -v)"
      return
    fi
  fi

  info "Installing Node.js 22 from NodeSource ..."
  curl -fsSL https://deb.nodesource.com/setup_22.x | bash - >/dev/null
  apt-get install -y -qq nodejs >/dev/null
  ok "Node.js $(node -v)"
  ok "npm $(npm -v)"
}

redis_password_from_conf() {
  local conf="/etc/redis/redis.conf"
  if [ -f "$conf" ]; then
    awk '$1 == "requirepass" && $2 != "" {print $2; exit}' "$conf"
  fi
}

redis_password_from_existing_config() {
  local config_path="$ARB_DIR/config.json"
  if [ -f "$config_path" ] && command -v jq >/dev/null 2>&1; then
    jq -r '.redis.password // ""' "$config_path" 2>/dev/null || true
  fi
}

redis_addr_from_config() {
  local config_path="$ARB_DIR/config.json"
  jq -r '.redis.addr // "127.0.0.1:6379"' "$config_path" 2>/dev/null || echo "127.0.0.1:6379"
}

redis_addr_is_local() {
  local addr="$1"
  case "$addr" in
    ""|localhost:*|127.0.0.1:*|0.0.0.0:*) return 0 ;;
    *) return 1 ;;
  esac
}

set_redis_requirepass() {
  local pass="$1"
  local conf="/etc/redis/redis.conf"
  local tmp

  tmp="$(mktemp)"
  awk -v pass="$pass" '
    BEGIN { done = 0 }
    /^[#[:space:]]*requirepass[[:space:]]+/ && done == 0 {
      print "requirepass " pass
      done = 1
      next
    }
    { print }
    END {
      if (done == 0) {
        print ""
        print "requirepass " pass
      }
    }
  ' "$conf" >"$tmp"
  cat "$tmp" >"$conf"
  rm -f "$tmp"
  REDIS_CONFIG_CHANGED=1
}

install_redis() {
  step "Install Redis"

  local redis_installed=0
  if ! command -v redis-server >/dev/null 2>&1; then
    apt-get install -y -qq redis-server >/dev/null
    redis_installed=1
  fi

  local pass
  local config_pass
  local conf_pass
  local config_addr
  local config_exists=0
  if [ -f "$ARB_DIR/config.json" ]; then
    config_exists=1
  fi
  config_pass="$(redis_password_from_existing_config)"
  conf_pass="$(redis_password_from_conf)"
  config_addr="$(redis_addr_from_config)"

  if [ "$config_exists" = "1" ] && [ -n "${ARB_REDIS_PASS:-}" ] && [ "$ARB_REDIS_PASS" != "$config_pass" ]; then
    fail "Existing config.json is preserved, but ARB_REDIS_PASS differs from config redis.password. Update config.json first or unset ARB_REDIS_PASS."
  fi
  if [ "$config_exists" = "1" ] && redis_addr_is_local "$config_addr" && [ "$redis_installed" != "1" ] && [ "$config_pass" != "$conf_pass" ]; then
    fail "Existing config.json redis.password does not match local Redis requirepass. The installer will not rewrite Redis auth for an existing install; fix config or Redis first."
  fi

  if [ "$config_exists" = "1" ]; then
    pass="$config_pass"
    if [ "$redis_installed" = "1" ] && redis_addr_is_local "$config_addr" && [ -n "$pass" ]; then
      info "Configuring newly installed local Redis to match existing config"
      set_redis_requirepass "$pass"
    else
      info "Preserving Redis auth for existing config"
    fi
  else
    pass="${ARB_REDIS_PASS:-$conf_pass}"
    if [ -z "$pass" ]; then
      pass="$(openssl rand -hex 24)"
      info "Setting a new Redis password"
      set_redis_requirepass "$pass"
    else
      if [ "$conf_pass" != "$pass" ]; then
        info "Configuring Redis password for fresh install"
        set_redis_requirepass "$pass"
      else
        info "Using existing Redis password"
      fi
    fi
  fi

  if [ "$config_exists" = "1" ] && ! redis_addr_is_local "$config_addr"; then
    ok "Existing config uses non-local Redis ($config_addr); local Redis auth/state left unchanged"
    ARB_REDIS_PASS_EFFECTIVE="$pass"
    return
  fi

  systemctl enable redis-server >/dev/null 2>&1 || true
  if [ "$redis_installed" = "1" ] || [ "$REDIS_CONFIG_CHANGED" = "1" ]; then
    systemctl restart redis-server
  elif ! systemctl is-active --quiet redis-server 2>/dev/null; then
    systemctl start redis-server
  fi
  sleep 1

  if [ -n "$pass" ]; then
    if redis-cli -a "$pass" ping >/dev/null 2>&1; then
      ok "Redis is running"
    else
      fail "Redis did not respond to authenticated PING"
    fi
  else
    if redis-cli ping >/dev/null 2>&1; then
      ok "Redis is running without auth"
    else
      fail "Redis did not respond to PING"
    fi
  fi

  ARB_REDIS_PASS_EFFECTIVE="$pass"
}

install_chrome_optional() {
  if [ "$ARB_INSTALL_CHROME" != "1" ]; then
    return
  fi

  step "Install Chromium"
  if command -v chromium >/dev/null 2>&1 || command -v chromium-browser >/dev/null 2>&1 || command -v google-chrome >/dev/null 2>&1; then
    ok "Chrome/Chromium is already installed"
    return
  fi

  if apt-get install -y -qq chromium >/dev/null 2>&1; then
    ok "Chromium installed"
  elif apt-get install -y -qq chromium-browser >/dev/null 2>&1; then
    ok "chromium-browser installed"
  else
    warn "Chromium install failed; spot_arb scraper can still be configured later."
  fi
}

clone_or_update_repo() {
  step "Install Arb Bot source"

  mkdir -p "$(dirname "$ARB_DIR")"
  if [ -d "$ARB_DIR/.git" ]; then
    info "Updating existing repo at $ARB_DIR"
    git -C "$ARB_DIR" config --global --add safe.directory "$ARB_DIR" || true
    git -C "$ARB_DIR" fetch origin "$ARB_BRANCH" --quiet
    if [ "$ARB_FORCE_UPDATE" = "1" ]; then
      warn "ARB_FORCE_UPDATE=1: discarding local repo edits and matching origin/$ARB_BRANCH"
      git -C "$ARB_DIR" checkout -B "$ARB_BRANCH" "origin/$ARB_BRANCH" --quiet
      git -C "$ARB_DIR" reset --hard "origin/$ARB_BRANCH" --quiet
    else
      if ! git -C "$ARB_DIR" checkout "$ARB_BRANCH" --quiet 2>/dev/null; then
        git -C "$ARB_DIR" checkout -B "$ARB_BRANCH" "origin/$ARB_BRANCH" --quiet
      fi
      git -C "$ARB_DIR" pull --ff-only origin "$ARB_BRANCH" --quiet || \
        fail "Repo has local changes or divergent history. Resolve manually or rerun with ARB_FORCE_UPDATE=1."
    fi
  else
    if [ -e "$ARB_DIR" ] && [ -n "$(find "$ARB_DIR" -mindepth 1 -maxdepth 1 2>/dev/null)" ]; then
      fail "$ARB_DIR exists but is not a git repo. Move it or set ARB_DIR to another path."
    fi
    info "Cloning $ARB_REPO_URL -> $ARB_DIR"
    git clone --branch "$ARB_BRANCH" --quiet "$ARB_REPO_URL" "$ARB_DIR"
    git -C "$ARB_DIR" config --global --add safe.directory "$ARB_DIR" || true
  fi

  ok "Repo ready: $(git -C "$ARB_DIR" rev-parse --short HEAD)"
}

write_default_config() {
  step "Create config.json"

  local config_path="$ARB_DIR/config.json"
  if [ -f "$config_path" ]; then
    ok "Existing config.json preserved"
    return
  fi

  local dashboard_password
  dashboard_password="$(openssl rand -hex 16)"

  cat >"$config_path" <<EOF
{
  "dry_run": true,
  "tradfi_signed": false,
  "exchanges": {
    "binance": {"api_key": "", "secret_key": "", "address": {"APT": "", "BEP20": ""}},
    "bybit": {"api_key": "", "secret_key": "", "address": {"APT": "", "BEP20": ""}},
    "gateio": {"api_key": "", "secret_key": "", "address": {"APT": "", "BEP20": ""}},
    "bitget": {"api_key": "", "secret_key": "", "passphrase": "", "address": {"APT": "", "BEP20": ""}},
    "okx": {"api_key": "", "secret_key": "", "passphrase": "", "address": {"APT": "", "BEP20": ""}},
    "bingx": {"api_key": "", "secret_key": "", "address": {"APT": "", "BEP20": ""}}
  },
  "redis": {
    "addr": "127.0.0.1:6379",
    "password": "$ARB_REDIS_PASS_EFFECTIVE",
    "db": 2
  },
  "dashboard": {
    "addr": ":8080",
    "password": "$dashboard_password"
  },
  "fund": {
    "max_positions": 1,
    "leverage": 3,
    "capital_per_leg": 0
  },
  "strategy": {
    "top_opportunities": 25,
    "scan_minutes": [10, 20, 30, 35, 40, 45, 50],
    "entry_scan_minute": 40,
    "exit_scan_minute": 30,
    "rotate_scan_minute": 35,
    "rebalance_scan_minute": 20,
    "discovery": {
      "min_hold_time_hours": 16,
      "max_cost_ratio": 0.5,
      "max_price_gap_bps": 200,
      "price_gap_free_bps": 40,
      "max_gap_recovery_intervals": 1,
      "allow_mixed_intervals": false,
      "delist_filter": true,
      "contract_refresh_min": 60,
      "persistence": {
        "lookback_min_1h": 90,
        "min_count_1h": 1,
        "lookback_min_4h": 180,
        "min_count_4h": 1,
        "lookback_min_8h": 360,
        "min_count_8h": 1,
        "spread_stability_ratio_1h": 0.5,
        "spread_stability_oi_rank_1h": 0,
        "spread_volatility_max_cv": 0,
        "spread_volatility_min_samples": 10
      }
    },
    "entry": {
      "slippage_limit_bps": 50,
      "min_chunk_usdt": 10,
      "entry_timeout_sec": 300,
      "loss_cooldown_hours": 4,
      "re_enter_cooldown_hours": 1,
      "backtest_days": 3,
      "backtest_min_profit": 0
    },
    "exit": {
      "depth_timeout_sec": 300,
      "enable_spread_reversal": true,
      "spread_reversal_tolerance": 1,
      "zero_spread_tolerance": 2,
      "max_gap_bps": 10
    },
    "rotation": {
      "threshold_bps": 100,
      "cooldown_min": 180
    }
  },
  "risk": {
    "margin_l3_threshold": 0.5,
    "margin_l4_threshold": 0.8,
    "margin_l4_headroom": 0.05,
    "margin_l5_threshold": 0.95,
    "l4_reduce_fraction": 0.3,
    "margin_safety_multiplier": 2,
    "entry_margin_headroom": 0.8,
    "withdraw_min_interval_ms": 11000,
    "risk_monitor_interval_sec": 300,
    "enable_capital_allocator": false
  },
  "spot_arb": {
    "enabled": false,
    "schedule": "15,35",
    "chrome_path": ""
  },
  "spot_futures": {
    "enabled": false,
    "max_positions": 1,
    "leverage": 3,
    "monitor_interval_sec": 60,
    "min_net_yield_apr": 0.1,
    "max_borrow_apr": 0.5,
    "auto_enabled": false,
    "auto_dry_run": true,
    "capital_separate_usdt": 200,
    "capital_unified_usdt": 500,
    "scanner_mode": "native",
    "enable_spot_only_exchanges": false,
    "backtest_enabled": false
  },
  "price_gap": {
    "enabled": false,
    "paper_mode": true,
    "budget": 0,
    "max_concurrent": 1,
    "gate_concentration_pct": 0.5,
    "max_hold_min": 240,
    "exit_reversion_factor": 0.5,
    "bar_persistence": 4,
    "kline_staleness_sec": 90,
    "poll_interval_sec": 30,
    "debug_log": false,
    "candidates": []
  },
  "allocation": {
    "enable_unified_capital": false,
    "total_capital_usdt": 0,
    "risk_profile": "balanced",
    "allocation_lookback_days": 7,
    "allocation_floor_pct": 0.2,
    "allocation_ceiling_pct": 0.8,
    "size_multiplier": 1
  },
  "safety": {
    "enable_loss_limits": false,
    "daily_loss_limit_usdt": 100,
    "weekly_loss_limit_usdt": 300,
    "enable_perp_telegram": false,
    "telegram_cooldown_sec": 300
  },
  "analytics": {
    "enable_analytics": false,
    "analytics_db_path": "data/analytics.db"
  },
  "telegram": {
    "bot_token": "",
    "chat_id": ""
  },
  "ai": {
    "endpoint": "",
    "api_key": "",
    "model": "gpt-5.4",
    "max_tokens": 4096
  }
}
EOF

  chmod 600 "$config_path"
  ok "Created safe dry-run config at $config_path"
  warn "Dashboard password: $dashboard_password"
}

build_bot() {
  step "Build Arb Bot"

  if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null && [ "$ARB_START" != "1" ]; then
    fail "$SERVICE_NAME is active. Building would replace the live binary and may trigger an automatic restart. Rerun with ARB_START=1 for an intentional restart, or stop the service first."
  fi

  export PATH="/usr/local/go/bin:$PATH"
  # This installer provisions Node.js directly. Avoid a stale root nvm install
  # taking precedence inside Makefile's build-frontend target.
  export NVM_DIR="/nonexistent"
  cd "$ARB_DIR"

  info "Installing frontend dependencies from package-lock with npm ci"
  info "Building frontend first, then Go binary"
  make build
  chmod +x "$ARB_DIR/arb"

  ok "Build completed: $ARB_DIR/arb"
}

install_service() {
  step "Install systemd service"

  local path
  path="$(service_path)"
  if [ -f "$path" ] && [ "$ARB_OVERWRITE_SERVICE" != "1" ]; then
    validate_existing_service_alignment
    ok "Existing systemd service preserved: $path"
    warn "Set ARB_OVERWRITE_SERVICE=1 to replace it with the generated unit."
    return
  fi

  cat >"$path" <<EOF
[Unit]
Description=Arb Bot
After=network-online.target redis-server.service
Wants=network-online.target redis-server.service

[Service]
Type=simple
User=root
WorkingDirectory=$ARB_DIR
Environment=CONFIG_FILE=$ARB_DIR/config.json
ExecStart=$ARB_DIR/arb
Restart=on-failure
RestartSec=5
LimitNOFILE=1048576
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
  ok "systemd service installed: $SERVICE_NAME"
}

maybe_start_service() {
  if [ "$ARB_START" != "1" ]; then
    warn "Service was not started. Set ARB_START=1 to start/restart automatically."
    return
  fi

  if [ "$ARB_SKIP_START_VALIDATION" != "1" ]; then
    local configured
    configured="$(configured_exchange_count)"
    if [ "$configured" -lt 2 ]; then
      fail "Not starting $SERVICE_NAME: config has fewer than 2 exchanges with API keys. Fill $ARB_DIR/config.json first, or rerun with ARB_SKIP_START_VALIDATION=1."
    fi
    probe_config_redis
  fi

  step "Start service"
  local restarts_before
  local restarts_after
  restarts_before="$(systemd_nrestarts)"
  systemctl restart "$SERVICE_NAME"
  sleep 8
  restarts_after="$(systemd_nrestarts)"
  if ! systemctl is-active --quiet "$SERVICE_NAME"; then
    warn "$SERVICE_NAME did not stay active. Check: journalctl -u $SERVICE_NAME -n 80 --no-pager"
  elif [ "$restarts_after" -gt "$restarts_before" ]; then
    fail "$SERVICE_NAME restarted $((restarts_after - restarts_before)) time(s) after start; likely crash loop. Check: journalctl -u $SERVICE_NAME -n 80 --no-pager"
  else
    ok "$SERVICE_NAME is active"
  fi
}

systemd_nrestarts() {
  systemctl show "$SERVICE_NAME" -p NRestarts --value 2>/dev/null | awk 'NF {print; found=1} END {if (!found) print 0}'
}

configured_exchange_count() {
  local config_path="$ARB_DIR/config.json"
  if [ ! -f "$config_path" ]; then
    echo 0
    return
  fi
  jq '[.exchanges // {} | to_entries[] | select((.value.enabled // true) != false) | select((.value.api_key // "") != "" and (.value.secret_key // "") != "" and ((.key != "bitget" and .key != "okx") or ((.value.passphrase // "") != "")))] | length' "$config_path" 2>/dev/null || echo 0
}

probe_config_redis() {
  local config_path="$ARB_DIR/config.json"
  local addr
  local pass
  local host
  local port

  addr="$(redis_addr_from_config)"
  pass="$(redis_password_from_existing_config)"
  host="${addr%:*}"
  port="${addr##*:}"
  if [ "$host" = "$port" ]; then
    host="127.0.0.1"
    port="6379"
  fi

  if [ -n "$pass" ]; then
    redis-cli -h "$host" -p "$port" -a "$pass" ping >/dev/null 2>&1 || \
      fail "Redis probe failed using $config_path. Fix redis.addr/password before starting $SERVICE_NAME."
  else
    redis-cli -h "$host" -p "$port" ping >/dev/null 2>&1 || \
      fail "Redis probe failed using $config_path. Fix redis.addr/password before starting $SERVICE_NAME."
  fi
}

print_summary() {
  local ip
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"

  echo
  echo "${GREEN}${BOLD}Arb Bot install completed${NC}"
  echo
  echo "Install dir: $ARB_DIR"
  echo "Service:     $SERVICE_NAME"
  echo "Config:      $ARB_DIR/config.json"
  echo "Dashboard:   http://${ip:-SERVER_IP}:8080"
  echo
  echo "Next steps:"
  echo "  1. Edit config:       nano $ARB_DIR/config.json"
  echo "  2. Start service:     systemctl start $SERVICE_NAME"
  echo "  3. Watch logs:        journalctl -u $SERVICE_NAME -f"
  echo "  4. Restart after cfg: systemctl restart $SERVICE_NAME"
  echo
  echo "Safety defaults:"
  echo "  - dry_run=true"
  echo "  - spot_futures.enabled=false"
  echo "  - price_gap.enabled=false"
  echo "  - service is not auto-started unless ARB_START=1"
  echo "  - running service updates require ARB_START=1"
}

main() {
  step "Preflight"
  require_root
  require_apt
  validate_overrides
  validate_existing_service_alignment
  guard_active_service_update
  ok "OS: $(. /etc/os-release && echo "${PRETTY_NAME:-$ID}")"

  step "Install base packages"
  apt-get update -qq
  apt-get install -y -qq ca-certificates curl git build-essential jq openssl tar gzip redis-tools >/dev/null
  ok "Base packages installed"

  install_go
  install_node
  install_chrome_optional
  clone_or_update_repo
  install_redis
  write_default_config
  build_bot
  install_service
  maybe_start_service
  print_summary
}

main "$@"
