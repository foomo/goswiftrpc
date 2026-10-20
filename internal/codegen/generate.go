package codegen

import (
	"errors"

	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/foomo/goswiftrpc/internal/model"
)

// Generate renders the runtime, the service clients and all mapped types, returns file contents by output path
func Generate(cfg *config.Config, m *model.Model) (map[string]string, error) {
	files, err := RenderTypes(m, cfg.Mappings)
	if err != nil {
		return nil, err
	}

	add := func(out, code string) error {
		if _, ok := files[out]; ok {
			return errors.New("multiple outputs render to " + out)
		}

		files[out] = code

		return nil
	}

	if err := add(cfg.Runtime, RenderSwiftRuntime()); err != nil {
		return nil, err
	}

	for _, goPackage := range cfg.ServicePackages() {
		if err := add(cfg.Services[goPackage].Out, RenderSwiftServices(m, m.Services[goPackage], cfg.Mappings)); err != nil {
			return nil, err
		}
	}

	return files, nil
}
