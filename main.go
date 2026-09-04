package main

import (
	"embed"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"chatty/internal/chat"
	"chatty/internal/config"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails 把 frontend/dist 嵌入二进制作为前端资产。
//
//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// 注册流式聊天事件，绑定生成器会为前端提供强类型订阅 API。
	application.RegisterEvent[ChatDeltaEvent]("chat:delta")
	application.RegisterEvent[ChatDoneEvent]("chat:done")
	application.RegisterEvent[ChatErrorEvent]("chat:error")
	application.RegisterEvent[ChatThinkingEvent]("chat:thinking")
}

func main() {
	dataDir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("resolve user config dir: %v", err)
	}
	dataDir = filepath.Join(dataDir, "Chatty")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("create config dir: %v", err)
	}

	settingsPath := filepath.Join(dataDir, "settings.json")
	cfg, err := config.Load(settingsPath)
	if err != nil {
		log.Fatalf("load settings: %v", err)
	}
	// 首次运行：把默认设置落盘，便于用户直接编辑。
	if len(cfg.Providers) == 0 {
		if err := config.Save(settingsPath, cfg); err != nil {
			log.Printf("save default settings: %v", err)
		}
	}

	store, err := chat.Open(filepath.Join(dataDir, "chatty.db"))
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer store.Close()

	// 调试日志：bin/chatty.log（追加写）。聊天/工具/流式阶段细节都在这里。
	if exePath, err := os.Executable(); err == nil {
		logPath := filepath.Join(filepath.Dir(exePath), "chatty.log")
		if lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			defer lf.Close()
			chatSvc := NewChatService(store, cfg, settingsPath, nil)
			chatSvc.AttachLogger(slog.New(slog.NewTextHandler(lf, &slog.HandlerOptions{Level: slog.LevelInfo})))
			run(chatSvc)
			return
		} else {
			log.Printf("open log file %s: %v", logPath, err)
		}
	}
	run(NewChatService(store, cfg, settingsPath, nil))
}

// run 构造窗口并启动应用。chatSvc 已在调用方创建并注入日志器。
func run(chatSvc *ChatService) {

	app := application.New(application.Options{
		Name:        "Chatty",
		Description: "Typst-first local chatbot",
		Services: []application.Service{
			application.NewService(chatSvc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	// 事件对象挂在 Application 上，而 service 在 Options 中构造；
	// 二者无循环依赖冲突——service 方法在 app.Run() 之后才被调用，
	// 在此之前注入 emitter 是安全的。
	chatSvc.emitter = app.Event

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Chatty",
		Width:  1080,
		Height: 720,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(248, 249, 250),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
