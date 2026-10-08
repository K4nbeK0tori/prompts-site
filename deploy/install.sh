#!/usr/bin/env bash
# ============================================================
#  Prompts Site —— 服务器侧安装脚本（Debian 13 / root 执行）
#
#  用法（在仓库里直接跑，任意工作目录都行）：
#     bash deploy/install.sh
#     bash deploy/install.sh <二进制路径> <systemd 单元路径>     # 手工指定
#     SITE_PORT=8090 bash deploy/install.sh                     # 换端口
#
#  本脚本只做四件事：建用户、放二进制、装 systemd 单元、起服务。
#  不碰 x-ui、不碰 xray、不碰 nginx 配置，不影响 8443 代理。
#  幂等：重复执行等于升级，数据库在 /opt/prompts/data/ 不受影响。
# ============================================================
set -euo pipefail

# 以脚本自身位置为基准，避免「必须在仓库根目录执行」这个坑
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

BIN_SRC="${1:-$REPO_ROOT/dist/prompts-server-linux-amd64}"
UNIT_SRC="${2:-$SCRIPT_DIR/prompts.service}"
SITE_PORT="${SITE_PORT:-8080}"

APP_DIR=/opt/prompts
DATA_DIR=/opt/prompts/data
BIN_DST=/usr/local/bin/prompts-server
UNIT_DST=/etc/systemd/system/prompts.service
SERVICE=prompts

log() { printf '\033[35m[prompts]\033[0m %s\n' "$*"; }
die() { printf '\033[31m[错误]\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "请用 root 执行"
[ -f "$BIN_SRC" ] || die "找不到二进制：$BIN_SRC（改过代码请先在本地重新交叉编译并推送）"
[ -f "$UNIT_SRC" ] || die "找不到 systemd 单元：$UNIT_SRC"

# ---------- 1. 专用系统用户（无登录 shell、无家目录） ----------
if id -u prompts >/dev/null 2>&1; then
  log "用户 prompts 已存在，跳过创建"
else
  log "创建系统用户 prompts"
  useradd --system --no-create-home --shell /usr/sbin/nologin prompts
fi

# ---------- 2. 目录与二进制 ----------
log "准备目录 $APP_DIR"
install -d -m 0755 "$APP_DIR"
install -d -m 0750 -o prompts -g prompts "$DATA_DIR"

if systemctl is-active --quiet "$SERVICE" 2>/dev/null; then
  log "停止旧服务"
  systemctl stop "$SERVICE"
fi

log "安装二进制 → $BIN_DST"
install -m 0755 -o root -g root "$BIN_SRC" "$BIN_DST"

# ---------- 3. systemd 单元 ----------
log "安装 systemd 单元 → $UNIT_DST"
install -m 0644 -o root -g root "$UNIT_SRC" "$UNIT_DST"

# 端口不是默认值时，改掉单元里的监听地址，避免两处配置不一致
if [ "$SITE_PORT" != "8080" ]; then
  log "把监听端口改写为 $SITE_PORT"
  sed -i "s#-addr 127.0.0.1:8080#-addr 127.0.0.1:$SITE_PORT#" "$UNIT_DST"
fi

systemctl daemon-reload
systemctl enable "$SERVICE" >/dev/null

# ---------- 4. 启动 ----------
FIRST_RUN=0
[ -f "$DATA_DIR/prompts.db" ] || FIRST_RUN=1

log "启动服务"
systemctl restart "$SERVICE"

# 等健康检查通过（最多 15 秒）
for i in $(seq 1 15); do
  if curl -fsS --max-time 2 "http://127.0.0.1:$SITE_PORT/api/health" >/dev/null 2>&1; then
    log "健康检查通过（${i}s）"
    break
  fi
  [ "$i" -eq 15 ] && die "服务 15 秒内未就绪，请查看：journalctl -u $SERVICE -n 50 --no-pager"
  sleep 1
done

# ---------- 5. 汇报 ----------
echo
log "监听状态："
ss -lntp 2>/dev/null | grep -E ":$SITE_PORT" || echo "  （未找到 $SITE_PORT 监听，请检查）"
echo
log "内存占用："
systemctl show "$SERVICE" -p MemoryCurrent --value | awk '{if ($1=="") print "  n/a"; else printf "  %.1f MB\n", $1/1024/1024}'
echo

if [ "$FIRST_RUN" -eq 1 ]; then
  echo "============================================================"
  printf '\033[33m  首次启动 —— 管理员密码在日志里，请立刻记下来：\033[0m\n'
  echo "============================================================"
  journalctl -u "$SERVICE" -n 200 --no-pager | grep -A2 "已生成管理员密码" || true
  echo
  echo "  之后可用这条命令改密码（旧登录会全部失效）："
  echo "    /usr/local/bin/prompts-server passwd -db $DATA_DIR/prompts.db -password '你的新密码'"
  echo
fi

log "完成。别忘了在 nginx 里放好 deploy/nginx-prompts.conf 然后 reload。"
log "站点地址：https://prompts.kanbekotori.top"
