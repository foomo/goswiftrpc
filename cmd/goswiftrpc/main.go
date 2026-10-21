package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// command is a goswiftrpc sub command
type command struct {
	name    string
	usage   string
	summary string
	help    string
	run     func(ctx context.Context, args []string, stdout, stderr io.Writer) error
}

// usageError is returned for invalid command lines, the usage of the command is printed with it
type usageError struct {
	cmd *command
	msg string
}

func (e *usageError) Error() string {
	return e.msg
}

var commands []*command

func init() {
	commands = []*command{generateCommand, helpCommand, versionCommand}
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the command line and returns the exit code
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	name, rest := args[0], args[1:]

	switch name {
	case "-h", "--help":
		name = "help"
	case "-v", "--version":
		name = "version"
	}

	cmd := lookupCommand(name)
	if cmd == nil {
		fmt.Fprintf(stderr, "goswiftrpc: unknown command %q\n\n", name)
		printUsage(stderr)

		return 2
	}

	if len(rest) > 0 && (rest[0] == "-h" || rest[0] == "--help") {
		printCommandHelp(stdout, cmd)
		return 0
	}

	if err := cmd.run(ctx, rest, stdout, stderr); err != nil {
		fmt.Fprintln(stderr, "goswiftrpc:", err)

		if usageErr, ok := err.(*usageError); ok { //nolint:errorlint
			fmt.Fprintln(stderr)
			printCommandHelp(stderr, usageErr.cmd)

			return 2
		}

		return 1
	}

	return 0
}

func lookupCommand(name string) *command {
	for _, cmd := range commands {
		if cmd.name == name {
			return cmd
		}
	}

	return nil
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `goswiftrpc generates Swift clients for gotsrpc services.

Usage:
  goswiftrpc <command> [arguments]

Commands:
`)

	for _, cmd := range commands {
		fmt.Fprintf(w, "  %-10s %s\n", cmd.name, cmd.summary)
	}

	fmt.Fprint(w, `
Use "goswiftrpc help <command>" for more information about a command.
`)
}

func printCommandHelp(w io.Writer, cmd *command) {
	fmt.Fprintf(w, "Usage:\n  goswiftrpc %s\n\n%s\n", cmd.usage, strings.TrimSpace(cmd.help))
}

var helpCommand = &command{
	name:    "help",
	usage:   "help [command]",
	summary: "show help for goswiftrpc or a command",
	help:    "Help shows the list of commands, or the help of the given command.",
	run: func(_ context.Context, args []string, stdout, _ io.Writer) error {
		switch len(args) {
		case 0:
			printUsage(stdout)
			return nil
		case 1:
			cmd := lookupCommand(args[0])
			if cmd == nil {
				return fmt.Errorf("unknown command %q", args[0])
			}

			printCommandHelp(stdout, cmd)

			return nil
		default:
			return &usageError{cmd: lookupCommand("help"), msg: "help takes at most one command"}
		}
	},
}
