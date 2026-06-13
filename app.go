package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// App holds global configuration parsed from CLI flags.
type App struct {
	Verbose     bool
	Interactive bool
	CDFile      string // file path to write cd-target into (set by shell wrapper)
	CopySub     bool   // use local worktree as --reference for submodule init
}

// FindReferenceWorktree returns the path to an existing worktree whose
// project uses submodules (a .gitmodules file exists). Individual submodules
// that are not initialized there are handled per-submodule by the caller.
// Returns ("", nil) if none found.
func (a *App) FindReferenceWorktree(exclude string) (string, error) {
	out, err := a.RunCmdOutput("git", "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := strings.TrimPrefix(line, "worktree ")
		if path == exclude {
			continue
		}
		// Must be a real worktree (has a .git file or dir), not the bare root.
		gitEntry := filepath.Join(path, ".git")
		if info, err := os.Stat(gitEntry); err != nil || info.IsDir() {
			// Bare root has a .git *directory*; worktrees have a .git *file*.
			continue
		}
		// Check for .gitmodules, it indicates the project uses submodules.
		if _, err := os.Stat(filepath.Join(path, ".gitmodules")); err != nil {
			continue
		}
		return path, nil
	}
	return "", nil
}

// RunCmd executes an external command, respecting verbose/interactive modes.
// Stdout and stderr are inherited from the parent process.
func (a *App) RunCmd(name string, args ...string) error {
	return a.RunCmdIn("", name, args...)
}

// RunCmdIn is RunCmd with the working directory set to dir ("" = inherit).
func (a *App) RunCmdIn(dir, name string, args ...string) error {
	if a.Interactive {
		fmt.Fprintf(os.Stderr, "%s ", yellow(fmt.Sprintf("? Run: %s %s", name, strings.Join(args, " "))))
		fmt.Fprint(os.Stderr, "[y/N] ")

		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		ans = strings.TrimSpace(ans)
		if ans != "y" && ans != "Y" {
			fmt.Fprintln(os.Stderr, "  Skipped.")
			return nil
		}
	} else if a.Verbose {
		fmt.Fprintln(os.Stderr, dim(fmt.Sprintf("+ %s %s", name, strings.Join(args, " "))))
	}

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RunCmdOutput executes a command and returns its stdout as a trimmed string.
// It does NOT use verbose/interactive modes (used for internal queries).
func (a *App) RunCmdOutput(name string, args ...string) (string, error) {
	return a.RunCmdOutputIn("", name, args...)
}

// RunCmdOutputIn is RunCmdOutput with the working directory set to dir.
func (a *App) RunCmdOutputIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RunCmdCombinedIn runs a command in dir and returns its combined stdout and
// stderr, trimmed, so the underlying tool's error message can be surfaced.
func (a *App) RunCmdCombinedIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// RunCmdSilent runs a command suppressing all output. Returns the error if any.
func (a *App) RunCmdSilent(name string, args ...string) error {
	return a.RunCmdSilentIn("", name, args...)
}

// RunCmdSilentIn is RunCmdSilent with the working directory set to dir.
func (a *App) RunCmdSilentIn(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.Run()
}

// SetCDTarget tells the shell wrapper to cd into the given path after the
// Go binary exits. If --cd-file was provided, the path is written to that
// file; otherwise the cd command is printed to stderr as a hint for manual
// invocations without the shell wrapper.
func (a *App) SetCDTarget(path string) error {
	if a.CDFile != "" {
		return os.WriteFile(a.CDFile, []byte(path), 0644)
	}
	fmt.Fprintf(os.Stderr, "cd %s\n", path)
	return nil
}

// PromptConfirm asks the user to confirm a destructive operation.
func (a *App) PromptConfirm(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stderr, "%s [y/N] ", prompt)
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(ans)
	return ans == "y" || ans == "Y"
}

// GetRoot finds the project root (the parent of the common .git directory).
// Works from any worktree or the bare repo itself.
func (a *App) GetRoot() (string, error) {
	gitDir, err := a.RunCmdOutput("git", "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}

	abs, err := filepath.Abs(gitDir)
	if err != nil {
		return "", fmt.Errorf("could not resolve git directory: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("could not resolve git directory: %w", err)
	}

	// The project root is the parent of .git.
	if filepath.Base(resolved) == ".git" {
		return filepath.Dir(resolved), nil
	}
	// For bare repos where git-common-dir == the repo root.
	return resolved, nil
}

// ResolveWorktree validates the branch name and returns the project root and
// the path of the branch's worktree under it.
func (a *App) ResolveWorktree(branch string) (root, wtPath string, err error) {
	if err := validateBranchName(branch); err != nil {
		return "", "", err
	}
	root, err = a.GetRoot()
	if err != nil {
		return "", "", err
	}
	return root, filepath.Join(root, branch), nil
}

// validateBranchName rejects names that would escape the project root when
// joined to it (absolute paths, "." / ".." components, backslashes).
func validateBranchName(branch string) error {
	if branch == "" || filepath.IsAbs(branch) || strings.Contains(branch, "\\") {
		return fmt.Errorf("invalid branch name %q", branch)
	}
	for _, part := range strings.Split(branch, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid branch name %q", branch)
		}
	}
	return nil
}

// IsRegisteredWorktree reports whether path is registered in
// `git worktree list`.
func (a *App) IsRegisteredWorktree(path string) bool {
	out, err := a.RunCmdOutput("git", "worktree", "list", "--porcelain")
	if err != nil {
		return false
	}
	target := resolvePath(path)
	for _, line := range strings.Split(out, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok && resolvePath(p) == target {
			return true
		}
	}
	return false
}

// BranchFullyMerged reports whether branch is fully merged into the current
// HEAD branch.
func (a *App) BranchFullyMerged(dir, branch string) (bool, error) {
	out, err := a.RunCmdOutputIn(dir, "git", "branch", "--merged")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(strings.TrimPrefix(line, "* ")) == branch {
			return true, nil
		}
	}
	return false, nil
}

// resolvePath returns the symlink-resolved absolute path, falling back to
// the input on failure.
func resolvePath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// isWithin reports whether path is dir or inside dir. Both paths should
// already be absolute and symlink-resolved.
func isWithin(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
