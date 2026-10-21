package main

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
)

// version can be set at build time with -ldflags "-X main.version=v1.2.3",
// otherwise it is taken from the module build info
var version = ""

var versionCommand = &command{
	name:    "version",
	usage:   "version",
	summary: "print the goswiftrpc version",
	help:    "Version prints the goswiftrpc version, the vcs revision it was built from and the go version.",
	run: func(_ context.Context, args []string, stdout, _ io.Writer) error {
		if len(args) > 0 {
			return &usageError{cmd: lookupCommand("version"), msg: "version takes no arguments"}
		}

		fmt.Fprintln(stdout, versionString())

		return nil
	},
}

func versionString() string {
	v, revision, modified := version, "", false

	if info, ok := debug.ReadBuildInfo(); ok {
		if v == "" {
			v = info.Main.Version
		}

		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value == "true"
			}
		}
	}

	if v == "" {
		v = "(devel)"
	}

	ret := "goswiftrpc " + v

	if revision != "" {
		if len(revision) > 12 {
			revision = revision[:12]
		}

		if modified {
			revision += "-dirty"
		}

		ret += " (" + revision + ")"
	}

	return ret + " " + runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH
}
