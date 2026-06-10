package main

import (
	"fmt"
	"sort"
	"strings"
)

// CmdList prints all active worktrees.
func (a *App) CmdList(args []string) int {
	if err := a.RunCmd("git", "worktree", "list"); err != nil {
		errf("git worktree list failed: %v", err)
		return 1
	}
	return 0
}

// CmdBranches prints all local and remote branch names (deduplicated, with
// the origin/ prefix stripped), one per line. Hidden subcommand used by the
// shell tab-completions.
func (a *App) CmdBranches(args []string) int {
	out, err := a.RunCmdOutput("git", "branch", "-a", "--format=%(refname:short)")
	if err != nil {
		return 1
	}

	seen := make(map[string]bool)
	var names []string
	for _, line := range strings.Split(out, "\n") {
		name := strings.TrimPrefix(strings.TrimSpace(line), "origin/")
		if name == "" || name == "HEAD" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		fmt.Println(name)
	}
	return 0
}

// CmdWorktrees prints the branch names that have an active worktree, one per
// line. Hidden subcommand used by the shell tab-completions.
func (a *App) CmdWorktrees(args []string) int {
	out, err := a.RunCmdOutput("git", "worktree", "list", "--porcelain")
	if err != nil {
		return 1
	}

	for _, line := range strings.Split(out, "\n") {
		if branch, ok := strings.CutPrefix(line, "branch refs/heads/"); ok {
			fmt.Println(branch)
		}
	}
	return 0
}
