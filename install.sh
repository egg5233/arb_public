#!/bin/bash
set -e

# 資金費率套利機器人 — 安裝腳本
# 支援：Ubuntu 22.04+、Debian 12+、WSL2
# 用法：bash install.sh

echo "========================================="
echo "  資金費率套利機器人 — 安裝程式"
echo "========================================="
echo ""

# 顏色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info()  { echo -e "${GREEN}[資訊]${NC} $1"; }
warn()  { echo -e "${YELLOW}[警告]${NC} $1"; }
error() { echo -e "${RED}[錯誤]${NC} $1"; exit 1; }

# 檢查作業系統
if [ ! -f /etc/os-release ]; then
    error "不支援的作業系統。此腳本僅支援 Ubuntu/Debian。"
fi
. /etc/os-release
if [[ "$ID" != "ubuntu" && "$ID" != "debian" ]]; then
    warn "未經測試的作業系統：$ID，繼續執行..."
fi

# 偵測 WSL2
IS_WSL=false
HAS_SYSTEMD=true
if grep -qi microsoft /proc/version 2>/dev/null; then
    IS_WSL=true
    info "偵測到 WSL2 環境"
    # 檢查 systemd 是否可用
    if ! pidof systemd &>/dev/null; then
        HAS_SYSTEMD=false
        warn "WSL2 未啟用 systemd，服務需要手動啟動。"
        warn "如需啟用 systemd，請在 /etc/wsl.conf 加入："
        warn "  [boot]"
        warn "  systemd=true"
        warn "然後重啟 WSL：wsl --shutdown"
    fi
fi

# 檢查是否以 root 執行
if [ "$EUID" -eq 0 ]; then
    error "請勿以 root 身份執行。請使用有 sudo 權限的一般使用者。"
fi

# 如果從 repo 目錄內執行，使用當前目錄；否則預設 ~/arb
if [ -f "$(pwd)/go.mod" ] || [ -f "$(pwd)/Makefile" ]; then
    INSTALL_DIR="${ARB_INSTALL_DIR:-$(pwd)}"
else
    INSTALL_DIR="${ARB_INSTALL_DIR:-$HOME/arb}"
fi
REDIS_PASS="${ARB_REDIS_PASS:-$(openssl rand -hex 16)}"

echo "安裝目錄：$INSTALL_DIR"
echo ""

# ---------------------------------------------------------------
# 1. 系統套件
# ---------------------------------------------------------------
info "安裝系統相依套件..."
sudo apt-get update -qq
sudo apt-get install -y -qq build-essential git curl wget jq unzip > /dev/null 2>&1

# ---------------------------------------------------------------
# 2. Go（若未安裝或版本過舊）
# ---------------------------------------------------------------
GO_VERSION="1.22.12"
NEED_GO=true
if command -v go &>/dev/null; then
    CURRENT_GO=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+')
    if [ "$(printf '%s\n' "1.22" "$CURRENT_GO" | sort -V | head -n1)" = "1.22" ]; then
        info "Go $CURRENT_GO 已安裝（>= 1.22）"
        NEED_GO=false
    fi
fi
if $NEED_GO; then
    info "正在安裝 Go $GO_VERSION..."
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    export PATH="/usr/local/go/bin:$PATH"
    if ! grep -q '/usr/local/go/bin' ~/.bashrc 2>/dev/null; then
        echo 'export PATH="/usr/local/go/bin:$PATH"' >> ~/.bashrc
    fi
    info "Go $(go version | grep -oP 'go\K[0-9.]+') 安裝完成"
fi

# ---------------------------------------------------------------
# 3. Node.js（透過 nvm，用於前端編譯）
# ---------------------------------------------------------------
NODE_VERSION="22"
NEED_NODE=true
if command -v node &>/dev/null; then
    CURRENT_NODE=$(node -v | grep -oP 'v\K[0-9]+')
    if [ "$CURRENT_NODE" -ge "$NODE_VERSION" ] 2>/dev/null; then
        info "Node v$(node -v | tr -d v) 已安裝（>= $NODE_VERSION）"
        NEED_NODE=false
    fi
fi
if $NEED_NODE; then
    info "正在透過 nvm 安裝 Node.js v$NODE_VERSION..."
    if [ ! -d "$HOME/.nvm" ]; then
        curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash > /dev/null 2>&1
    fi
    export NVM_DIR="$HOME/.nvm"
    [ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
    nvm install "$NODE_VERSION" > /dev/null 2>&1
    nvm use "$NODE_VERSION" > /dev/null 2>&1
    info "Node $(node -v) 安裝完成"
fi

# ---------------------------------------------------------------
# 4. Redis
# ---------------------------------------------------------------
if command -v redis-server &>/dev/null; then
    info "Redis 已安裝"
else
    info "正在安裝 Redis..."
    sudo apt-get install -y -qq redis-server > /dev/null 2>&1
    info "Redis 安裝完成"
fi

# 設定 Redis 密碼
if [ -n "$REDIS_PASS" ]; then
    REDIS_CONF="/etc/redis/redis.conf"
    if [ -f "$REDIS_CONF" ]; then
        if ! grep -q "^requirepass" "$REDIS_CONF"; then
            info "設定 Redis 密碼..."
            echo "requirepass $REDIS_PASS" | sudo tee -a "$REDIS_CONF" > /dev/null
            if $HAS_SYSTEMD; then
                sudo systemctl restart redis-server
            else
                sudo redis-server "$REDIS_CONF" --daemonize yes 2>/dev/null || true
            fi
        else
            warn "Redis 已設定密碼，跳過。"
            REDIS_PASS="<既有密碼>"
        fi
    fi
fi
if $HAS_SYSTEMD; then
    sudo systemctl enable redis-server > /dev/null 2>&1
fi

# ---------------------------------------------------------------
# 5. 建立安裝目錄
# ---------------------------------------------------------------
mkdir -p "$INSTALL_DIR"

# ---------------------------------------------------------------
# 6. 產生設定檔範本
# ---------------------------------------------------------------
if [ ! -f "$INSTALL_DIR/config.json" ]; then
    info "建立 config.json 設定檔範本..."
    cat > "$INSTALL_DIR/config.json" << 'CONFIGEOF'
{
  "dry_run": true,
  "exchanges": {
    "binance": {
      "api_key": "",
      "secret_key": "",
      "address": {
        "APT": ""
      }
    },
    "bybit": {
      "api_key": "",
      "secret_key": "",
      "address": {
        "APT": ""
      }
    },
    "gateio": {
      "api_key": "",
      "secret_key": "",
      "address": {
        "APT": ""
      }
    },
    "bitget": {
      "api_key": "",
      "secret_key": "",
      "passphrase": "",
      "address": {
        "APT": ""
      }
    },
    "okx": {
      "api_key": "",
      "secret_key": "",
      "passphrase": "",
      "address": {
        "APT": ""
      }
    },
    "bingx": {
      "api_key": "",
      "secret_key": "",
      "address": {
        "APT": ""
      }
    }
  },
  "redis": {
    "addr": "localhost:6379",
    "password": "REDIS_PASS_PLACEHOLDER",
    "db": 2
  },
  "dashboard": {
    "addr": ":8080",
    "password": "changeme"
  },
  "fund": {
    "max_positions": 5,
    "leverage": 2,
    "capital_per_leg": 50,
    "rebalance_advance_min": 10
  },
  "strategy": {
    "top_opportunities": 25,
    "scan_minutes": [10, 20, 30, 35, 40, 45, 50],
    "entry_scan_minute": 40,
    "exit_scan_minute": 30,
    "rebalance_scan_minute": 10,
    "rotate_scan_minute": 35,
    "discovery": {
      "min_hold_time_hours": 16,
      "max_cost_ratio": 0.50,
      "max_price_gap_bps": 200,
      "price_gap_free_bps": 40,
      "max_gap_recovery_intervals": 1.0,
      "persistence": {
        "funding_window_min": 30,
        "lookback_min_1h": 90,
        "lookback_min_4h": 180,
        "lookback_min_8h": 360,
        "min_count_1h": 2,
        "min_count_4h": 3,
        "min_count_8h": 4,
        "spread_stability_ratio_1h": 0.5,
        "spread_stability_ratio_4h": 0,
        "spread_stability_ratio_8h": 0,
        "spread_stability_oi_rank_1h": 0,
        "spread_stability_oi_rank_4h": 0,
        "spread_stability_oi_rank_8h": 0,
        "spread_volatility_max_cv": 0.5,
        "spread_volatility_min_samples": 10
      }
    },
    "entry": {
      "slippage_limit_bps": 50,
      "min_chunk_usdt": 10,
      "entry_timeout_sec": 300,
      "loss_cooldown_hours": 4.0,
      "re_enter_cooldown_hours": 1
    },
    "exit": {
      "depth_timeout_sec": 300,
      "spread_reversal_tolerance": 0
    },
    "rotation": {
      "threshold_bps": 100,
      "cooldown_min": 60
    }
  },
  "risk": {
    "margin_l3_threshold": 0.50,
    "margin_l4_threshold": 0.80,
    "margin_l5_threshold": 0.95,
    "l4_reduce_fraction": 0.30,
    "risk_monitor_interval_sec": 300
  },
  "ai": {
    "endpoint": "https://codex.egg5233.com/openai-codex-oauth/v1/messages",
    "api_key": "maki_30e89f64534199fa522408ac51cca15f",
    "model": "gpt-5.4",
    "max_tokens": 20000
  }
}
CONFIGEOF
    # 替換 Redis 密碼佔位符
    sed -i "s/REDIS_PASS_PLACEHOLDER/$REDIS_PASS/" "$INSTALL_DIR/config.json"
else
    warn "config.json 已存在，跳過。"
fi

# ---------------------------------------------------------------
# 7. 建立 systemd 服務（或 WSL2 啟動腳本）
# ---------------------------------------------------------------
if $HAS_SYSTEMD; then
    SERVICE_FILE="/etc/systemd/system/arb.service"
    if [ ! -f "$SERVICE_FILE" ]; then
        info "建立 systemd 服務..."
        sudo tee "$SERVICE_FILE" > /dev/null << SERVICEEOF
[Unit]
Description=Funding Rate Arbitrage Bot
After=network.target redis-server.service
Wants=redis-server.service

[Service]
Type=simple
User=$USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/arb
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
SERVICEEOF
        sudo systemctl daemon-reload
        sudo systemctl enable arb > /dev/null 2>&1
        info "systemd 服務已建立並啟用"
    else
        warn "服務檔案已存在，跳過。"
    fi
else
    # WSL2 無 systemd — 建立啟動/停止腳本
    info "建立 WSL2 啟動/停止腳本..."
    cat > "$INSTALL_DIR/start.sh" << STARTEOF
#!/bin/bash
cd "$INSTALL_DIR"
# 啟動 Redis（若未執行）
if ! pgrep -x redis-server > /dev/null; then
    echo "正在啟動 Redis..."
    sudo redis-server /etc/redis/redis.conf --daemonize yes
fi
# 在背景啟動機器人
echo "正在啟動套利機器人..."
nohup ./arb > /dev/null 2>&1 &
echo "機器人已啟動（PID: \$!）"
STARTEOF
    chmod +x "$INSTALL_DIR/start.sh"

    cat > "$INSTALL_DIR/stop.sh" << STOPEOF
#!/bin/bash
echo "正在停止套利機器人..."
pkill -f "$INSTALL_DIR/arb" 2>/dev/null && echo "已停止。" || echo "未在執行中。"
STOPEOF
    chmod +x "$INSTALL_DIR/stop.sh"
    info "已建立 start.sh 和 stop.sh"
fi

# ---------------------------------------------------------------
# 8. 編譯機器人
# ---------------------------------------------------------------
if [ -f "$INSTALL_DIR/go.mod" ]; then
    info "正在編譯機器人..."
    cd "$INSTALL_DIR"
    export PATH="/usr/local/go/bin:$PATH"
    export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"
    [ -s "$NVM_DIR/nvm.sh" ] && source "$NVM_DIR/nvm.sh"

    go mod tidy
    make build
    info "編譯完成"
else
    warn "未找到 go.mod，跳過編譯。請手動執行：cd $INSTALL_DIR && go mod tidy && make build"
fi

# ---------------------------------------------------------------
# 完成
# ---------------------------------------------------------------
echo ""
echo "========================================="
echo -e "${GREEN}  安裝完成！${NC}"
echo "========================================="
echo ""
echo "後續步驟："
echo "  1. 編輯 config.json，填入你的交易所 API 金鑰："
echo "     nano $INSTALL_DIR/config.json"
echo ""
if $HAS_SYSTEMD; then
    echo "  2. 啟動服務："
    echo "     sudo service arb start"
else
    echo "  2. 啟動機器人（WSL2）："
    echo "     cd $INSTALL_DIR && ./start.sh"
    echo "     停止：./stop.sh"
fi
echo ""
echo "  4. 開啟控制面板："
echo "     http://localhost:8080"
echo ""
if [ "$REDIS_PASS" != "<既有密碼>" ]; then
    echo "  Redis 密碼：$REDIS_PASS"
    echo "  （已儲存至 config.json）"
    echo ""
fi
echo "  注意：預設為模擬模式（dry_run）。"
echo "  準備好實盤交易後，請將 config.json 中的 dry_run 改為 false。"
if $IS_WSL && ! $HAS_SYSTEMD; then
    echo ""
    echo "  WSL2 注意：每次 WSL 重啟後，Redis 和機器人需手動啟動。"
    echo "  使用 ./start.sh 可同時啟動兩者。"
fi
echo ""
echo "========================================="
echo "  請設定各交易所 API 金鑰與提領白名單"
echo "========================================="
echo ""
echo "  Bitget"
echo "    API：https://www.bitget.com/zh-TC/account/newapi"
echo "    提領白名單：https://www.bitget.com/zh-TC/asset/address"
echo "    提領免密碼:https://www.bitget.com/zh-TC/account/setting"
echo ""
echo "  Binance"
echo "    API：https://www.binance.com/zh-TC/my/settings/api-management"
echo "    提領白名單：https://www.binance.com/zh-TC/my/security/address-management"
echo ""
echo "  OKX"
echo "    API：https://www.okx.com/zh-hant/account/my-api"
echo "    提領白名單：https://www.okx.com/zh-hant/balance/withdrawal-address/usdt"
echo ""
echo "  Bybit"
echo "    API：https://www.bybit.com/app/user/api-management"
echo "    提領白名單：https://www.bybit.com/zh-TW/user/assets/money-address"
echo ""
echo "  Gate.io"
echo "    API：https://www.gate.com/zh-tw/myaccount/profile/api-key/manage"
echo "    提領白名單：https://www.gate.com/zh-tw/myaccount/funds/withdraw_address"
echo ""
echo "  BingX"
echo "    API：https://bingx.com/zh-tc/account/api"
echo "    提領白名單：https://bingx.com/zh-tc/assets/withdraw/addressManagement?assetId=4&coinName=USDT&coinDisplayName=USDT"
echo ""
