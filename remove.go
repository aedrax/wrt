package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CmdRemove safely tears down a worktree: deinitializes submodules, removes
// the worktree (force if necessary), and attempts to delete the branch if
// it is fully merged.
func (a *App) CmdRemove(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: wrt remove <branch_name>")
		return 1
	}
	branch := args[0]

	root, wtPath, err := a.ResolveWorktree(branch)
	if err != nil {
		errf("%v", err)
		return 1
	}

	if stat, err := os.Stat(wtPath); err != nil || !stat.IsDir() {
		errf("worktree '%s' not found at %s", branch, wtPath)
		return 1
	}
	if !a.IsRegisteredWorktree(wtPath) {
		errf("directory %s is not a registered worktree", wtPath)
		return 1
	}

	// Remember whether the user is inside the worktree being removed so the
	// shell can be moved to the project root afterwards. Resolve symlinks
	// now, while the worktree still exists.
	cwdInside := false
	if cwd, err := os.Getwd(); err == nil {
		cwdInside = isWithin(resolvePath(cwd), resolvePath(wtPath))
	}

	info(fmt.Sprintf("Cleaning up worktree: %s...", branch))

	// 1. Safely de-initialize submodules.
	if _, err := os.Stat(filepath.Join(wtPath, ".git")); err == nil {
		a.RunCmdSilentIn(wtPath, "git", "submodule", "deinit", "--all", "-f")
	}

	// 2. Remove the worktree; force if git blocks it (e.g. submodule state).
	// Run from the project root so removal works even when invoked from
	// inside the worktree being removed.
	if err := a.RunCmdIn(root, "git", "worktree", "remove", wtPath); err != nil {
		warn("Git blocked standard removal. Forcing removal...")
		if err := a.RunCmdIn(root, "git", "worktree", "remove", "--force", wtPath); err != nil {
			errf("failed to force-remove worktree: %v", err)
			return 1
		}
	}
	success("Worktree removed.")

	// 3. Delete the branch if fully merged.
	info("Attempting to delete merged branch...")
	if out, err := a.RunCmdCombinedIn(root, "git", "branch", "-d", branch); err == nil {
		success(fmt.Sprintf("Branch '%s' deleted.", branch))
	} else {
		reason := strings.SplitN(out, "\n", 2)[0]
		if reason == "" {
			reason = err.Error()
		}
		warn(fmt.Sprintf("Branch '%s' kept: %s", branch, reason))
	}

	// 4. Don't leave the shell in a deleted directory.
	if cwdInside {
		if err := a.SetCDTarget(root); err != nil {
			errf("could not set cd target: %v", err)
		}
	}

	return 0
}
