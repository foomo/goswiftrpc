package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/foomo/goswiftrpc/internal/codegen"
	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/foomo/goswiftrpc/internal/model"
)

var generateCommand = &command{
	name:    "generate",
	usage:   "generate <goswiftrpc.yaml>",
	summary: "generate swift clients, types and runtime",
	help: `
Generate loads the go packages of the services configured in the yaml file and
writes the swift runtime, the service clients and the types they use.

All paths in the yaml file are relative to the yaml file. Nothing is written if
the configuration is invalid or generated swift names collide.

Example:
  goswiftrpc generate example/goswiftrpc.yaml
`,
	run: func(ctx context.Context, args []string, stdout, _ io.Writer) error {
		if len(args) != 1 {
			return &usageError{cmd: lookupCommand("generate"), msg: "generate expects exactly one config file"}
		}

		return generate(ctx, args[0], stdout)
	},
}

func generate(ctx context.Context, configPath string, stdout io.Writer) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	m, err := model.Load(ctx, cfg)
	if err != nil {
		return err
	}

	files, err := codegen.Generate(cfg, m)
	if err != nil {
		return err
	}

	outs := make([]string, 0, len(files))
	for out := range files {
		outs = append(outs, out)
	}

	sort.Strings(outs)

	for _, out := range outs {
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}

		if err := os.WriteFile(out, []byte(files[out]), 0o644); err != nil { //nolint:gosec
			return err
		}

		fmt.Fprintln(stdout, "wrote", out)
	}

	return nil
}
