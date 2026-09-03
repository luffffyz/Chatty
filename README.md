# Chatty

experimental Typst chatbot —— 以 **Typst 为默认文本格式**的本地聊天桌面应用。

- 框架：Wails v3（Go 后端）+ Vue3/Vite（前端）
- 渲染：Typst（wasm，浏览器内编译）+ Mermaid
- LLM：可插拔多 provider（OpenAI-compatible 系）
- 会话：SQLite 本地存储

架构决策与模块划分见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)。

## 开发环境

| 依赖 | 说明 |
|---|---|
| Go 1.25+ | 后端 |
| Node + npm | 前端（Vite） |
| wails3 CLI | `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.16` |
| git 的 `usr/bin` | Windows 下提供 `sh`/`uname` 等（Taskfile 依赖） |
| WebView2 runtime | Windows 10/11 一般自带 |

注意：`wails3` 装在 `$(go env GOPATH)\bin`，Windows 下需保证它在 `PATH` 中；
Taskfile 内部的 `sh:` 步骤需要 `C:\Program Files\Git\usr\bin`（或 scoop 等安装位置）也在 `PATH`。

## 常用命令

```powershell
# 开发（热重载）
wails3 dev

# 构建生产版（输出 bin/chatty.exe）
wails3 build
```

## 参考

架构与实现受 [wailschat](https://github.com/jacksalad/wailschat)（MIT, Copyright 2026 RedJACK）启发。
