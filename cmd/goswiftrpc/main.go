package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/foomo/goswiftrpc/internal/codegen"
	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/foomo/goswiftrpc/internal/model"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: goswiftrpc <goswiftrpc.yaml>")
		os.Exit(1)
	}

	if err := run(context.Background(), os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "goswiftrpc:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, configPath string) error {
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

		fmt.Println("wrote", out)
	}

	return nil
}
