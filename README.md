# Image2 MCP

Image2 MCP 是一个给 Codex 使用的本地 STDIO MCP 服务。它提供一个工具
`generate_image2`，用于调用 OpenAI-compatible 的 `gpt-image-2` 生图接口，
把接口返回的 `b64_json` 解码成 PNG 文件并保存到本地。

默认图片网关：

```text
https://api.schyler.top
```

最终调用接口：

```text
{OPENAI_IMAGE_BASE_URL}/v1/images/generations
```

如果 `OPENAI_IMAGE_BASE_URL` 已经以 `/v1` 或 `/v1/images/generations`
结尾，程序会自动避免重复拼接 `/v1`。

## 前置条件

- 已安装 Go
- 已安装 Codex
- 有可用的 `OPENAI_IMAGE_API_KEY`

仓库里只需要提交源码和脚本，不需要提交编译后的 `dist/`，别人拉取后直接运行安装脚本即可。

## 从 GitHub 拉取后安装

### macOS / Linux

```bash
git clone <your-repo-url>
cd image2-mcp
./install.sh --interactive --configure-codex
```

### Windows PowerShell

```powershell
git clone <your-repo-url>
cd image2-mcp
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Interactive -ConfigureCodex
```

安装脚本会做这些事：

1. 交互式输入 `OPENAI_IMAGE_BASE_URL`
2. 交互式输入 `OPENAI_IMAGE_API_KEY`
3. 写入本地 `.env.local`
4. 执行 `go test ./...`
5. 编译 MCP 服务到 `dist/`
6. 写入 Codex MCP 配置

`.env.local`、`dist/`、`output/` 都已加入 `.gitignore`，不要提交到 GitHub。

## 本地配置

交互式安装会生成 `.env.local`：

```dotenv
OPENAI_IMAGE_BASE_URL="https://api.schyler.top"
OPENAI_IMAGE_API_KEY="sk-your-key"
```

Codex 配置里不会直接保存 key。启动 MCP 时，runner 脚本会读取本地
`.env.local`。

## Codex MCP 配置

安装脚本会自动写入 `~/.codex/config.toml`。

macOS / Linux 写入示例：

```toml
[mcp_servers.image2]
command = "/path/to/image2-mcp/scripts/run-image2-mcp.sh"
```

Windows 写入示例：

```toml
[mcp_servers.image2]
command = "powershell.exe"
args = ["-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "C:\\path\\to\\image2-mcp\\scripts\\run-image2-mcp.ps1"]
```

安装完成后，重启 Codex 或开启新会话，让 Codex 重新加载 MCP 配置。

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

真实 smoke test 会调用一次接口，并生成：

```text
output/imagegen/mcp-smoke-test.png
```

## Codex 里怎么调用

MCP 工具名：

```text
generate_image2
```

入参示例：

```json
{
  "prompt": "A realistic studio desk setup",
  "size": "1024x1024",
  "output_dir": "/Users/you/Desktop/images",
  "output_name": "desk.png"
}
```

字段说明：

```text
prompt       必填，生图提示词
size         可选，默认 1024x1024
output_dir   可选，图片保存目录；如果传入，必须是绝对路径
output_name  可选，图片文件名；不传则自动生成 image2-时间戳.png
```

`output_dir` 规则：

- 不传：保存到默认目录 `output/imagegen/`
- 传绝对路径：保存到该目录
- 传相对路径：直接报错，不会调用生图接口

返回示例：

```json
{
  "file_path": "/Users/you/Desktop/images/desk.png",
  "model": "gpt-image-2",
  "size": "1024x1024"
}
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
--interactive / -Interactive        交互式输入 URL 和 key
--configure-codex / -ConfigureCodex 写入 Codex MCP 配置
--base-url / -BaseUrl               指定图片网关地址
--skip-tests / -SkipTests           跳过测试，只构建
--smoke / -Smoke                    调用真实接口做生图测试
```

## 常见问题

### 需要提前打包吗？

不需要。别人从 GitHub 拉取源码后，直接运行安装脚本即可。脚本会在本机编译
`dist/image2-mcp` 或 `dist/image2-mcp.exe`。

### 为什么 key 不写进 Codex config？

为了避免泄漏。Codex config 只保存 runner 路径，真实 key 放在本地
`.env.local`，并且 `.env.local` 不提交。

### Codex 看不到 generate_image2 怎么办？

确认 `~/.codex/config.toml` 里有 `[mcp_servers.image2]`，然后重启 Codex 或开启新会话。

### 生图失败怎么办？

检查：

- `OPENAI_IMAGE_API_KEY` 是否正确
- `OPENAI_IMAGE_BASE_URL` 是否可访问
- `output_dir` 是否是绝对路径
- 目标保存目录是否有写入权限
