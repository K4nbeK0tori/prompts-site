# 焚绝预设 · AI 生图提示词收藏站

个人自用的提示词收集站：**公开可读，只有管理员能改**。
樱花主题对齐 KotoriVPN 面板，跑在同一台 Debian 13 服务器上，**不影响 VPN 服务**。

线上地址：`https://prompts.kanbekotori.top`
后端监听：`127.0.0.1:8080`（仅回环，由 nginx 终结 TLS）

---

## 1. 技术选型（以及为什么）

| 层 | 选型 | 理由 |
|---|---|---|
| 后端 | Go + `net/http` | 单二进制，运行内存约 **25 MB**，服务器上不用装任何运行时 |
| 数据库 | SQLite（`modernc.org/sqlite`） | 纯 Go 实现，`CGO_ENABLED=0`，交叉编译零痛苦 |
| 前端 | Vite + React + Ant Design | 樱花配色直接复用面板的 `useTheme` token |
| 打包 | `//go:embed web` | 前端产物打进二进制，**部署只有一个文件** |
| 部署 | 本地交叉编译 → scp → systemd | 服务器 2 核 967 MB，**永远不在服务器上编译** |

单条提示词平均 539 字，151 条约 200 KB —— SQLite 完全够用，不引入任何外部数据库。

---

## 2. 目录结构

```
prompts-site/
├── main.go                     # 入口 + serve / passwd / import / export 子命令
├── internal/
│   ├── store/                  # SQLite 建表、CRUD、标签聚合、导入导出
│   └── server/                 # 路由、鉴权、限流、静态资源、SPA 兜底
├── frontend/                   # Vite + React + AntD 源码
│   └── src/
│       ├── App.jsx             # 页面骨架、筛选状态、增删改查编排
│       ├── styles.css          # 樱花主题（CSS 变量 + 暗色）
│       └── components/         # 卡片 / 详情 / 表单 / 侧栏 / 樱花层 / 登录框
├── web/                        # 前端构建产物（被 go:embed 打进二进制）
├── data/
│   ├── prompts.json            # 种子数据（151 条，随二进制一起发布）
│   └── prompts.db              # 本地数据库（git 忽略）
├── tools/xlsx_to_json.py       # xlsx → JSON 导入转换脚本
├── dist/
│   └── prompts-server-linux-amd64   # 预编译发布件（服务器直接拉取，不在服务器上编译）
├── deploy/
│   ├── preflight.sh            # 部署前端口预检（只读，有冲突就中止）
│   ├── prompts.service         # systemd 单元
│   ├── install.sh              # 服务器侧安装脚本
│   ├── nginx-prompts.conf      # nginx 站点块（含 gzip）
│   └── deploy.ps1              # 本地一键构建 + 上传 + 安装（scp 方案）
└── .tools/                     # 便携 Go 工具链与构建缓存（git 忽略）
```

> **仓库里没有的东西**：`PROMPTS-SITE-HANDOFF.md`（含服务器 IP、面板默认凭据、
> AccessKey 泄露记录）和提示词原始 `.xlsx` 都在 `.gitignore` 里 —— 本仓库是公开的，
> 这两样只留在本地。

---

## 3. 本地开发

### 依赖

- Node.js ≥ 18（本机 v24）
- Go ≥ 1.24（本机把便携版放在 `.tools/go`，仓库已 gitignore）

### 起服务

```powershell
# 1. 后端（Windows 直接跑）
. .\.tools\goenv.ps1                      # 设置 GOROOT/GOPATH/GOCACHE/GOPROXY
go build -o prompts-server.exe .
.\prompts-server.exe passwd -password "localdev123"
.\prompts-server.exe serve -addr 127.0.0.1:8080
```

```powershell
# 2. 前端热更新（另开一个终端，dev server 会把 /api 代理到 8080）
cd frontend
npm run dev        # http://localhost:5173
```

> 前端改完必须 `npm run build`（产物直接落到 `../web`）**再重新编译 Go**，
> 否则二进制里还是旧的前端。

### 首次启动会做什么

1. 建表（`prompts` / `prompt_tags` / `settings`）
2. 数据库为空时自动导入内置的 `data/prompts.json`（151 条）
3. 没有管理员密码时随机生成一个，**打印到日志里**：

```
==================================================
  首次启动，已生成管理员密码: cNuFzFpK8MgHzAZ5
  请立即保存，并尽快用 passwd 子命令改成自己的密码
==================================================
```

---

## 4. 数据导入

原始数据是 `焚绝预设合集_整理 (1).xlsx`，列：`序号 / 名称 / 标签 / 来源 / 启用 / 提示词 / 更新时间`。

```powershell
pip install openpyxl
python tools\xlsx_to_json.py "焚绝预设合集_整理 (1).xlsx" data\prompts.json

# 导入（-replace 会先清空）
.\prompts-server.exe import -file data\prompts.json -replace
# 导出备份
.\prompts-server.exe export -out backup.json
```

转换脚本会：`\r\n` 统一成 `\n`、标签按 `、，,;；|/` 与空白拆分并去重、
`启用` 的「是/否」转布尔、`更新时间` 统一成 RFC3339。

---

## 5. 部署到服务器

服务器只有 2 核 967 MB，`modernc.org/sqlite` 这种巨型依赖在上面编译又慢又容易 OOM，
所以**发布件是本地预编译好、随仓库一起提交的**：`dist/prompts-server-linux-amd64`。
服务器侧只做「拉代码 → 预检 → 安装」，全程不编译。

### 方案 A：服务器从 GitHub 拉取（推荐）

```bash
# 1. 拉代码
cd /opt
git clone https://github.com/K4nbeK0tori/prompts-site.git
cd prompts-site

# 2. 部署前端口预检（只读，发现冲突会直接中止）
bash deploy/preflight.sh

# 3. 安装并启动
bash deploy/install.sh dist/prompts-server-linux-amd64 deploy/prompts.service
```

以后更新只要：

```bash
cd /opt/prompts-site && git pull
bash deploy/preflight.sh
bash deploy/install.sh dist/prompts-server-linux-amd64 deploy/prompts.service
```

`install.sh` 是幂等的，重复执行等于升级，不会丢数据（数据库在 `/opt/prompts/data/`）。

### 方案 B：本地 scp 直推

```powershell
.\deploy\deploy.ps1
```

依次：构建前端 → 交叉编译 → ELF 校验 → **ssh 端口预检** → scp →
执行 `install.sh`（建用户、装单元、起服务、健康检查、打印首次密码）。
需要输入服务器 root 密码。

### 端口预检做什么

`deploy/preflight.sh` 在动任何东西之前先跑，**只读**：

1. 列出服务器当前全部监听端口
2. 检查站点端口（默认 8080）是否被占用 —— 被非本站进程占用就**直接中止**
3. 确认 443（nginx）与 8443（xray）的现状，不做任何修改
4. 报告内存 / Swap / 磁盘余量

换端口：`SITE_PORT=8090 bash deploy/preflight.sh`，install.sh 会自动改写 systemd 单元里的监听地址。

### nginx

站点块已经存在，但**建议替换成 `deploy/nginx-prompts.conf`**（多了 gzip，
前端主包 764 KB → 245 KB）：

```bash
cp /etc/nginx/conf.d/kotori.conf /root/kotori.conf.bak.$(date +%F)
# 用 deploy/nginx-prompts.conf 里的 server 块替换 prompts.kanbekotori.top 那段
nginx -t && systemctl reload nginx
```

> `443` 由 nginx 独占，新增站点只是多一个 server 块；证书是双域名 SAN，
> 已经覆盖 `prompts.kanbekotori.top`，不用重新签发。

---

## 6. 运维

```bash
systemctl status prompts          # 状态
journalctl -u prompts -f          # 实时日志
systemctl restart prompts         # 重启
```

改密码（**会让所有旧登录立即失效**）：

```bash
/usr/local/bin/prompts-server passwd -db /opt/prompts/data/prompts.db -password '新密码'
```

备份（整个数据目录就两三个文件）：

```bash
sqlite3 /opt/prompts/data/prompts.db ".backup '/root/prompts-$(date +%F).db'"
```

内存保险丝已配好：`MemoryHigh=220M` / `MemoryMax=300M`，`OOMScoreAdjust=500`
（内存吃紧时优先杀本站，保住 Xray）。同时 `GOMAXPROCS=1`，只吃一个核。

---

## 7. 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/health` | 公开 | 健康检查 |
| GET | `/api/me` | 公开 | 当前是否已登录 |
| POST | `/api/login` | 公开 | 登录（**6 次失败锁 15 分钟**） |
| POST | `/api/logout` | 公开 | 退出 |
| GET | `/api/stats` | 公开 | 总数 / 启用 / 停用 / 收藏 / 来源分布 |
| GET | `/api/tags` | 公开 | 标签 + 计数 |
| GET | `/api/prompts` | 公开 | 列表，支持 `q` `tag`(可重复) `source` `status` `favorite` `sort` `page` `size` |
| GET | `/api/prompts/{id}` | 公开 | 详情 |
| POST | `/api/prompts` | 需登录 | 新增 |
| PUT | `/api/prompts/{id}` | 需登录 | 修改 |
| DELETE | `/api/prompts/{id}` | 需登录 | 删除 |
| POST | `/api/prompts/{id}/favorite` | 需登录 | 收藏 / 取消 |
| POST | `/api/prompts/{id}/enabled` | 需登录 | 启用 / 停用 |
| GET | `/api/export` | 需登录 | 导出全部 JSON |

响应统一是 `{"ok":true,"data":…}` 或 `{"ok":false,"error":"…"}`。

已登录时除了 Cookie，也可以直接带 `Authorization: Bearer <token>`，
方便用脚本拉数据。

---

## 8. 安全

- 密码用 **PBKDF2-HMAC-SHA256，12 万次迭代 + 随机盐**，只存哈希
- 会话是 HMAC-SHA256 签名的无状态 Cookie（30 天），
  **改密码后 `auth_epoch` 自增，所有旧会话立刻失效**
- Cookie：`HttpOnly` + `SameSite=Lax`，走 HTTPS 时自动加 `Secure`
- 登录按来源 IP 限流：6 次失败锁 15 分钟
- 服务只绑 `127.0.0.1`，公网访问一律经 nginx
- 容器内沙箱：`NoNewPrivileges` / `ProtectSystem=strict` / `ProtectHome`，
  只允许写 `/opt/prompts/data`
- 请求体上限 8 MB，JSON 字段严格校验（未知字段直接拒绝）
- 图片**只存外链**，服务器磁盘只有 15 G，也不往 SQLite 里塞 BLOB

---

## 9. 服务器铁律（来自交接文档，务必遵守）

1. **`8443` 是代理命脉，任何时候别碰**；新增服务一律绑 `127.0.0.1`
2. **绝对不要点面板里的「更新」按钮** —— 它会拉官方版覆盖樱花皮肤并删除 systemd 单元
3. **不要在服务器上编译**；必须编译时先 `export GOTMPDIR=/opt/gotmp TMPDIR=/opt/gotmp`（`/tmp` 是 484 MB 的 tmpfs）
4. `acme.sh` 不要加 `--keylength ec`（裸 `ec` 不是合法值）
5. 不装 MySQL / PostgreSQL / 宝塔 / 1Panel（会抢 443、重写 iptables，直接搞死 VPN）
6. 面板改设置时 `UPDATE settings` 可能静默失效，用 `DELETE` + `INSERT`

---

## 10. 顺手可做的小事

- [ ] 面板默认密码还是 `admin/admin`，建议尽快改掉
- [ ] 更换曾在聊天里明文出现过的阿里云 AccessKey
- [ ] `/swapfile` 写进 `/etc/fstab`：`echo '/swapfile none swap sw 0 0' >> /etc/fstab`
- [ ] 面板开启 2FA、随机化 `webBasePath`

---

## 11. 常见问题

**Q：前端改了但页面没变？**
`npm run build` 之后必须重新 `go build`，前端是**编译期**嵌进二进制的。

**Q：`database is locked`？**
不会。连接池固定 1 条连接 + WAL + `busy_timeout=5000`，写入天然串行。

**Q：想改站点标题 / 颜色？**
标题在 `frontend/index.html` 与 `frontend/src/App.jsx`；
配色在 `frontend/src/theme.js`（亮/暗）与 `frontend/src/styles.css`（CSS 变量）。

**Q：想加图片上传？**
别。磁盘只有 15 G，且这是个人自用站点 —— 一律用图床外链填 `preview_url`。

**Q：`?theme=dark` 是什么？**
URL 参数可以临时锁定主题，方便分享链接或截图预览。
