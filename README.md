# ms-cli

AI infrastructure agent for MindSpore development.

## Install

### One-liner (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/vigo999/ms-cli/main/scripts/install.sh | bash
```

This downloads the latest release binary to `~/.ms-cli/bin/mscli` and configures your PATH.

### Build from source

Requires Go 1.24.2+.

```bash
git clone https://github.com/vigo999/ms-cli.git
cd ms-cli
go build -o mscli ./cmd/ms-cli
./mscli
```

## Quick Start

```bash
# Set your LLM API key
export MSCLI_API_KEY=sk-...

# Run
mscli
```

### Slash Commands

| Command | Description |
|---------|-------------|
| `/project` | Show project status (overview, milestones, tasks, support) |
| `/project add tasks "title" --owner X --due 03-25` | Add a task (admin) |
| `/project add milestones "title" --progress 50` | Add a milestone (admin) |
| `/project add support "feature name"` | Add a support entry (admin) |
| `/project update <id> --progress 80` | Update a task by ID (admin) |
| `/project update "title" --progress 80` | Update a milestone by title (admin) |
| `/project rm <id\|title>` | Remove an item (admin) |
| `/project overview --phase X --owner Y` | Edit project overview (admin) |
| `/bugs` | List bugs |
| `/report <title>` | Report a bug |
| `/claim <id>` | Claim a bug |
| `/dock` | Show bug dashboard |
| `/login <token>` | Login to the bug/project server |
| `/model <provider:model>` | Switch LLM model |
| `/clear` | Clear chat |

### Server Setup

The bug and project server runs separately:

```bash
# Build and deploy
go build -o /opt/mscli/ms-cli-server ./cmd/ms-cli-server

# Start with config
cd /opt/mscli && ./ms-cli-server -config ./server.yaml
```

See `configs/server.yaml` for server configuration (auth tokens, database, listen address).

## Documentation

- Architecture: [`docs/arch.md`](docs/arch.md)
- Contributor guide: [`docs/agent-contributor-guide.md`](docs/agent-contributor-guide.md)
- Implementation plans: [`docs/impl-guide/`](docs/impl-guide/)
- Feature backlog: [`docs/features-backlog.md`](docs/features-backlog.md)

### Command-Line Options

```bash
# Select URL and model
./ms-cli --url https://api.openai.com/v1 --model gpt-4o

# Set API key directly
./ms-cli --api-key sk-xxx
```

## LLM API Configuration

`ms-cli` supports three provider modes:

- `openai-completion`: OpenAI Chat Completions API and compatible gateways (default)
- `openai-responses`: OpenAI Responses API
- `anthropic`: Anthropic Messages API protocol

Provider routing is fully configuration-driven (no runtime protocol probing).

### Config files

Layered merge (low -> high):

1. built-in defaults
2. user config: `~/.ms-cli/config.yaml`
3. project config: `./.ms-cli/config.yaml`
4. environment variables: `MSCLI_*`
5. session overrides (`/model` in current process only, not persisted)

Each higher layer overrides only the fields it sets.

```yaml
model:
  provider: openai-completion
  url: https://api.openai.com/v1
  model: gpt-4o-mini
  key: ""
```

### Environment variables

Use unified `MSCLI_*` names:

- `MSCLI_PROVIDER`
- `MSCLI_MODEL`
- `MSCLI_API_KEY`
- `MSCLI_BASE_URL`
- `MSCLI_TEMPERATURE`
- `MSCLI_MAX_TOKENS`
- `MSCLI_TIMEOUT`

CLI flags `--api-key`, `--url`, `--model` are startup overrides for the current run.

### Use OpenAI API

```bash
export MSCLI_PROVIDER=openai-completion
export MSCLI_API_KEY=sk-...
export MSCLI_MODEL=gpt-4o-mini
./ms-cli
```

If you specifically want the Responses API path, use `openai-responses`.

### Use Anthropic API

```bash
export MSCLI_PROVIDER=anthropic
export MSCLI_API_KEY=sk-ant-...
export MSCLI_MODEL=claude-3-5-sonnet
./ms-cli
```

### Use OpenRouter (OpenAI-compatible third-party routing)

OpenRouter uses an OpenAI-compatible interface, so set provider to `openai-completion`:

```bash
export MSCLI_PROVIDER=openai-completion
export MSCLI_API_KEY=sk-or-...
export MSCLI_BASE_URL=https://openrouter.ai/api/v1
export MSCLI_MODEL=anthropic/claude-3.5-sonnet
./ms-cli
```

You can also set custom headers in `model.headers` in config when required by a gateway.

### In-session model/provider switch

Inside CLI:

- `/model gpt-4o-mini` (switch model, keep current provider)
- `/model openai-completion:gpt-4o-mini`
- `/model openai-responses:gpt-4o`
- `/model anthropic:claude-3-5-sonnet`

## Known Limitations

- Running Bubble Tea in non-interactive shells may fail with `/dev/tty` errors.
- The bug/project server requires SQLite (CGO enabled).

## Architecture Rule

UI listens to events; agent loop emits events; tool execution does not depend on UI.
