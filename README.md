# mscode

AI Infra Agent

## Install

### One-liner (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/vigo999/mindspore-code/main/scripts/install.sh | bash
```

Optional overrides:

```bash
# Force one source instead of auto-probing.
MSCODE_INSTALL_SOURCE=github curl -fsSL https://raw.githubusercontent.com/vigo999/mindspore-code/main/scripts/install.sh | bash
MSCODE_INSTALL_SOURCE=mirror curl -fsSL https://raw.githubusercontent.com/vigo999/mindspore-code/main/scripts/install.sh | bash

# Override the mirror base URL if you host your own Caddy/Nginx mirror.
MSCODE_MIRROR_BASE_URL=http://13.229.44.116/mscode/releases curl -fsSL https://raw.githubusercontent.com/vigo999/mindspore-code/main/scripts/install.sh | bash
```

### Build from source

Requires Go 1.24.2+.

```bash
git clone https://github.com/vigo999/mindspore-code.git
cd mindspore-code
go build -o mscode ./cmd/mscode
./mscode
```

## Quick Start

```bash
# Set your LLM API key
export MSCODE_API_KEY=sk-...

# Run
mscode
```

### Use OpenAI API

```bash
export MSCODE_PROVIDER=openai-completion
export MSCODE_API_KEY=sk-...
export MSCODE_MODEL=gpt-4o-mini
./mscode
```

If you specifically want the Responses API path, use `openai-responses`.

### Use Anthropic API

```bash
export MSCODE_PROVIDER=anthropic
export MSCODE_API_KEY=sk-ant-...
export MSCODE_MODEL=claude-3-5-sonnet
./mscode
```

### Use OpenRouter (OpenAI-compatible third-party routing)

OpenRouter uses an OpenAI-compatible interface, so set provider to `openai-completion`:

```bash
export MSCODE_PROVIDER=openai-completion
export MSCODE_API_KEY=sk-or-...
export MSCODE_BASE_URL=https://openrouter.ai/api/v1
export MSCODE_MODEL=anthropic/claude-3.5-sonnet
./mscode
```
