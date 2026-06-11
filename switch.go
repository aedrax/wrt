package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CmdSwitch creates (or re-uses) a worktree for the given branch and signals
// the shell wrapper to cd into it.
func (a *App) CmdSwitch(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: wrt switch <branch_name>")
		return 1
	}
	branch := args[0]

	_, wtPath, err := a.ResolveWorktree(branch)
	if err != nil {
		errf("%v", err)
		return 1
	}

	if stat, err := os.Stat(wtPath); err == nil && stat.IsDir() {
		if !a.IsRegisteredWorktree(wtPath) {
			errf("directory %s exists but is not a registered worktree", wtPath)
			return 1
		}
		success("Worktree already exists.")
	} else {
		// git worktree add creates the branch from its remote tracking branch
		// when one exists; for unknown branches, create from HEAD instead.
		if a.branchExists(branch) {
			if err := a.RunCmd("git", "worktree", "add", wtPath, branch); err != nil {
				errf("failed to create worktree: %v", err)
				return 1
			}
		} else {
			warn("Branch not found locally or on a remote, creating new branch from HEAD...")
			if err := a.RunCmd("git", "worktree", "add", "-b", branch, wtPath); err != nil {
				errf("failed to create worktree: %v", err)
				return 1
			}
		}

		// Auto-initialize submodules in the new worktree.
		if a.CopySub {
			a.initSubmodulesFromLocal(wtPath)
		} else {
			a.updateSubmodules(wtPath)
		}

		success(fmt.Sprintf("Worktree created at %s", wtPath))
	}

	// Signal the shell wrapper to cd into the worktree.
	if err := a.SetCDTarget(wtPath); err != nil {
		errf("could not set cd target: %v", err)
	}

	return 0
}

// branchExists reports whether the branch exists locally or on any remote.
func (a *App) branchExists(branch string) bool {
	out, err := a.RunCmdOutput("git", "branch", "-a", "--list", branch, "*/"+branch)
	return err == nil && out != ""
}

// updateSubmodules fetches and checks out all submodules in the worktree.
func (a *App) updateSubmodules(dir string) {
	a.RunCmdIn(dir, "git", "submodule", "update", "--init", "--recursive")
}

// initSubmodulesFromLocal initializes submodules by temporarily pointing their
// URLs at the local git dirs from an existing worktree, avoiding network
// fetches entirely. Falls back to normal remote fetch if no reference
// worktree is available.
func (a *App) initSubmodulesFromLocal(newWt string) {
	// Check for .gitmodules, no file means no submodules.
	if _, err := os.Stat(filepath.Join(newWt, ".gitmodules")); err != nil {
		return
	}

	sourceWt, err := a.FindReferenceWorktree(newWt)
	if err != nil || sourceWt == "" {
		warn("No existing worktree with submodules found; falling back to network fetch")
		a.updateSubmodules(newWt)
		return
	}
	info(fmt.Sprintf("Copying submodule objects from %s (no network fetch)", sourceWt))

	// Parse .gitmodules for submodule names and paths.
	subs := a.parseGitmodules(newWt)
	if len(subs) == 0 {
		a.updateSubmodules(newWt)
		return
	}

	// Register submodules (copies URLs from .gitmodules into git config).
	a.RunCmdSilentIn(newWt, "git", "submodule", "init")

	// For each submodule, override the URL to point at the local git dir
	// from the source worktree so `git submodule update` clones locally.
	redirected := 0
	for _, sub := range subs {
		localGitDir := a.resolveSubmoduleGitDir(sourceWt, sub.path)
		if localGitDir == "" {
			warn(fmt.Sprintf("  %s: not initialized in source, will fetch from network", sub.name))
			continue
		}
		a.RunCmdSilentIn(newWt, "git", "config", fmt.Sprintf("submodule.%s.url", sub.name), localGitDir)
		if a.Verbose {
			fmt.Fprintln(os.Stderr, dim(fmt.Sprintf("  %s -> %s", sub.name, localGitDir)))
		}
		redirected++
	}

	if redirected > 0 {
		info(fmt.Sprintf("Redirected %d submodule(s) to local sources", redirected))
	}

	// Clone from the (now local) URLs. protocol.file.allow is needed because
	// the URL points to a local gitdir path.
	a.RunCmdIn(newWt, "git", "-c", "protocol.file.allow=always", "submodule", "update")

	// Reset all URLs back to their proper remote values from .gitmodules.
	a.RunCmdIn(newWt, "git", "submodule", "sync", "--recursive")

	// Handle any nested submodules (these will fetch from network).
	a.RunCmdSilentIn(newWt, "git", "submodule", "update", "--init", "--recursive")
}

// submodule holds a parsed entry from .gitmodules.
type submodule struct {
	name string // key in .gitmodules (e.g. "libs/mylib")
	path string // filesystem path relative to repo root
}

// parseGitmodules reads .gitmodules and returns the list of submodules.
func (a *App) parseGitmodules(wtPath string) []submodule {
	gitmodules := filepath.Join(wtPath, ".gitmodules")
	out, err := a.RunCmdOutput("git", "config", "-f", gitmodules, "--get-regexp", `submodule\..*\.path`)
	if err != nil {
		return nil
	}

	var subs []submodule
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "submodule.NAME.path PATH"
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]  // submodule.NAME.path
		path := parts[1] // PATH

		name := strings.TrimPrefix(key, "submodule.")
		name = strings.TrimSuffix(name, ".path")
		subs = append(subs, submodule{name: name, path: path})
	}
	return subs
}

// resolveSubmoduleGitDir finds the actual git object directory for a submodule
// in the given worktree. Returns "" if the submodule isn't initialized there.
func (a *App) resolveSubmoduleGitDir(wtPath, subPath string) string {
	gitFile := filepath.Join(wtPath, subPath, ".git")
	content, err := os.ReadFile(gitFile)
	if err != nil {
		return ""
	}

	// The .git file contains "gitdir: <path>"
	line := strings.TrimSpace(string(content))
	if !strings.HasPrefix(line, "gitdir: ") {
		return ""
	}
	gitdir := strings.TrimPrefix(line, "gitdir: ")

	// Resolve relative paths against the submodule's working directory.
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(wtPath, subPath, gitdir)
	}
	gitdir, err = filepath.Abs(gitdir)
	if err != nil {
		return ""
	}

	// Verify it actually exists.
	if _, err := os.Stat(gitdir); err != nil {
		return ""
	}
	return gitdir
}
