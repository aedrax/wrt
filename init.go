package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Bash / Zsh shell function + completions

const bashZshFunc = `# wrt - Git Worktree Wrapper (shell integration)
# Add this to your ~/.bashrc or ~/.zshrc:
#   eval "$(wrt init)"

wrt() {
    local _wrt_bin
    _wrt_bin=$(command -v wrt) || { echo "wrt: binary not found in PATH" >&2; return 1; }

    local _wrt_cd_file
    _wrt_cd_file=$(mktemp "${XDG_RUNTIME_DIR:-${TMPDIR:-/tmp}}/wrt_cd.XXXXXX")

    command "$_wrt_bin" --cd-file="$_wrt_cd_file" "$@"
    local _wrt_rc=$?

    if [[ -s "$_wrt_cd_file" ]]; then
        local _wrt_target
        _wrt_target=$(cat "$_wrt_cd_file")
        if [[ -d "$_wrt_target" ]]; then
            cd "$_wrt_target" || true
        fi
    fi

    rm -f "$_wrt_cd_file"
    return $_wrt_rc
}
`

const bashCompletions = `
# bash tab-completion
_wrt_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local commands="clone switch remove list init"
    local global_flags="-v --verbose -i --interactive -c --copy-sub -h --help"

    # Walk previous words to find the subcommand.
    local cmd=""
    local i
    for (( i=1; i < COMP_CWORD; i++ )); do
        case "${COMP_WORDS[i]}" in
            -v|--verbose|-i|--interactive|-c|--copy-sub|-h|--help) ;;
            --cd-file|--cd-file=*) ;;
            clone|switch|remove|list|init) cmd="${COMP_WORDS[i]}"; break ;;
        esac
    done

    # No subcommand yet so complete commands and flags.
    if [[ -z "$cmd" ]]; then
        COMPREPLY=( $(compgen -W "$commands $global_flags" -- "$cur") )
        return
    fi

    case "$cmd" in
        switch)
            # Offer flags specific to switch.
            if [[ "$cur" == -* ]]; then
                COMPREPLY=( $(compgen -W "-c --copy-sub $global_flags" -- "$cur") )
                return
            fi
            # Offer all local + remote branches (deduplicated, origin/ stripped).
            local branches
            branches=$(command wrt _branches 2>/dev/null)
            COMPREPLY=( $(compgen -W "$branches" -- "$cur") )
            ;;
        remove)
            # Offer only branches that have an active worktree.
            local worktrees
            worktrees=$(command wrt _worktrees 2>/dev/null)
            COMPREPLY=( $(compgen -W "$worktrees" -- "$cur") )
            ;;
        init)
            COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
            ;;
    esac
}
complete -F _wrt_completions wrt
`

const zshCompletions = `
# zsh tab-completion
_wrt() {
    local -a commands
    commands=(
        'clone:Clone as a bare repo ready for worktrees'
        'switch:Create/switch to a worktree (auto-cd)'
        'remove:Safely remove worktree and delete merged branch'
        'list:List all active worktrees'
        'init:Print shell integration code'
    )

    # Find subcommand in current line.
    local cmd=""
    local w
    for w in "${words[@]:1}"; do
        case "$w" in
            clone|switch|remove|list|init) cmd="$w"; break ;;
        esac
    done

    if [[ -z "$cmd" ]]; then
        _describe 'command' commands
        _arguments \
            '(-v --verbose)'{-v,--verbose}'[Print underlying git commands]' \
            '(-i --interactive)'{-i,--interactive}'[Prompt before running commands]' \
            '(-c --copy-sub)'{-c,--copy-sub}'[Reuse submodule objects from existing worktree]' \
            '(-h --help)'{-h,--help}'[Show help]'
        return
    fi

    case "$cmd" in
        switch)
            _arguments '(-c --copy-sub)'{-c,--copy-sub}'[Reuse submodule objects from existing worktree]'
            local -a branches
            branches=( $(command wrt _branches 2>/dev/null) )
            compadd -a branches
            ;;
        remove)
            local -a worktrees
            worktrees=( $(command wrt _worktrees 2>/dev/null) )
            compadd -a worktrees
            ;;
        init)
            compadd bash zsh fish
            ;;
    esac
}
compdef _wrt wrt
`

// Fish shell function + completions

const fishFunc = `# wrt - Git Worktree Wrapper (shell integration)
# Add this to your ~/.config/fish/config.fish:
#   wrt init fish | source

function wrt
    set -l _wrt_bin (command -v wrt)
    or begin
        echo "wrt: binary not found in PATH" >&2
        return 1
    end

    set -l _wrt_tmpdir $XDG_RUNTIME_DIR
    test -n "$_wrt_tmpdir"; or set _wrt_tmpdir $TMPDIR
    test -n "$_wrt_tmpdir"; or set _wrt_tmpdir /tmp
    set -l _wrt_cd_file (mktemp $_wrt_tmpdir/wrt_cd.XXXXXX)

    command $_wrt_bin --cd-file=$_wrt_cd_file $argv
    set -l _wrt_rc $status

    if test -s $_wrt_cd_file
        set -l _wrt_target (cat $_wrt_cd_file)
        if test -d $_wrt_target
            cd $_wrt_target
        end
    end

    rm -f $_wrt_cd_file
    return $_wrt_rc
end

# fish tab-completion

# Subcommands (only when no subcommand has been given yet).
complete -c wrt -n '__fish_use_subcommand' -a clone   -d 'Clone as a bare repo ready for worktrees'
complete -c wrt -n '__fish_use_subcommand' -a switch  -d 'Create/switch to a worktree (auto-cd)'
complete -c wrt -n '__fish_use_subcommand' -a remove  -d 'Safely remove worktree'
complete -c wrt -n '__fish_use_subcommand' -a list    -d 'List all active worktrees'
complete -c wrt -n '__fish_use_subcommand' -a init    -d 'Print shell integration code'

# Global flags.
complete -c wrt -s v -l verbose     -d 'Print underlying git commands'
complete -c wrt -s i -l interactive -d 'Prompt before running commands'
complete -c wrt -s c -l copy-sub    -d 'Reuse submodule objects from existing worktree'
complete -c wrt -s h -l help        -d 'Show help'

# Branch names for "switch" (local + remote, deduped, origin/ stripped).
complete -c wrt -n '__fish_seen_subcommand_from switch' -f -a '(command wrt _branches 2>/dev/null)'

# Worktree branch names for "remove".
complete -c wrt -n '__fish_seen_subcommand_from remove' -f -a '(command wrt _worktrees 2>/dev/null)'

# Shell names for "init".
complete -c wrt -n '__fish_seen_subcommand_from init' -f -a 'bash zsh fish'
`

// CmdInit prints shell integration code (wrapper function + completions).

// CmdInit prints shell integration code that the user can eval in their shell
// profile. Outputs the wrapper function AND tab-completion definitions.
func (a *App) CmdInit(args []string) int {
	shell := ""
	if len(args) > 0 {
		shell = args[0]
	}

	// Auto-detect shell from $SHELL if not specified.
	if shell == "" {
		shell = filepath.Base(os.Getenv("SHELL"))
	}

	switch shell {
	case "bash":
		fmt.Print(bashZshFunc)
		fmt.Print(bashCompletions)
	case "zsh":
		fmt.Print(bashZshFunc)
		fmt.Print(zshCompletions)
	case "fish":
		fmt.Print(fishFunc)
	default:
		if runtime.GOOS == "windows" {
			errf("shell integration is not supported on Windows; use bash/zsh/fish via WSL")
		} else {
			errf("unknown shell %q : supported: bash, zsh, fish", shell)
		}
		return 1
	}
	return 0
}
