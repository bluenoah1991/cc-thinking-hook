# 智谱官方GLM4.6使用方法

CC-ification Hook 是一个HTTP代理，可以让 Claude Code CLI 与 OpenAI 兼容的 API 后端（如智谱GLM4.6）配合工作。它会拦截 Claude Code 发送的 Anthropic API 请求，将其转换为 OpenAI 聊天补全格式，并将响应转换回 Anthropic 格式。

## 工作原理

在实际测试中发现，智谱官方的 **OpenAI 兼容接口**比 **Anthropic 兼容接口**更容易触发模型的深度思考能力。因此，本项目采用以下策略：

1. **协议转换**：将 Claude Code 发出的 Anthropic API 请求转换为 OpenAI 格式，再发送给智谱后端
2. **唤醒词注入**：通过 `ultrathink.txt` 配置文件注入特定的提示词（唤醒词），引导模型进入深度思考模式
3. **响应回转**：将智谱返回的 OpenAI 格式响应转换回 Anthropic 格式，使 Claude Code 能够正常解析

通过协议转换 + 唤醒词注入的组合，可以更有效地激发 GLM4.6 的推理能力，获得更高质量的代码生成和问题解答。

## 1. 环境准备

### 安装 Go 环境

1. 访问 [Go 官方下载页面](https://golang.org/dl/)
2. 下载适合您操作系统的安装包
3. 安装完成后，验证安装：
   ```bash
   go version
   ```

### 安装 Node.js 环境

1. 访问 [Node.js 官方网站](https://nodejs.org/)
2. 下载并安装 LTS 版本
3. 验证安装：
   ```bash
   node --version
   npm --version
   ```

## 2. 克隆代码库

```bash
git clone https://github.com/bluenoah1991/cc-thinking-hook.git
cd cc-thinking-hook
```

## 3. 编译程序

### Windows 系统

```bash
# 编译为 GUI 程序（无命令行窗口，后台运行）
npm run build:ifi:windows:gui

# 或编译为控制台程序
npm run build:ifi:windows
```

> 💡 **提示**：推荐使用 `build:ifi:windows:gui` 编译为 GUI 程序。这样程序会在后台运行，不会占用命令行窗口，您可以通过 Web 控制台 (`http://localhost:5281`) 查看日志和管理服务。

### Linux / macOS 系统

```bash
npm run build:ifi:unix
```

编译完成后，会在项目根目录生成可执行文件：
- Windows: `cc-ification-hook.exe`
- Linux/macOS: `cc-ification-hook-bin`

## 4. 配置（可选）

### 4.1 配置 anthropic.json - 获取准确的 Token 计数

如果您希望获得准确的 token 计数（而非估算值），可以配置 Anthropic 兼容接口进行精确计算。

1. 复制示例配置文件：
   ```bash
   cp anthropic.json.example anthropic.json
   ```

2. 编辑 `anthropic.json`：
   ```json
   {
       "url": "https://open.bigmodel.cn/api/anthropic",
       "api_key": "your-zhipu-api-key",
       "model": "glm-4.6"
   }
   ```

   | 字段 | 说明 |
   |------|------|
   | `url` | 智谱 Anthropic 兼容接口地址 |
   | `api_key` | 您的智谱 API 密钥 |
   | `model` | 用于计数的模型 |

配置后，程序启动时会显示 `📊 TokenCount: proxy`，表示使用代理进行精确计数。

> 💡 **说明**：Token 计数仅用于在 Claude Code 界面显示准确的使用量，不影响自动压缩功能。即使不配置此项，程序也能正常工作。

### 4.2 配置 multimodal.json - 增强图像识别能力

由于 GLM4.6 模型的图像识别能力不稳定，且对于直接复制图片直传适配不好，您可以配置一个支持视觉的模型来增强图像处理能力。

1. 复制示例配置文件：
   ```bash
   cp multimodal.json.example multimodal.json
   ```

2. 编辑 `multimodal.json`：
   ```json
   {
       "url": "https://open.bigmodel.cn/api/coding/paas/v4",
       "api_type": "openai",
       "api_key": "your_api_key_here",
       "model": "glm-4.6v",
       "max_rounds": 3,
       "max_tokens": 4096
   }
   ```

   | 字段 | 说明 |
   |------|------|
   | `url` | 多模态 API 地址 |
   | `api_type` | API 类型，支持 `openai` 和 `anthropic` |
   | `api_key` | API 密钥 |
   | `model` | 视觉模型名称（如 `glm-4.6v`） |
   | `max_rounds` | 最大对话轮数（默认 3） |
   | `max_tokens` | 最大输出 token 数（默认 4096） |

配置后，程序启动时会显示 `👁️ Multimodal: enabled`，表示多模态增强已启用。

### 4.3 配置 ultrathink.txt - 自定义推理提示

项目根目录已默认放置了 `ultrathink.txt` 文件，程序启动时会自动加载。您可以根据需要修改其中的内容，例如改为"（好好想想）"等唤醒词来引导模型进入深度思考模式。

## 5. 启动程序

### 交互式启动

直接运行可执行文件，程序会提示您输入必要的参数：

```bash
# Windows
./cc-ification-hook.exe

# Linux/macOS
./cc-ification-hook-bin
```

程序会依次询问：
1. **Backend OpenAI API URL** (必填): 智谱API地址，如 `https://open.bigmodel.cn/api/coding/paas/v4`
2. **Backend API Key** (可选): 您的智谱API密钥
3. **Backend Model** (可选): 模型名称，如 `glm-4.6`

> 💡 **提示**：API Key 和 Model 推荐不配置，程序会自动从 Claude Code 的请求中获取。这样可以直接使用 Claude Code 中配置的密钥和模型。

### 命令行参数启动

您也可以通过命令行参数直接启动：

```bash
./cc-ification-hook.exe -u https://open.bigmodel.cn/api/coding/paas/v4 -r 3
```

## 6. 命令行参数说明

| 参数 | 简写 | 说明 | 默认值 |
|------|------|------|--------|
| `--url` | `-u` | 后端 OpenAI API 地址 | (必填) |
| `--key` | `-k` | 后端 API 密钥 | (可选) |
| `--model` | `-m` | 后端模型名称 | (可选) |
| `--port` | `-p` | 代理服务端口 | 5281 |
| `--scale` | `-s` | Token 缩放因子 | 1.0 |
| `--round` | `-r` | 保留最近 N 轮不压缩 | 0 |
| `--diagnostic` | `-d` | 启用诊断模式 | false |

## 7. 配置 Claude Code

启动代理后，需要配置 Claude Code 使用此代理：

### Windows (CMD)
```cmd
set ANTHROPIC_BASE_URL=http://localhost:5281
```

### Windows (PowerShell)
```powershell
$env:ANTHROPIC_BASE_URL = "http://localhost:5281"
```

### Linux / macOS
```bash
export ANTHROPIC_BASE_URL=http://localhost:5281
```

然后正常使用 Claude Code CLI 即可，代理会自动处理 Anthropic 和 OpenAI API 之间的格式转换。

## 8. Web 控制台

程序启动后，您可以通过浏览器访问 Web 控制台：

```
http://localhost:5281
```

控制台会实时显示当前配置状态和请求日志，方便监控和调试。

点击 **Shutdown** 按钮可以安全关闭代理服务。