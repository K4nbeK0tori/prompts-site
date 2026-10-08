package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"prompts-site/internal/server"
	"prompts-site/internal/store"
)

const appVersion = "1.0.0"

//go:embed all:web
var webFS embed.FS

//go:embed all:data
var dataFS embed.FS

func defaultDBPath() string {
	if p := strings.TrimSpace(os.Getenv("PROMPTS_DB")); p != "" {
		return p
	}
	if runtime.GOOS == "windows" {
		return filepath.Join("data", "prompts.db")
	}
	return "/opt/prompts/data/prompts.db"
}

func defaultAddr() string {
	if a := strings.TrimSpace(os.Getenv("PROMPTS_ADDR")); a != "" {
		return a
	}
	return "127.0.0.1:8080"
}

func usage() {
	fmt.Fprintf(os.Stderr, `prompts-server %s —— 提示词收藏站

用法:
  prompts-server serve   [-addr 127.0.0.1:8080] [-db 数据库路径] [-seed=true]
  prompts-server passwd  [-db 数据库路径] [-password 新密码]
  prompts-server import  [-db 数据库路径] -file 数据.json [-replace]
  prompts-server export  [-db 数据库路径] [-out 导出.json]
  prompts-server version

环境变量:
  PROMPTS_DB     数据库路径（默认 %s）
  PROMPTS_ADDR   监听地址（默认 127.0.0.1:8080）
`, appVersion, defaultDBPath())
}

func main() {
	log.SetFlags(log.LstdFlags)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	case "passwd", "password":
		cmdPasswd(os.Args[2:])
	case "import":
		cmdImport(os.Args[2:])
	case "export":
		cmdExport(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("prompts-server %s (%s/%s)\n", appVersion, runtime.GOOS, runtime.GOARCH)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func openStore(path string) *store.Store {
	st, err := store.Open(path)
	if err != nil {
		log.Fatalf("打开数据库失败 (%s): %v", path, err)
	}
	return st
}

func seedRecords() []store.ImportRecord {
	raw, err := dataFS.ReadFile("data/prompts.json")
	if err != nil {
		return nil
	}
	var recs []store.ImportRecord
	if err := json.Unmarshal(raw, &recs); err != nil {
		log.Printf("解析内置种子数据失败: %v", err)
		return nil
	}
	return recs
}

func cmdServe(args []string) {
	fl := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fl.String("addr", defaultAddr(), "监听地址")
	dbPath := fl.String("db", defaultDBPath(), "SQLite 数据库路径")
	seed := fl.Bool("seed", true, "数据库为空时导入内置种子数据")
	fl.Parse(args)

	if !strings.HasPrefix(*addr, "127.0.0.1") && !strings.HasPrefix(*addr, "localhost") {
		log.Printf("警告: 监听地址 %s 不是回环地址，请确认已由 nginx 反向代理", *addr)
	}

	st := openStore(*dbPath)
	defer st.Close()

	if *seed {
		n, err := st.SeedIfEmpty(seedRecords())
		if err != nil {
			log.Printf("写入种子数据失败: %v", err)
		} else if n > 0 {
			log.Printf("已导入内置种子数据 %d 条", n)
		}
	}

	if password, generated, err := server.EnsureAdminPassword(st); err != nil {
		log.Fatalf("初始化管理员密码失败: %v", err)
	} else if generated {
		log.Println("==================================================")
		log.Printf("  首次启动，已生成管理员密码: %s", password)
		log.Println("  请立即保存，并尽快用 passwd 子命令改成自己的密码")
		log.Println("==================================================")
	}

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("加载内嵌前端失败: %v", err)
	}
	srv, err := server.New(server.Options{Store: st, Static: sub, Version: appVersion})
	if err != nil {
		log.Fatalf("初始化服务失败: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              *addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("prompts-server %s 监听 http://%s (db=%s)", appVersion, *addr, *dbPath)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("监听失败: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("收到退出信号，正在关闭…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Printf("关闭超时: %v", err)
	}
	log.Println("已退出")
}

func cmdPasswd(args []string) {
	fl := flag.NewFlagSet("passwd", flag.ExitOnError)
	dbPath := fl.String("db", defaultDBPath(), "SQLite 数据库路径")
	password := fl.String("password", "", "新密码（留空则自动生成）")
	fl.Parse(args)

	st := openStore(*dbPath)
	defer st.Close()

	pw := strings.TrimSpace(*password)
	if pw == "" {
		var err error
		pw, err = server.RandomPassword(16)
		if err != nil {
			log.Fatalf("生成密码失败: %v", err)
		}
	}
	if err := server.SetAdminPassword(st, pw); err != nil {
		log.Fatalf("设置密码失败: %v", err)
	}
	fmt.Printf("管理员密码已更新: %s\n", pw)
	fmt.Println("所有旧登录会话已失效。")
}

func cmdImport(args []string) {
	fl := flag.NewFlagSet("import", flag.ExitOnError)
	dbPath := fl.String("db", defaultDBPath(), "SQLite 数据库路径")
	file := fl.String("file", "", "要导入的 JSON 文件")
	replace := fl.Bool("replace", false, "导入前清空已有数据")
	fl.Parse(args)

	if strings.TrimSpace(*file) == "" {
		log.Fatal("必须用 -file 指定 JSON 文件")
	}
	raw, err := os.ReadFile(*file)
	if err != nil {
		log.Fatalf("读取文件失败: %v", err)
	}
	var recs []store.ImportRecord
	if err := json.Unmarshal(raw, &recs); err != nil {
		log.Fatalf("解析 JSON 失败: %v", err)
	}

	st := openStore(*dbPath)
	defer st.Close()

	n, err := st.Import(recs, *replace)
	if err != nil {
		log.Fatalf("导入失败: %v", err)
	}
	total, _ := st.Count()
	log.Printf("导入完成: 新增 %d 条，当前共 %d 条", n, total)
}

func cmdExport(args []string) {
	fl := flag.NewFlagSet("export", flag.ExitOnError)
	dbPath := fl.String("db", defaultDBPath(), "SQLite 数据库路径")
	out := fl.String("out", "", "导出文件路径（留空打印到标准输出）")
	fl.Parse(args)

	st := openStore(*dbPath)
	defer st.Close()

	items, err := st.All()
	if err != nil {
		log.Fatalf("导出失败: %v", err)
	}
	raw, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		log.Fatalf("序列化失败: %v", err)
	}
	if strings.TrimSpace(*out) == "" {
		os.Stdout.Write(raw)
		os.Stdout.Write([]byte("\n"))
		return
	}
	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		log.Fatalf("写文件失败: %v", err)
	}
	log.Printf("已导出 %d 条到 %s", len(items), *out)
}
