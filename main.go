package main

import (
	"fmt"
	"os"
)

const version = "v0.1.0"

const helpText = `wrt - Git Worktree Wrapper

Usage: wrt [options] <command> [args...]

Commands:
  clone  <url> <name>    Clone as a bare repo ready for worktrees
  switch <branch>        Create/switch to a worktree (auto-cd)
  remove <branch>        Safely remove worktree, deinit submodules, delete merged branch
  list                   List all active worktrees
  init   [shell]         Print shell integration code (bash, zsh, fish)
  version                Print the installed wrt version

Options:
  -v, --verbose          Print underlying git commands being executed
  -i, --interactive      Prompt for confirmation before running git commands
  -c, --copy-sub         Reuse submodule objects from an existing worktree (no network fetch)
  -V, --version          Print the installed wrt version
  -h, --help             Show this help message

Shell Integration:
  Add to your shell profile for automatic directory switching:

    # bash / zsh
    eval "$(wrt init)"

    # fish
    wrt init fish | source
`

func main() {
	app := &App{}
	var cmd string
	var args []string

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-v", "--verbose":
			app.Verbose = true
		case "-i", "--interactive":
			app.Interactive = true
		case "-c", "--copy-sub":
			app.CopySub = true
		case "-h", "--help":
			cmd = "help"
		case "-V", "--version":
			fmt.Println(version)
			return
		case "--cd-file":
			// --cd-file <path> (two-arg form)
			i++
			if i >= len(os.Args) {
				errf("--cd-file needs a path argument")
				os.Exit(1)
			}
			app.CDFile = os.Args[i]
		default:
			// Handle --cd-file=<path> (single-arg form)
			if len(arg) > 10 && arg[:10] == "--cd-file=" {
				app.CDFile = arg[10:]
				continue
			}
			// First non-flag is the command; rest are args.
			if cmd == "" {
				cmd = arg
			} else {
				args = append(args, arg)
			}
		}
	}

	var rc int
	switch cmd {
	case "clone":
		rc = app.CmdClone(args)
	case "switch":
		rc = app.CmdSwitch(args)
	case "remove":
		rc = app.CmdRemove(args)
	case "list":
		rc = app.CmdList(args)
	case "init":
		rc = app.CmdInit(args)
	case "version":
		fmt.Println(version)
	case "_branches":
		// Hidden: used by the shell tab-completions.
		rc = app.CmdBranches(args)
	case "_worktrees":
		// Hidden: used by the shell tab-completions.
		rc = app.CmdWorktrees(args)
	case "help":
		fmt.Print(helpText)
	default:
		fmt.Print(helpText)
		if cmd != "" {
			fmt.Fprintf(os.Stderr, "\nUnknown command: %s\n", cmd)
			rc = 1
		}
	}

	os.Exit(rc)
}
