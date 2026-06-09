# Image2 MCP

这是一个给 Codex 使用的本地 STDIO MCP 服务。它暴露一个工具：
`generate_image2`，用于调用 OpenAI-compatible 的 `gpt-image-2` 生图接口，
并把返回的 PNG 图片保存到本地 `output/imagegen/` 目录。

默认使用的图片网关是：

```text
https://api.schyler.top
```

实际请求接口会自动拼成：

```text
https://api.schyler.top/v1/images/generations
```

如果你配置的 `OPENAI_IMAGE_BASE_URL` 已经带了 `/v1` 或
`/v1/images/generations`，程序会自动避免重复拼接 `/v1`。

## 一键安装

### macOS / Linux

在项目目录执行：

```bash
cd /Library/idea/ai-go/image2-mcp
./install.sh --interactive --configure-codex
```

脚本会依次完成：

1. 交互式询问 `OPENAI_IMAGE_BASE_URL`
2. 交互式询问 `OPENAI_IMAGE_API_KEY`
3. 写入本地 `.env.local`
4. 执行 `go test ./...`
5. 构建 `dist/image2-mcp`
6. 给 Codex 写入 MCP 配置

### Windows PowerShell

在项目目录执行：

```powershell
cd C:\path\to\image2-mcp
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Interactive -ConfigureCodex
```

Windows 脚本会构建：

```text
dist\image2-mcp.exe
```

并把 Codex 配置成通过 PowerShell runner 启动：

```text
scripts\run-image2-mcp.ps1
```

## 本地配置文件

交互式安装会生成：

```text
.env.local
```

内容类似：

```dotenv
OPENAI_IMAGE_BASE_URL="https://api.schyler.top"
OPENAI_IMAGE_API_KEY="sk-your-key"
```

`.env.local` 已经加入 `.gitignore`，不要提交到 GitHub。

如果你不想用交互式安装，也可以自己设置环境变量：

```bash
export OPENAI_IMAGE_BASE_URL='https://api.schyler.top'
export OPENAI_IMAGE_API_KEY='sk-your-key'
```

Windows PowerShell：

```powershell
$env:OPENAI_IMAGE_BASE_URL = "https://api.schyler.top"
$env:OPENAI_IMAGE_API_KEY = "sk-your-key"
```

## Codex MCP 配置

一键脚本会自动写入 Codex 配置。配置文件通常是：

```text
~/.codex/config.toml
```

macOS / Linux 写入内容类似：

```toml
[mcp_servers.image2]
command = "/Library/idea/ai-go/image2-mcp/scripts/run-image2-mcp.sh"
```

Windows 写入内容类似：

```toml
[mcp_servers.image2]
command = "powershell.exe"
args = ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "C:\\path\\to\\image2-mcp\\scripts\\run-image2-mcp.ps1"]
```

注意：Codex 配置里不直接写 key。runner 启动时会读取本地 `.env.local`。

## 手动构建

macOS / Linux：

```bash
go build -o ./dist/image2-mcp ./cmd/image2-mcp
```

Windows：

```powershell
go build -o .\dist\image2-mcp.exe .\cmd\image2-mcp
```

## 真实生图测试

macOS / Linux：

```bash
./install.sh --interactive --configure-codex --smoke
```

Windows：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Interactive -ConfigureCodex -Smoke
```

真实 smoke test 会调用一次 `gpt-image-2`，生成图片会保存到：

```text
output/imagegen/mcp-smoke-test.png
```

## Codex 里怎么用

安装完成后，重启 Codex 或开启新会话，让 Codex 重新加载 MCP 配置。

之后可以直接让 Codex 调用：

```text
generate_image2
```

工具入参：

```json
{
  "prompt": "A realistic studio desk setup",
  "size": "1024x1024",
  "output_dir": "/tmp/image-output",
  "output_name": "desk.png"
}
```

`output_dir` 可选，但如果传入，必须是绝对路径：

- 不传：保存到默认目录 `output/imagegen/`
- 传绝对路径：直接保存到这个目录
- 传相对路径：直接报错，不会生成图片

返回结果：

```json
{
  "file_path": "/Library/idea/ai-go/image2-mcp/output/imagegen/desk.png",
  "model": "gpt-image-2",
  "size": "1024x1024"
}
```

生成图片默认保存在：

```text
output/imagegen/
```

## 脚本参数

macOS / Linux：

```bash
./install.sh --help
```

Windows：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Help
```

常用参数：

```text
--interactive / -Interactive      交互式输入 URL 和 key
--configure-codex / -ConfigureCodex
                                   写入 Codex MCP 配置
--base-url / -BaseUrl             指定图片网关地址
--skip-tests / -SkipTests         跳过测试，只构建
--smoke / -Smoke                  调用真实接口做生图测试
```

## 注意事项

- 不要把 `.env.local` 提交到 GitHub。
- 不要把真实 `OPENAI_IMAGE_API_KEY` 写进 README 或公开配置。
- 如果 Codex 里看不到 `generate_image2`，先确认 `~/.codex/config.toml`
  里有 `mcp_servers.image2`，然后重启 Codex 或开启新会话。
- 如果真实 smoke test 失败，先检查 key、base URL 和网络连通性。
