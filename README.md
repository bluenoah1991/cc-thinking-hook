# Claude UltraThink Proxy

This HTTP proxy intercepts requests from Claude Code CLI to Anthropic API and automatically injects "(Please enable ultrathink reasoning.)" into the last user message, triggering Claude's enhanced reasoning mode.

The proxy listens on localhost:5280, receives API requests from Claude Code, finds the last user message in the conversation, and appends the ultrathink instruction. It handles both string and array content formats, only injects when the last message role is "user", and silently ignores other types to preserve request structure. The modified request is forwarded to the backend API and the response is returned transparently.

## Usage

Run `python cc-thinking-hook.py` and enter your backend API URL when prompted. Then configure Claude Code:

```bash
export ANTHROPIC_BASE_URL=http://localhost:5280
```

Now use Claude Code CLI normally. The proxy transparently modifies all requests by adding the ultrathink instruction to your last message, enabling enhanced reasoning mode automatically without any manual intervention.
