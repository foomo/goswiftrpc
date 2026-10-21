package codegen

import (
	"errors"

	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/foomo/goswiftrpc/internal/model"
)

// Generate renders the runtime, the service clients and all mapped types, returns file contents by output path
func Generate(cfg *config.Config, m *model.Model) (map[string]string, error) {
	g := newSwiftGen(m, cfg.Mappings)

	files, err := g.renderTypes()
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
		if err := add(cfg.Services[goPackage].Out, g.renderServices(goPackage)); err != nil {
			return nil, err
		}
	}

	// references are recorded while rendering, so names are checked last
	if err := g.checkNames(); err != nil {
		return nil, err
	}

	return files, nil
}
