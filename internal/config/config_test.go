package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const services = `
runtime: swift/GotsRPC.swift
services:
  example.com/server:
    out: swift/Clients.swift
    services:
      /service: Service
mappings:
  example.com/server:
    out: swift/VOs.swift
`

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "goswiftrpc.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

func TestLoad(t *testing.T) {
	path := writeConfig(t, services)
	dir := filepath.Dir(path)

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.Equal(t, dir, cfg.Module, "module defaults to the config directory")
	assert.Equal(t, filepath.Join(dir, "swift/GotsRPC.swift"), cfg.Runtime)
	assert.Equal(t, filepath.Join(dir, "swift/Clients.swift"), cfg.Services["example.com/server"].Out)
	assert.Equal(t, filepath.Join(dir, "swift/VOs.swift"), cfg.Mappings["example.com/server"].Out)
}

func TestLoadMissingModule(t *testing.T) {
	path := writeConfig(t, "module: ./does-not-exist\n"+services)

	_, err := config.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `module "./does-not-exist": directory `)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestLoadModuleIsFile(t *testing.T) {
	path := writeConfig(t, "module: goswiftrpc.yaml\n"+services)

	_, err := config.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}
