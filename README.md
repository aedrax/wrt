# wrt

`wrt` is a small command-line tool for managing Git worktrees. It wraps common worktree operations in a lightweight interface so you can create, switch, list, and remove worktrees without memorizing the full Git commands.

## Why use `wrt`

- Create a new worktree for a branch or commit.
- Switch between existing worktrees quickly.
- List active worktrees in the current repository.
- Remove worktrees cleanly.

## Install

From source:

```bash
cd /data/projects/wrt/main
go install ./...
```

This installs `wrt` to your Go `bin` directory.

## Build

To build the binary locally:

```bash
go build -o wrt .
```

That produces an executable named `wrt` in the current directory.

## Usage

After installing or building, run `wrt` from a Git repository root.

```bash
wrt [command] [options]
```

Typical commands include:

- `clone` - create a new worktree
- `switch` - change to an existing worktree
- `list` - show current worktrees
- `remove` - delete a worktree

For Example:

```
 wrt list                
/data/projects/wrt                (bare)
/data/projects/wrt/main           a8cbd32 [main]
/data/projects/wrt/update-readme  0d2e204 [update-readme]
```

## Notes

This tool is intended to simplify worktree workflows without replacing Git. It is best used inside repositories where you already use Git worktrees.
