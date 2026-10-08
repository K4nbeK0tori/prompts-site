#!/usr/bin/env bash
# ============================================================
#  Prompts Site —— 部署前端口预检（Debian 13 / root 执行）
#
#  目的：在动任何东西之前，先把服务器上真实监听的端口列出来，
#        确认 8080 没被占用，并再次确认 443 / 8443 的状态。
#
#  退出码：0 = 可以部署；1 = 有冲突，已中止
# ============================================================
set -uo pipefail

SITE_PORT="${SITE_PORT:-8080}"

red()  { printf '\033[31m%s\033[0m\n' "$*"; }
grn()  { printf '\033[32m%s\033[0m\n' "$*"; }
ylw()  { printf '\033[33m%s\033[0m\n' "$*"; }
mag()  { printf '\033[35m%s\033[0m\n' "$*"; }

mag "──────────────── 部署前端口预检 ────────────────"

echo
mag "[1/4] 当前全部监听端口"
if command -v ss >/dev/null 2>&1; then
  ss -lntp 2>/dev/null | sed '1s/^/  /'
else
  ylw "  没有 ss 命令，退化用 netstat"
  netstat -lntp 2>/dev/null | grep -i listen | sed 's/^/  /'
fi

# ---------------------------------------------------------------
# 取某个端口对应的进程
# ---------------------------------------------------------------
port_owner() {
  local p="$1"
  ss -lntpH 2>/dev/null | awk -v P=":$p\$" '$4 ~ P {print $0}' | head -n1
}

echo
mag "[2/4] 站点端口 $SITE_PORT 占用检查"
OWNER="$(port_owner "$SITE_PORT")"
if [ -n "$OWNER" ]; then
  # 是我们自己上一次部署留下的进程就允许覆盖，其它一律中止
  if echo "$OWNER" | grep -q 'prompts-server'; then
    grn "  $SITE_PORT 被上一次部署的 prompts-server 占用 —— 属于正常升级，将继续。"
  else
    red  "  ✗ $SITE_PORT 已被其它进程占用："
    echo "    $OWNER"
    red  "  已中止。请换端口（deploy.ps1 -Port <新端口>）或先停掉占用它的服务。"
    exit 1
  fi
else
  grn "  ✓ $SITE_PORT 空闲，可以使用。"
fi

echo
mag "[3/4] 关键端口现状（只读，不做任何修改）"
for p in 80 443 8443 2087; do
  OWNER="$(port_owner "$p")"
  if [ -n "$OWNER" ]; then
    case "$p" in
      443)  echo "  ✓ $p  nginx 持有（正常，站点靠它对外）" ;;
      8443) echo "  ✓ $p  xray 持有（代理命脉，本次不动它）" ;;
      *)    echo "  · $p  已占用：$OWNER" ;;
    esac
  else
    echo "  · $p  空闲"
  fi
done

echo
mag "[4/4] 内存与磁盘余量"
free -m | awk '/Mem:/  {printf "  内存 已用 %s MB / 共 %s MB，可用 %s MB\n", $3, $2, $7}'
free -m | awk '/Swap:/ {printf "  Swap 已用 %s MB / 共 %s MB\n", $3, $2}'
df -h / | awk 'NR==2 {printf "  根分区 已用 %s / 共 %s（剩余 %s）\n", $3, $2, $4}'

echo
grn "──────────────── 预检通过，可以部署 ────────────────"
exit 0
