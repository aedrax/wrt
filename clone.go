package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// CmdClone clones a repository as a bare repo into <name>/.git, configured
// for worktree-based development.
func (a *App) CmdClone(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: wrt clone <url> <directory_name>")
		return 1
	}
	url, name := args[0], args[1]

	// The clone runs inside <name>, so a relative local path would resolve
	// against the wrong directory so anchor it to the caller's cwd.
	if _, err := os.Stat(url); err == nil {
		if abs, err := filepath.Abs(url); err == nil {
			url = abs
		}
	}

	if err := os.MkdirAll(name, 0755); err != nil {
		errf("failed to create directory %s: %v", name, err)
		return 1
	}

	// Clone directly into <name>/.git as a bare repo.
	if err := a.RunCmdIn(name, "git", "clone", "--bare", url, ".git"); err != nil {
		errf("git clone --bare failed: %v", err)
		return 1
	}

	// Configure fetch to track branches correctly.
	if err := a.RunCmdIn(name, "git", "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		errf("failed to configure remote fetch: %v", err)
		return 1
	}

	if err := a.RunCmdIn(name, "git", "fetch", "origin"); err != nil {
		errf("git fetch failed: %v", err)
		return 1
	}

	fmt.Fprintln(os.Stderr)
	success(fmt.Sprintf("Cloned bare repository into %s/.git", name))
	info("Ready to spawn worktrees!")

	return 0
}
