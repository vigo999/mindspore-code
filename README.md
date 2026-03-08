# ms-cli

MindSpore CLI — an AI infrastructure agent with a terminal UI.

## Prerequisites

- Go 1.24.2+ (see `go.mod`)

## Quick Start

Build:

```bash
go build -o ms-cli ./app
```

Run demo mode:

```bash
go run ./app --demo
# or
./ms-cli --demo
```

Run real mode:

```bash
go run ./app
# or
./ms-cli
```

### Command-Line Options

```bash
# Select URL and model
./ms-cli --url https://api.openai.com/v1 --model gpt-4o

# Use custom config file
./ms-cli --config /path/to/config.yaml

# Set API key directly
./ms-cli --api-key sk-xxx
```

## Commands

In TUI input, use slash commands:

### Project Commands
- `/roadmap status [path]` (default: `roadmap.yaml`)
- `/weekly status [path]` (default: `weekly.md`)

### Model Commands
- `/model` - Show current model configuration
- `/model <model-name>` - Switch to a new model
- `/model <openai:model>` - Backward-compatible provider prefix format (e.g., `/model openai:gpt-4o-mini`)

### Session Commands
- `/compact` - Compact conversation context to save tokens
- `/clear` - Clear chat history
- `/mouse [on|off|toggle|status]` - Control mouse wheel scrolling
- `/train [run_id|stop]` - Start/stop distributed training dashboard workflow
- `/exit` - Exit the application
- `/help` - Show available commands

Any non-slash input is treated as a normal task prompt and routed to the engine.

### Slash Command Autocomplete

Type `/` to see available slash commands. Use `↑`/`↓` keys to navigate and `Tab` or `Enter` to select.

## Keybindings

| Key | Action |
|-----|--------|
| `enter` | Send input |
| `mouse wheel` | Scroll chat |
| `pgup` / `pgdn` | Scroll chat |
| `up` / `down` | Scroll chat / Navigate slash suggestions |
| `home` / `end` | Jump to top / bottom |
| `tab` / `enter` | Accept slash suggestion |
| `esc` | Cancel slash suggestions |
| `/` | Start a slash command |
| `ctrl+c` | Quit |

## Distributed Training Dashboard (`/train`)

Run `/train` to enter a dedicated training dashboard UI and execute workflow stages:

1. Push code to multiple hosts via bounded-parallel `rsync`
2. Start remote verification training with `nohup` (defaults to `examples/fake_log_generator.py`, with `run_id`, `log.txt`, `train.pid`)
3. Create SSH master connections (`ControlMaster/ControlPath/ControlPersist`)
4. Start multi-host log streaming via `tail -F`
5. Parse multiple log streams and update dashboard in real time

The sync stage now reuses SSH control sockets, respects `.gitignore` by default, and disables `rsync -z` compression unless you explicitly enable it for a slow WAN link.

Typical generated commands:

```bash
# 1) sync
rsync -a --delete --omit-dir-times --filter ':- .gitignore' --exclude '.git' --exclude '.cache' -e 'ssh -o ControlMaster=auto -o ControlPath=~/.ssh/cm-gpua -o ControlPersist=30m' /local/code/ user@gpuA:/remote/code/
rsync -a --delete --omit-dir-times --filter ':- .gitignore' --exclude '.git' --exclude '.cache' -e 'ssh -o ControlMaster=auto -o ControlPath=~/.ssh/cm-gpub -o ControlPersist=30m' /local/code/ user@gpuB:/remote/code/

# 2) launch
ssh -o ControlMaster=auto -o ControlPath=~/.ssh/cm-gpua -o ControlPersist=30m user@gpuA 'mkdir -p /remote/runs/run_20260306 && cd /remote/code && nohup python -u examples/fake_log_generator.py --run-id run_20260306 --host gpuA --total-steps 120 > /remote/runs/run_20260306/log.txt 2>&1 & echo $! > /remote/runs/run_20260306/train.pid'

# 3) ssh master
ssh -O check -o ControlMaster=auto -o ControlPath=~/.ssh/cm-gpua -o ControlPersist=30m user@gpuA >/dev/null 2>&1 || ssh -MNf -o ControlMaster=yes -o ControlPath=~/.ssh/cm-gpua -o ControlPersist=30m user@gpuA

# 4) log stream
ssh -o ControlMaster=auto -o ControlPath=~/.ssh/cm-gpua -o ControlPersist=30m user@gpuA 'bash -lc '"'"'pid=$(cat /remote/runs/run_20260306/train.pid 2>/dev/null); if [ -n "$pid" ]; then tail --pid="$pid" -n 0 -F /remote/runs/run_20260306/log.txt; else tail -n 0 -F /remote/runs/run_20260306/log.txt; fi'"'"''
```

Dashboard layout:

- Left: per-host key metrics (`model`, `steps`, `throughput`, `loss`, `grad_norm`) and workflow stage state
- Right: multi-host loss curves in different colors with highlighted endpoints, x-axis showing total step range, y-axis auto-scaled by observed loss

## Project Status Data

Roadmap status engine:

- `internal/project/roadmap.go`
- Parses roadmap YAML, validates schema, and computes phase + overall progress.

Weekly update parser (Markdown + YAML front matter):

- `internal/project/weekly.go`
- Template: `docs/updates/WEEKLY_TEMPLATE.md`

Public roadmap page:

- `docs/roadmap/ROADMAP.md`

Project reports:

- `docs/updates/` (see latest `*-report.md`)

## Repository Structure

```text
ms-cli/
├── app/                        # entry point + wiring
│   ├── main.go
│   ├── bootstrap.go
│   ├── wire.go
│   ├── run.go
│   └── commands.go
├── agent/
│   ├── loop/                   # engine, task/event types, permissions
│   ├── context/                # budget, compaction, context manager
│   └── memory/                 # policy, store, retrieve
├── executor/
│   └── runner.go               # pluggable task executor
├── integrations/
│   ├── domain/                 # external domain client + schema
│   └── skills/                 # skill invocation + repo
├── internal/
│   └── project/
│       ├── roadmap.go
│       └── weekly.go
├── tools/
│   ├── fs/                     # filesystem operations
│   └── shell/                  # shell command runner
├── trace/
│   └── writer.go               # execution trace logging
├── report/
│   └── summary.go              # report generation
├── ui/
│   ├── app.go                  # root Bubble Tea model
│   ├── model/model.go          # shared state types
│   ├── components/             # spinner, textinput, viewport
│   └── panels/                 # topbar, chat, hintbar
├── docs/
│   ├── roadmap/ROADMAP.md
│   └── updates/
├── go.mod
└── README.md
```

## Configuration

Configuration can be provided via:

1. **Config file** (`mscli.yaml` or `~/.config/mscli/config.yaml`)
2. **Environment variables**
3. **Command-line flags** (highest priority)

### Environment Variables

| Variable | Description |
|----------|-------------|
| `MSCLI_BASE_URL` | OpenAI-compatible API base URL (higher priority) |
| `MSCLI_MODEL` | Model name |
| `MSCLI_API_KEY` | API key (higher priority) |
| `OPENAI_BASE_URL` | API base URL (fallback) |
| `OPENAI_MODEL` | Model name (fallback) |
| `OPENAI_API_KEY` | API key (fallback) |

### Example Config File

```yaml
model:
  url: https://api.openai.com/v1
  model: gpt-4o-mini
  key: ""
  temperature: 0.7
budget:
  max_tokens: 32768
  max_cost_usd: 10
context:
  max_tokens: 24000
  compaction_threshold: 0.85
training:
  enabled: false
  local_path: .
  startup_command: source ~/.bashrc && conda activate trainer
  remote_code_path: ~/workspace/zhy-ms-cli
  run_base_dir: ~/workspace/train_runs
  hosts_file: train_hosts.yaml
  train_command: python -u examples/fake_log_generator.py --run-id {{RUN_ID}} --host {{HOST_NAME}} --total-steps 120
  ssh_control_persist: 30m
  sync_parallelism: 0
  rsync_compress: false
  rsync_respect_gitignore: true
  exclude: [".git", ".cache", "__pycache__", "mscli-demo.mp4"]
```

Example `train_hosts.yaml`:

```yaml
hosts:
  - name: gpuA
    user: your_user
    address: gpu-a.example.com
    startup_command: source ~/.bashrc && conda activate gpu-train
  - name: gpuB
    user: your_user
    address: gpu-b.example.com
    startup_command: source /usr/local/Ascend/ascend-toolkit/set_env.sh
```

## Known Limitations

- The real-mode engine flow is still minimal/stub-oriented.
- Running Bubble Tea in non-interactive shells may fail with `/dev/tty` errors.

## Architecture Rule

UI listens to events; agent loop emits events; executor/tools do not depend on UI.
