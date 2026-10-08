<#
.SYNOPSIS
    Prompts Site —— 本地构建 + 上传 + 安装（Windows → Debian 13）

.DESCRIPTION
    1. 构建前端（Vite）到 web/
    2. 交叉编译 Go 单二进制（CGO 关闭，linux/amd64）
    3. scp 到服务器 /tmp
    4. ssh 执行 install.sh（建用户、装单元、起服务、健康检查）

    服务器只有 2 核 967MB，永远不要在服务器上编译。

.EXAMPLE
    .\deploy\deploy.ps1
    .\deploy\deploy.ps1 -SkipFrontend      # 前端没改，只重编后端
#>
param(
    [string]$Server = "149.62.44.144",
    [string]$User = "root",
    [int]$Port = 8080,
    [switch]$SkipFrontend,
    [switch]$SkipBuild,
    [switch]$SkipPreflight
)

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$FrontendDir = Join-Path $Root "frontend"
$BinName = "prompts-server"
$Remote = "$User@$Server"

function Step($msg) { Write-Host "`n=== $msg ===" -ForegroundColor Magenta }
function Ok($msg) { Write-Host "  $msg" -ForegroundColor Green }
function Warn($msg) { Write-Host "  $msg" -ForegroundColor Yellow }

foreach ($cmd in @("ssh", "scp")) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
        throw "找不到 $cmd。请先安装 Windows OpenSSH 客户端（设置 → 应用 → 可选功能）。"
    }
}

# ---------- 1. 前端 ----------
if (-not $SkipFrontend) {
    Step "构建前端 (Vite)"
    if (-not (Test-Path (Join-Path $FrontendDir "node_modules"))) {
        Warn "node_modules 不存在，先执行 npm install"
        Push-Location $FrontendDir
        npm install --no-audit --no-fund
        Pop-Location
    }
    Push-Location $FrontendDir
    try {
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "前端构建失败" }
    } finally { Pop-Location }
    Ok "前端产物已输出到 web/"
}

# ---------- 2. 交叉编译 ----------
if (-not $SkipBuild) {
    Step "交叉编译 linux/amd64"
    $localGo = Join-Path $Root ".tools\go\bin\go.exe"
    $goExe = if (Test-Path $localGo) { $localGo } else { "go" }

    $env:CGO_ENABLED = "0"
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    if (Test-Path (Join-Path $Root ".tools\gocache")) {
        $env:GOROOT = Join-Path $Root ".tools\go"
        $env:GOPATH = Join-Path $Root ".tools\gopath"
        $env:GOMODCACHE = Join-Path $Root ".tools\gopath\pkg\mod"
        $env:GOCACHE = Join-Path $Root ".tools\gocache"
        $env:GOTMPDIR = Join-Path $Root ".tools\tmp"
        if (-not $env:GOPROXY) { $env:GOPROXY = "https://goproxy.cn,direct" }
    }

    Push-Location $Root
    try {
        & $goExe build -trimpath -ldflags "-s -w" -o $BinName .
        if ($LASTEXITCODE -ne 0) { throw "Go 构建失败" }
    } finally { Pop-Location }

    $bin = Join-Path $Root $BinName
    $mb = [math]::Round((Get-Item $bin).Length / 1MB, 2)
    Ok "$BinName  ($mb MB)"

    # 顺手确认它真的是 linux 可执行文件
    $bytes = [System.IO.File]::ReadAllBytes($bin)[0..3]
    if ($bytes[0] -ne 0x7F -or $bytes[1] -ne 0x45 -or $bytes[2] -ne 0x4C -or $bytes[3] -ne 0x46) {
        throw "产物不是 ELF 文件，GOOS/GOARCH 可能没生效"
    }
    Ok "ELF 校验通过"
}

# ---------- 3. 端口预检（上传前先看清服务器现状）----------
if (-not $SkipPreflight) {
    Step "服务器端口预检"
    & scp (Join-Path $Root "deploy\preflight.sh") "$Remote`:/tmp/preflight.sh"
    if ($LASTEXITCODE -ne 0) { throw "scp preflight.sh 失败" }
    & ssh $Remote "sed -i 's/\r$//' /tmp/preflight.sh && bash -n /tmp/preflight.sh && SITE_PORT=$Port bash /tmp/preflight.sh"
    if ($LASTEXITCODE -ne 0) {
        throw "端口预检未通过，已中止部署（没有对服务器做任何修改）"
    }
} else {
    Warn "已跳过端口预检"
}

# ---------- 4. 上传 ----------
Step "上传到 $Remote:/tmp"
$binPath = Join-Path $Root $BinName
& scp $binPath "$Remote`:/tmp/$BinName"
if ($LASTEXITCODE -ne 0) { throw "scp 二进制失败" }
& scp (Join-Path $Root "deploy\prompts.service") "$Remote`:/tmp/prompts.service"
if ($LASTEXITCODE -ne 0) { throw "scp unit 失败" }
& scp (Join-Path $Root "deploy\install.sh") "$Remote`:/tmp/install.sh"
if ($LASTEXITCODE -ne 0) { throw "scp install.sh 失败" }
Ok "上传完成"

# ---------- 5. 安装 ----------
Step "在服务器上安装并重启"
& ssh $Remote "sed -i 's/\r$//' /tmp/install.sh && bash -n /tmp/install.sh && SITE_PORT=$Port bash /tmp/install.sh /tmp/$BinName /tmp/prompts.service"
if ($LASTEXITCODE -ne 0) { throw "远端安装失败" }

Step "部署完成"
Write-Host "  https://prompts.kanbekotori.top" -ForegroundColor Magenta
Write-Host "  查看日志：ssh $Remote 'journalctl -u prompts -f'" -ForegroundColor DarkGray
