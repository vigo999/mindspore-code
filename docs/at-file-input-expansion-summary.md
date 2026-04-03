# `@file` Input Expansion Summary

## Overview

This change adds conservative `@file` prompt expansion support on branch
`refactor-arch-4.11-support-at-file`.

The goal is to let users reference workspace files in prompts without
changing slash command recognition or disturbing structured command parsing.

## Supported Surfaces

`@relative/path` expansion is enabled for:

- Plain chat input
- `/report`
- `/diagnose`
- `/fix`
- `/skill <name> ...`
- Direct skill aliases such as `/pdf ...`

It is intentionally not enabled for structured commands such as `/project`,
`/train`, `/model`, `/permission`, `/login`, `/issues`, `/status`, `/bugs`,
`/claim`, `/close`, or `/dock`.

## Syntax and Safety Rules

Version 1 behavior is intentionally strict:

- Only standalone whitespace-delimited `@relative/path` tokens expand
- `@@name` keeps a literal `@name`
- Paths must remain inside the current workspace
- Absolute paths and escaping paths are rejected
- Directories and missing files are rejected
- Any invalid `@file` reference fails the whole input

Expanded file references are injected into prompts in this form:

```text
[file path="/absolute/workspace/path.txt"]
```

## Implementation Notes

The change is split across three main areas:

1. Shared file validation and file-path resolution

- Added `internal/workspacefile/workspacefile.go`
- Centralizes workspace-relative path validation
- Reused by input expansion

2. Input expansion and raw command parsing

- Added `internal/app/input_expansion.go`
- Keeps slash detection unchanged
- Expands plain chat only after confirming input is not a slash command
- Parses slash commands from raw input first, then expands only approved
  command remainders

3. Command-specific behavior preservation

- `/diagnose` and `/fix` still use the first raw token to decide whether the
  command targets `ISSUE-*`
- In issue mode, only the remainder after the issue key is expanded
- `/skill` and direct skill aliases now preserve the raw request tail so the
  skill name itself is never changed by `@file`

## Documentation Updates

User-facing notes were added to:

- `README.md`
- `/help` output in `internal/app/commands.go`

## Validation

Targeted tests were added in `internal/app/input_expansion_test.go`.

Validated scenarios include:

- Plain chat path expansion
- Multiple `@file` tokens
- `@@` escaping
- Excluded commands remaining unchanged
- `/report`, `/diagnose`, `/fix`, `/skill`, and skill alias behavior
- Issue-target preservation for `/diagnose` and `/fix`
- Failure on missing, unsafe, or directory paths

The targeted verification command that passed was:

```powershell
go test ./internal/app ./tools/fs -run "Test(ExpandInputText|ProcessInput|HandleCommand|CmdIssue|ParseIssueCommandTarget|InterruptTokenCancelsActiveTask)"
```

## Notes

- Some broader Windows-specific tests in the repository already fail for
  unrelated path/session reasons and were not changed as part of this work.
- This implementation is intentionally conservative and leaves punctuation-
  adjacent `@file` forms and paths with spaces for future expansion if needed.
