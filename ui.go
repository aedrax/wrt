package main

import (
	"fmt"
	"os"
)

// ANSI color helpers for terminal output.
//
// Colors are disabled when NO_COLOR is set (https://no-color.org), when
// TERM=dumb, or when stderr is not a terminal. All colored output goes to
// stderr, so stderr is the stream that is checked.
var colorEnabled = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stderr.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}()

func colorize(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + "\033[0m"
}

func green(s string) string  { return colorize("\033[1;32m", s) }
func yellow(s string) string { return colorize("\033[1;33m", s) }
func dim(s string) string    { return colorize("\033[2m", s) }

// Status messages go to stderr so stdout stays clean for shell-evaluable
// output (e.g. `eval "$(wrt init)"`) and scripting.

func success(msg string) { fmt.Fprintln(os.Stderr, green("$ "+msg)) }
func warn(msg string)    { fmt.Fprintln(os.Stderr, yellow("! "+msg)) }
func info(msg string)    { fmt.Fprintln(os.Stderr, msg) }
func errf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", a...)
}
