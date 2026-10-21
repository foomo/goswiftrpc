package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runForTest(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errOut bytes.Buffer
	code = run(t.Context(), args, &out, &errOut)

	return code, out.String(), errOut.String()
}

func TestHelp(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		code, stdout, _ := runForTest(t, args...)
		assert.Equal(t, 0, code, args)

		for _, name := range []string{"generate", "help", "version"} {
			assert.Contains(t, stdout, "\n  "+name+" ", args)
		}
	}
}

func TestHelpCommand(t *testing.T) {
	code, stdout, _ := runForTest(t, "help", "generate")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "goswiftrpc generate <goswiftrpc.yaml>")

	code, stdout, _ = runForTest(t, "generate", "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "goswiftrpc generate <goswiftrpc.yaml>")

	code, _, stderr := runForTest(t, "help", "nope")
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, `unknown command "nope"`)
}

func TestNoCommand(t *testing.T) {
	code, _, stderr := runForTest(t)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "Usage:")
}

func TestUnknownCommand(t *testing.T) {
	code, _, stderr := runForTest(t, "example/goswiftrpc.yaml")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, `unknown command "example/goswiftrpc.yaml"`)
}

func TestVersion(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"-v"}, {"--version"}} {
		code, stdout, _ := runForTest(t, args...)
		assert.Equal(t, 0, code, args)
		assert.True(t, strings.HasPrefix(stdout, "goswiftrpc "), stdout)
	}

	version = "v1.2.3"
	defer func() { version = "" }()

	_, stdout, _ := runForTest(t, "version")
	assert.True(t, strings.HasPrefix(stdout, "goswiftrpc v1.2.3 "), stdout)

	code, _, stderr := runForTest(t, "version", "extra")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "version takes no arguments")
}

func TestGenerateUsage(t *testing.T) {
	code, _, stderr := runForTest(t, "generate")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "generate expects exactly one config file")
	assert.Contains(t, stderr, "goswiftrpc generate <goswiftrpc.yaml>")
}

func TestGenerate(t *testing.T) {
	root, err := filepath.Abs("../..")
	require.NoError(t, err)

	dir := t.TempDir()
	configPath := filepath.Join(dir, "goswiftrpc.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`module: `+root+`
runtime: swift/GotsRPC.swift
services:
  github.com/foomo/goswiftrpc/example/forest:
    out: swift/ForestServiceClients.swift
    services:
      /forest-service: Service
mappings:
  github.com/foomo/goswiftrpc/example/forest:
    out: swift/ForestVOs.swift
    namespace: Forest
`), 0o600))

	code, stdout, stderr := runForTest(t, "generate", configPath)
	require.Equal(t, 0, code, stderr)

	for _, name := range []string{"GotsRPC.swift", "ForestServiceClients.swift", "ForestVOs.swift"} {
		assert.Contains(t, stdout, "wrote "+filepath.Join(dir, "swift", name))
		assert.FileExists(t, filepath.Join(dir, "swift", name))
	}

	code, _, stderr = runForTest(t, "generate", filepath.Join(dir, "missing.yaml"))
	assert.Equal(t, 1, code)
	assert.Contains(t, stderr, "missing.yaml")
}
