package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
            COMPREPLY=( $(compgen -W "bash zsh fish --install" -- "$cur") )
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
            _arguments '--install[Install shell integration into config]'
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
complete -c wrt -n '__fish_seen_subcommand_from init' -l install -d 'Install shell integration to shell config'
`

func shellIntegrationCode(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashZshFunc + bashCompletions, nil
	case "zsh":
		return bashZshFunc + zshCompletions, nil
	case "fish":
		return fishFunc, nil
	default:
		if runtime.GOOS == "windows" {
			return "", fmt.Errorf("shell integration is not supported on Windows; use bash/zsh/fish via WSL")
		}
		return "", fmt.Errorf("unknown shell %q : supported: bash, zsh, fish", shell)
	}
}

func shellConfigPath(shell string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}

	switch shell {
	case "bash":
		bashrc := filepath.Join(home, ".bashrc")
		bashProfile := filepath.Join(home, ".bash_profile")
		if _, err := os.Stat(bashrc); err == nil {
			return bashrc, nil
		}
		if _, err := os.Stat(bashProfile); err == nil {
			return bashProfile, nil
		}
		return bashrc, nil
	case "zsh":
		return filepath.Join(home, ".zshrc"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), nil
	default:
		if runtime.GOOS == "windows" {
			return "", fmt.Errorf("shell integration is not supported on Windows; use bash/zsh/fish via WSL")
		}
		return "", fmt.Errorf("unknown shell %q : supported: bash, zsh, fish", shell)
	}
}

func shellInstallLine(shell string) (string, error) {
	switch shell {
	case "bash", "zsh":
		return `eval "$(wrt init)"`, nil
	case "fish":
		return `wrt init fish | source`, nil
	default:
		if runtime.GOOS == "windows" {
			return "", fmt.Errorf("shell integration is not supported on Windows; use bash/zsh/fish via WSL")
		}
		return "", fmt.Errorf("unknown shell %q : supported: bash, zsh, fish", shell)
	}
}

func installShellIntegration(shell string) int {
	installLine, err := shellInstallLine(shell)
	if err != nil {
		errf("%v", err)
		return 1
	}

	configPath, err := shellConfigPath(shell)
	if err != nil {
		errf("%v", err)
		return 1
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		errf("failed to create config directory: %v", err)
		return 1
	}

	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		errf("failed to read %s: %v", configPath, err)
		return 1
	}

	if strings.Contains(string(existing), installLine) {
		warn(fmt.Sprintf("wrt shell integration already installed in %s", configPath))
		return 0
	}

	content := string(existing)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += installLine + "\n"

	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		errf("failed to write %s: %v", configPath, err)
		return 1
	}

	success(fmt.Sprintf("Installed wrt shell integration to %s", configPath))
	return 0
}

// CmdInit prints shell integration code (wrapper function + completions).

// CmdInit prints shell integration code that the user can eval in their shell
// profile. Outputs the wrapper function AND tab-completion definitions.
func (a *App) CmdInit(args []string) int {
	shell := ""
	install := false
	for _, arg := range args {
		switch arg {
		case "--install":
			install = true
		default:
			if shell != "" {
				errf("unexpected argument %q", arg)
				return 1
			}
			shell = arg
		}
	}

	// Auto-detect shell from $SHELL if not specified.
	if shell == "" {
		shell = filepath.Base(os.Getenv("SHELL"))
	}

	if install {
		return installShellIntegration(shell)
	}

	code, err := shellIntegrationCode(shell)
	if err != nil {
		errf("%v", err)
		return 1
	}
	fmt.Print(code)
	return 0
}
