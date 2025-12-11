# Z.ai Official GLM4.6 Usage Guide

CC-ification Hook is an HTTP proxy that enables Claude Code CLI to work with OpenAI-compatible API backends such as Z.ai GLM4.6. It interceptures Anthropic API requests sent by Claude Code, converts them to OpenAI chat completion format, and converts the responses back to Anthropic format.

## How It Works

In actual testing, it was found that Z.ai's official **OpenAI-compatible API** more easily triggers the model's deep thinking capabilities compared to the **Anthropic-compatible API**. Therefore, this project adopts the following strategy:

1. **Protocol Conversion**: Convert Anthropic API requests sent by Claude Code to OpenAI format, then send to Z.ai backend
2. **Wake Word Injection**: Inject specific prompts (wake words) through the `ultrathink.txt` configuration file to guide the model into deep thinking mode
3. **Response Conversion**: Convert the OpenAI format response from Z.ai back to Anthropic format, enabling Claude Code to parse normally

Through the combination of protocol conversion + wake word injection, you can more effectively stimulate GLM4.6's reasoning capabilities and obtain higher-quality code generation and problem-solving answers.

## 1. Environment Preparation

### Install Go Environment

1. Visit [Go Official Download Page](https://golang.org/dl/)
2. Download the installation package suitable for your operating system
3. After installation, verify:
   ```bash
   go version
   ```

### Install Node.js Environment

1. Visit [Node.js Official Website](https://nodejs.org/)
2. Download and install the LTS version
3. Verify installation:
   ```bash
   node --version
   npm --version
   ```

## 2. Clone Repository

```bash
git clone https://github.com/bluenoah1991/cc-thinking-hook.git
cd cc-thinking-hook
```

## 3. Build Program

### Windows System

```bash
# Build as GUI program (no command line window, runs in background)
npm run build:ifi:windows:gui

# Or build as console program
npm run build:ifi:windows
```

> 💡 **Tip**: Recommend using `build:ifi:windows:gui` to build as GUI program. This way the program runs in background without occupying command line window, and you can view logs and manage services through Web Console (`http://localhost:5281`).

### Linux / macOS System

```bash
npm run build:ifi:unix
```

After building, executable files will be generated in the project root:
- Windows: `cc-ification-hook.exe`
- Linux/macOS: `cc-ification-hook-bin`

## 4. Configuration (Optional)

### 4.1 Configure anthropic.json - Get Accurate Token Count

If you want to get accurate token count (rather than estimated values), you can configure Anthropic-compatible interface for precise calculation.

1. Copy example configuration file:
   ```bash
   cp anthropic.json.example anthropic.json
   ```

2. Edit `anthropic.json`:
   ```json
   {
       "url": "https://api.z.ai/api/anthropic",
       "api_key": "your-z-ai-api-key",
       "model": "glm-4.6"
   }
   ```

   | Field | Description |
   |-------|-------------|
   | `url` | Z.ai Anthropic-compatible API address |
   | `api_key` | Your Z.ai API key |
   | `model` | Model for counting |

After configuration, when the program starts it will display `📊 TokenCount: proxy`, indicating use of proxy for precise counting.

> 💡 **Note**: Token count is only used to display accurate usage in Claude Code interface, does not affect auto-compression feature. Even without configuring this, the program works normally.

### 4.2 Configure multimodal.json - Enhance Image Recognition

Since GLM4.6 model's image recognition capability is unstable and not well-adapted for direct image copy-and-paste, you can configure a vision-supporting model to enhance image processing capabilities.

1. Copy example configuration file:
   ```bash
   cp multimodal.json.example multimodal.json
   ```

2. Edit `multimodal.json`:
   ```json
   {
       "url": "https://api.z.ai/api/coding/paas/v4",
       "api_type": "openai",
       "api_key": "your_api_key_here",
       "model": "glm-4.6v",
       "max_rounds": 3,
       "max_tokens": 4096
   }
   ```

   | Field | Description |
   |-------|-------------|
   | `url` | Multimodal API address |
   | `api_type` | API type, supports `openai` and `anthropic` |
   | `api_key` | API key |
   | `model` | Vision model name (e.g., `glm-4.6v`) |
   | `max_rounds` | Maximum conversation rounds (default 3) |
   | `max_tokens` | Maximum output tokens (default 4096) |

After configuration, when the program starts it will display `👁️ Multimodal: enabled`, indicating multimodal enhancement is enabled.

### 4.3 Configure ultrathink.txt - Custom Reasoning Prompts

The project root already has a default `ultrathink.txt` file, which is automatically loaded when the program starts. You can modify its content as needed, such as changing to wake words like "Please think" to guide the model into deep thinking mode.

## 5. Start Program

### Interactive Start

Run the executable file directly, the program will prompt you to enter necessary parameters:

```bash
# Windows
./cc-ification-hook.exe

# Linux/macOS
./cc-ification-hook-bin
```

The program will ask in order:
1. **Backend OpenAI API URL** (Required): Z.ai API address, e.g., `https://api.z.ai/api/coding/paas/v4`
2. **Backend API Key** (Optional): Your Z.ai API key
3. **Backend Model** (Optional): Model name, e.g., `glm-4.6`

> 💡 **Tip**: API Key and Model are recommended not to configure, the program will automatically get them from Claude Code's requests. This way you can directly use the keys and models configured in Claude Code.

### Command Line Parameter Start

You can also start directly through command line parameters:

```bash
./cc-ification-hook.exe -u https://api.z.ai/api/coding/paas/v4 -r 3
```

## 6. Command Line Parameters

| Parameter | Short | Description | Default Value |
|-----------|-------|-------------|---------------|
| `--url` | `-u` | Backend OpenAI API address | (Required) |
| `--key` | `-k` | Backend API key | (Optional) |
| `--model` | `-m` | Backend model name | (Optional) |
| `--port` | `-p` | Proxy service port | 5281 |
| `--scale` | `-s` | Token scaling factor | 1.0 |
| `--round` | `-r` | Keep recent N rounds uncompressed | 0 |
| `--diagnostic` | `-d` | Enable diagnostic mode | false |

## 7. Configure Claude Code

After starting the proxy, you need to configure Claude Code to use this proxy:

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

Then use Claude Code CLI normally, the proxy will automatically handle format conversion between Anthropic and OpenAI APIs.

## 8. Web Console

After the program starts, you can access the Web console through browser:

```
http://localhost:5281
```

The console will display current configuration status and request logs in real-time, convenient for monitoring and debugging.

Click the **Shutdown** button to safely shut down the proxy service.