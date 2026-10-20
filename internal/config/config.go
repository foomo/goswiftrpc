package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"go.yaml.in/yaml/v3"
)

// Config is the goswiftrpc.yaml configuration
type Config struct {
	// Module is the directory containing the go.mod file
	Module string `yaml:"module"`
	// Runtime is the output file of the shared swift runtime
	Runtime string `yaml:"runtime"`
	// Services maps go packages to the service interfaces to generate clients for
	Services map[string]*ServiceTarget `yaml:"services"`
	// Mappings maps go packages to the swift files their types are rendered to
	Mappings map[string]*Mapping `yaml:"mappings"`
}

// ServiceTarget renders the clients of the services in a go package into one swift file
type ServiceTarget struct {
	Out string `yaml:"out"`
	// Services maps endpoints to interface names
	Services map[string]string `yaml:"services"`
}

// Mapping renders the types of a go package into one swift file
type Mapping struct {
	Out string `yaml:"out"`
	// Namespace is an optional swift enum the types are nested in
	Namespace string `yaml:"namespace"`
}

// Load reads a config file and resolves all paths relative to it
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("could not parse %s: %w", path, err)
	}

	if err := cfg.resolve(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}

	return cfg, nil
}

func (c *Config) resolve(dir string) error {
	abs := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}

		return filepath.Join(dir, p)
	}

	c.Module = abs(c.Module)

	if c.Runtime == "" {
		return errors.New("missing runtime out")
	}

	c.Runtime = abs(c.Runtime)

	if len(c.Services) == 0 {
		return errors.New("no services configured")
	}

	for goPackage, target := range c.Services {
		if target == nil || target.Out == "" {
			return errors.New("missing out for services of " + goPackage)
		}

		if len(target.Services) == 0 {
			return errors.New("no services configured for " + goPackage)
		}

		target.Out = abs(target.Out)
	}

	for goPackage, mapping := range c.Mappings {
		if mapping == nil || mapping.Out == "" {
			return errors.New("missing out for mapping of " + goPackage)
		}

		mapping.Out = abs(mapping.Out)
	}

	return nil
}

// ServicePackages returns the sorted go packages containing services
func (c *Config) ServicePackages() []string {
	ret := make([]string, 0, len(c.Services))
	for goPackage := range c.Services {
		ret = append(ret, goPackage)
	}

	sort.Strings(ret)

	return ret
}
