# ms-factory Implementation Guide

## Goal

Define the v0.1 Factory repo structure first.

Priority order:

1. repo structure
2. card taxonomy
3. promotion/review flow
4. dedup/update rules
5. evidence/confidence model

Factory v0.1 should be **card-first**, not report-first.

## v0.1 Structure

```text
factory/
├── README.md
├── docs/
├── schemas/
├── reports/
├── cards/
├── manifests/
└── packs/
```

Optional later:

```text
factory/
└── vocab/
```

Use `vocab/` only if structured fields like `framework`, `hardware`, or
`platform` need deterministic normalization later. Do not overbuild it now.

## Top-Level Folders

### `README.md`

Purpose:

- short repo entrypoint
- points to docs, schemas, cards, packs

### `docs/`

Purpose:

- human-facing explanation
- overview, lifecycle, pack format

Recommended files:

```text
docs/
├── overview.md
├── lifecycle.md
└── pack-format.md
```

### `schemas/`

Purpose:

- machine-readable structure for Factory assets

Recommended files:

```text
schemas/
├── report.schema.yaml
├── known_failure.schema.yaml
├── operator.schema.yaml
├── trick.schema.yaml
└── model.schema.yaml
```

Keep these enums in schema, not vocab:

- `kind`
- `lifecycle.state`
- `confidence.level`

### `reports/`

Purpose:

- evidence-oriented assets
- concrete runs, incidents, benchmark observations

Recommended v0.1 shape:

```text
reports/
└── .gitkeep
```

v0.1 note:

- reports are part of the long-term model
- they are not the main implementation target yet
- `/factory report` can stay deferred
- lifecycle stays in metadata, not report subdirectories

### `cards/`

Purpose:

- stable reusable knowledge

Recommended structure:

```text
cards/
├── known_failures/
├── operators/
├── tricks/
└── models/
```

Example paths:

```text
cards/known_failures/flash_attention_cann_rc3.yaml
cards/operators/matmul.yaml
cards/tricks/mhc.yaml
cards/models/qwen3.yaml
```

Lifecycle should stay in metadata, not directory moves.

Recommended v0.1 states:

- `draft`
- `stable`
- `archived`

### `manifests/`

Purpose:

- machine-readable release metadata

Recommended files:

```text
manifests/
├── index.yaml
└── pack.yaml
```

Use:

- `index.yaml` for channel -> latest version
- `pack.yaml` for one pack's metadata and included cards

### `packs/`

Purpose:

- released snapshots for CLI/tool consumption

Recommended structure:

```text
packs/
└── stable/
    └── v0.1/
```

Packs are distribution output, not the authoring source of truth.

## Common Metadata

### Card example

```yaml
kind: known_failure
id: flash_attention_cann_rc3
lifecycle:
  state: stable
source:
  kind: bootstrap
confidence:
  level: bootstrap
```

### Report example

```yaml
kind: report
id: 2026-03-18-qwen3-ascend-failure-001
lifecycle:
  state: draft
source:
  kind: execution
confidence:
  level: observed
```

## How It Works

Default flow:

```text
reports/ -> cards/ -> manifests/ -> packs/
```

Interpretation:

- `reports/` keeps evidence
- `cards/` keeps reusable knowledge
- `manifests/` describes releases
- `packs/` is what downstream consumers install/use

Query note:

- deterministic code should filter on structured fields such as `kind`
- returned card content is then interpreted by the LLM
- prose fields should not be treated as exact-match query keys

## v0.1 Rules

1. Stable cards are the primary deliverable.
2. Bootstrap seeding is allowed.
3. Bootstrap cards must declare `source.kind: bootstrap`.
4. Keep card paths stable.
5. Do not model lifecycle by moving files between directories.
6. Keep schema enums strict.
7. Keep prose fields flexible for LLM reasoning.

## First Files To Create

```text
factory/
├── README.md
├── docs/overview.md
├── schemas/report.schema.yaml
├── schemas/known_failure.schema.yaml
├── schemas/operator.schema.yaml
├── schemas/trick.schema.yaml
├── schemas/model.schema.yaml
├── reports/.gitkeep
├── cards/known_failures/.gitkeep
├── cards/operators/.gitkeep
├── cards/tricks/.gitkeep
├── cards/models/.gitkeep
├── manifests/index.yaml
└── manifests/pack.yaml
```

Add `vocab/` only when structured normalization becomes necessary.
