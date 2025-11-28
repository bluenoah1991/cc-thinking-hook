# CC-ification Hook

This HTTP proxy enables Claude Code CLI to work with OpenAI-compatible API backends. It intercepts Anthropic API requests from Claude Code, converts them to OpenAI chat completions format, and translates responses back to Anthropic format—allowing you to use any OpenAI-compatible model provider (like OpenRouter, Together AI, or local LLMs) as a backend for Claude Code.

## Usage

Build and run the proxy from the project root directory:

```bash
npm run build:ifi:windows  # or build:ifi:unix on Linux/macOS
./cc-ification-hook.exe    # or ./cc-ification-hook on Linux/macOS
# Enter: Backend OpenAI API URL, API Key (optional), Model (optional)
```

Then configure Claude Code to use the proxy:

```bash
export ANTHROPIC_BASE_URL=http://localhost:5281
```

Now use Claude Code CLI normally. The proxy transparently handles format conversion between Anthropic and OpenAI APIs, including streaming responses, tool calls, and extended thinking (converted to reasoning tokens). Place an `ultrathink.txt` file in the working directory to automatically inject custom prompts for enhanced reasoning.
