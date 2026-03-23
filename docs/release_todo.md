# Release Plan: Ship by 2026-03-30

## What the demo shows today (all fake/hardcoded in demo.go)

1. `/train qwen3 lora` -> setup checks (local + SSH + remote env)
2. Training runs with log lines + metrics (loss, throughput, lr)
3. Eval on ceval-valid benchmark
4. Failure injection -> crash diagnosis -> fix -> rerun
5. Accuracy drift detection -> diagnosis -> fix -> rerun
6. Performance bottleneck -> diagnosis -> fix -> rerun
7. Algo feature application (MHC) -> rerun with better accuracy

## What's needed for REAL end-to-end: user runs `ms-cli`, trains qwen3 lora on a real Ascend machine

---

## ms-cli -- TODO (runtime, commands, factory store)

### 1. Build `internal/factory/` Store (3 days)
- **Why**: Skills need to query cards during diagnosis. No Go code reads incubating/factory/ cards yet.
- **How**: Store struct with Search(kind, tags), GetCard(id), ListCatalog(kind), Status(). Reads YAML files from cards/ directory. Register as an LLM tool so the agent can call factory_query.

### 2. Wire `/diagnose` and `/fix` commands (2 days)
- **Why**: Users have no way to trigger diagnosis skills. Currently only /skill failure-agent works, which is a developer command, not a user command.
- **How**: Add to handleCommand() in commands.go. Keyword-based symptom classification routes to the right skill. /diagnose loads skill in diagnose-only mode, /fix loads skill in full-cycle mode. Both compose SKILL.md into the task's system prompt.

### 3. Add `/factory` command (1 day)
- **Why**: Users need to check factory status and update packs from terminal.
- **How**: /factory status shows local version, card counts from pack.yaml. /factory update fetches latest pack (v0.1 can just copy from incubating/factory/ to ~/.ms-cli/factory/).

### 4. Real SSH backend for `/train` (3 days)
- **Why**: DemoBackend fakes everything. Need a RealBackend that SSHs to the target, runs the training script, streams logs/metrics back.
- **How**: Implement Backend interface with real SSH execution using runtime/shell/. Setup probes already work with real SSH (sshprobe.Probe). The Run() method needs to: SSH to target, execute training script, parse stdout for metrics (step/loss/throughput), emit Event structs.

### 5. Config for real training targets (1 day)
- **Why**: Users need to configure their target machine (SSH host, address, key path, workdir).
- **How**: Extend configs/types.go with TrainTarget config. Load from ~/.ms-cli/config.yaml. The deleted mscli.yaml had some of this -- check the new config loader pattern from the recent PR.

### 6. Remove orchestrator/planner dead code (0.5 day)
- **Why**: Dead code that confuses contributors. Orchestrator is a passthrough, planner is unused.
- **How**: Delete agent/orchestrator/, agent/planner/, workflow/executor/, demo/scenarios/. Update wire.go imports. Already documented in ms-cli-refactor-3.md Phase A1.

### 7. End-to-end smoke test (1 day)
- **Why**: Validate the full chain works before release.
- **How**: Integration test that wires factory store + skills loader + engine, sends a /fix command, verifies skill loads and factory query tool returns cards.

---

## mindspore-skills -- TODO (skill content, quality)

### 1. Fix AGENTS.md completeness (0.5 day)
- **Why**: setup-agent and accuracy-agent are not listed in Available Skills or Active Skills tables. Users and agents won't discover them.
- **How**: Add both skills to the tables and activation hints section.

### 2. Add factory query instructions to SKILL.md files (2 days)
- **Why**: All 4 skills (failure-agent, accuracy-agent, performance-agent, setup-agent) currently don't mention factory cards at all. They reason from their own reference docs only. The design says skills MUST query factory before generating hypotheses.
- **How**: Add Phase 2 (Factory Query) section to each SKILL.md per the diagnosis-workflow.md design. Add mandatory gate: "You MUST call factory_query before reasoning about root causes."

### 3. Add `/diagnose` and `/fix` mode awareness to SKILL.md (1 day)
- **Why**: Skills don't know whether they're in diagnose-only or fix mode. They'll always try to apply fixes.
- **How**: Add mode-aware instructions: "If the user invoked /diagnose, stop after presenting diagnosis. If the user invoked /fix, proceed to propose fix, get confirmation, apply, and verify."

### 4. Add `setup-agent` command doc (0.5 day)
- **Why**: commands/setup-agent.md doesn't exist but commands/failure-agent.md and commands/performance-agent.md do. Inconsistent.
- **How**: Add commands/setup-agent.md and commands/accuracy-agent.md.

### 5. Validate invoke ratio on real LLM (2 days)
- **Why**: SKILL.md prompt quality determines whether the agent actually follows the workflow or skips steps. Untested skills will fail in production.
- **How**: Run each skill against 5-10 test scenarios using a real LLM. Track step compliance. Iterate on SKILL.md wording until invoke ratio is acceptable. Use existing tests/evals.json files as starting points.

---

## ms-factory (incubating/) -- TODO (cards, pack distribution)

### 1. Add `perf-adam-latency` known_issue card (0.5 day)
- **Why**: The demo has a performance diagnosis story (adam 400ms bottleneck) but there's no known_issue card with symptom: performance for it. Currently all 3 known_issues are failure or accuracy symptoms.
- **How**: Add cards/known_issues/perf-adam-latency.yaml with symptom: performance, detection pattern adam.*400ms|optimizer.*latency, fix pointing to fused-adam.

### 2. Pack distribution mechanism (1 day)
- **Why**: Users need /factory update to actually download cards. Currently cards only exist in the git repo.
- **How**: For v0.1, the simplest path: incubating/factory/ is bundled with the ms-cli binary (embedded via go:embed or copied at install time to ~/.ms-cli/factory/). /factory update re-copies from the embedded source or pulls from a release artifact. Full remote pack fetch can come in v0.2.

### 3. Validate cards against schemas (0.5 day)
- **Why**: No validation that the 28 seeded cards actually conform to their schemas. Broken cards will cause runtime errors.
- **How**: Write a simple Python or Go script that loads each card YAML, checks required fields, validates enum values. Run in CI or as a pre-commit check.

### 4. Add README.md and overview doc (0.5 day)
- **Why**: incubating/factory/ has no README. Contributors won't understand the structure.
- **How**: Short README pointing to schemas, card taxonomy, and the design doc.

---

## Timeline (12 working days: Mar 19-30)

### Week 1 (Mar 19-23):
- **ms-cli**: Factory Store + /factory command + /diagnose + /fix wiring
- **skills**: Fix AGENTS.md + add factory query to SKILL.md files
- **factory**: Add perf known_issue + validate schemas + README

### Week 2 (Mar 24-28):
- **ms-cli**: Real SSH backend + config + remove dead code
- **skills**: Add mode awareness + missing command docs + invoke ratio testing
- **factory**: Pack distribution mechanism

### Mar 29-30:
- **All**: End-to-end smoke test + bug fixes + release prep
