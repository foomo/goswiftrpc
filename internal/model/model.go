package model

import (
	"context"
	"errors"
	"fmt"
	"go/types"
	"reflect"
	"sort"
	"strings"

	"github.com/foomo/gocontemplate/pkg/contemplate"
	"github.com/foomo/goswiftrpc/internal/config"
)

// Builtins are named go types that are provided by swift or the swift runtime and are not rendered
var Builtins = map[string]bool{
	"time.Time":                         true,
	"encoding/json.RawMessage":          true,
	"encoding/json/jsontext.Value":      true, // json.RawMessage is an alias of jsontext.Value in newer go versions
	"github.com/foomo/gotsrpc/v3.Error": true,
	"github.com/foomo/gotsrpc/v2.Error": true,
}

// Param is a method argument or return value
type Param struct {
	Name string
	Type types.Type
}

// Method is a service method without leading context / http args
type Method struct {
	Name    string
	Args    []Param
	Returns []Param
}

// Service is a go interface exposed by a gotsrpc proxy
type Service struct {
	Name     string
	Package  string
	Endpoint string
	Methods  []*Method
}

// Model holds the services and all named types reachable from them
type Model struct {
	// Services maps go packages to their services sorted by endpoint
	Services map[string][]*Service
	// Types maps full type names (package.Name) to reachable named types
	Types map[string]*types.Named
	// Consts maps full type names to their constants sorted by name
	Consts map[string][]*types.Const
}

// FullName returns package.Name of a named type, or the plain name of universe types like error
func FullName(named *types.Named) string {
	obj := named.Obj()
	if obj.Pkg() == nil {
		return obj.Name()
	}

	return obj.Pkg().Path() + "." + obj.Name()
}

// Load loads the configured service packages and collects all types reachable from the services
func Load(ctx context.Context, cfg *config.Config) (*Model, error) {
	ctplCfg := &contemplate.Config{Directory: cfg.Module}

	for _, goPackage := range cfg.ServicePackages() {
		var names []string
		for _, name := range cfg.Services[goPackage].Services {
			names = append(names, name)
		}

		sort.Strings(names)

		ctplCfg.Packages = append(ctplCfg.Packages, &contemplate.PackageConfig{Path: goPackage, Types: names})
	}

	ctpl, err := contemplate.Load(ctx, ctplCfg)
	if err != nil {
		return nil, err
	}

	m := New()

	for _, goPackage := range cfg.ServicePackages() {
		pkg := ctpl.Package(goPackage)
		if pkg == nil {
			return nil, errors.New("could not load package " + goPackage)
		}

		if errs := pkg.Raw().Errors; len(errs) > 0 {
			return nil, fmt.Errorf("could not load package %s: %v", goPackage, errs[0])
		}

		endpoints := make([]string, 0, len(cfg.Services[goPackage].Services))
		for endpoint := range cfg.Services[goPackage].Services {
			endpoints = append(endpoints, endpoint)
		}

		sort.Strings(endpoints)

		for _, endpoint := range endpoints {
			name := cfg.Services[goPackage].Services[endpoint]

			if err := m.AddService(pkg.Raw().Types, name, endpoint); err != nil {
				return nil, err
			}
		}
	}

	return m, nil
}

func New() *Model {
	return &Model{
		Services: map[string][]*Service{},
		Types:    map[string]*types.Named{},
		Consts:   map[string][]*types.Const{},
	}
}

// AddService adds the service interface name of pkg and all types reachable from it
func (m *Model) AddService(pkg *types.Package, name, endpoint string) error {
	service, err := m.loadService(pkg, name, endpoint)
	if err != nil {
		return err
	}

	m.Services[pkg.Path()] = append(m.Services[pkg.Path()], service)

	return nil
}

func (m *Model) loadService(pkg *types.Package, name, endpoint string) (*Service, error) {
	obj, ok := pkg.Scope().Lookup(name).(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("service %s.%s not found", pkg.Path(), name)
	}

	iface, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("service %s.%s is not an interface", pkg.Path(), name)
	}

	service := &Service{Name: name, Package: pkg.Path(), Endpoint: endpoint}

	for i := range iface.NumMethods() {
		fn := iface.Method(i)
		if !fn.Exported() {
			continue
		}

		sig, _ := fn.Type().(*types.Signature)
		if sig.Variadic() {
			return nil, fmt.Errorf("variadic method %s.%s.%s is not supported", pkg.Path(), name, fn.Name())
		}

		method := &Method{Name: fn.Name()}

		params := sig.Params()
		for j := skipArgs(params); j < params.Len(); j++ {
			method.Args = append(method.Args, Param{Name: params.At(j).Name(), Type: params.At(j).Type()})
		}

		for j := range sig.Results().Len() {
			method.Returns = append(method.Returns, Param{Name: sig.Results().At(j).Name(), Type: sig.Results().At(j).Type()})
		}

		for _, p := range append(append([]Param{}, method.Args...), method.Returns...) {
			if err := m.walk(p.Type); err != nil {
				return nil, fmt.Errorf("%s.%s.%s: %w", pkg.Path(), name, fn.Name(), err)
			}
		}

		service.Methods = append(service.Methods, method)
	}

	return service, nil
}

// skipArgs returns the number of leading context and http args that are not sent over the wire
func skipArgs(params *types.Tuple) int {
	offset := 0

	for i := range min(params.Len(), 2) {
		switch t := types.TypeString(params.At(i).Type(), nil); {
		case i == 0 && (t == "context.Context" || t == "net/http.ResponseWriter"):
			offset = 1
		case i == 1 && offset == 1 && t == "*net/http.Request":
			offset = 2
		}
	}

	return offset
}

// walk registers all named types reachable from t
func (m *Model) walk(t types.Type) error {
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		obj := t.Obj()
		if obj.Pkg() == nil {
			return nil // error
		}

		fullName := FullName(t)
		if Builtins[fullName] || m.Types[fullName] != nil {
			return nil
		}

		if _, isInterface := t.Underlying().(*types.Interface); isInterface {
			return nil
		}

		if t.TypeArgs().Len() > 0 {
			return errors.New("generic types are not supported yet: " + fullName)
		}

		if str, isStruct := t.Underlying().(*types.Struct); isStruct && isUnion(str) {
			return errors.New("unions are not supported yet: " + fullName)
		}

		m.Types[fullName] = t
		m.collectConsts(t)

		return m.walk(t.Underlying())
	case *types.Pointer:
		return m.walk(t.Elem())
	case *types.Slice:
		return m.walk(t.Elem())
	case *types.Array:
		return m.walk(t.Elem())
	case *types.Map:
		if err := m.walk(t.Key()); err != nil {
			return err
		}

		return m.walk(t.Elem())
	case *types.Struct:
		for i := range t.NumFields() {
			f := t.Field(i)
			if JSONTag(t.Tag(i)).Ignore || (!f.Exported() && !f.Embedded()) {
				continue
			}

			if err := m.walk(f.Type()); err != nil {
				return fmt.Errorf("field %s: %w", f.Name(), err)
			}
		}

		return nil
	case *types.Basic, *types.Interface:
		return nil
	default:
		return fmt.Errorf("unsupported type %s", t)
	}
}

func (m *Model) collectConsts(named *types.Named) {
	scope := named.Obj().Pkg().Scope()

	for _, name := range scope.Names() {
		if c, ok := scope.Lookup(name).(*types.Const); ok && c.Exported() && types.Identical(c.Type(), named) {
			m.Consts[FullName(named)] = append(m.Consts[FullName(named)], c)
		}
	}
}

func isUnion(str *types.Struct) bool {
	for i := range str.NumFields() {
		for _, opt := range strings.Split(reflect.StructTag(str.Tag(i)).Get("gotsrpc"), ",") {
			if strings.TrimSpace(opt) == "union" {
				return true
			}
		}
	}

	return false
}

// JSONInfo is the parsed json struct tag
type JSONInfo struct {
	Name      string
	OmitEmpty bool
	Ignore    bool
}

// JSONTag parses the json part of a struct tag
func JSONTag(tag string) JSONInfo {
	value, ok := reflect.StructTag(tag).Lookup("json")
	if !ok {
		return JSONInfo{}
	}

	if value == "-" {
		return JSONInfo{Ignore: true}
	}

	parts := strings.Split(value, ",")
	info := JSONInfo{Name: parts[0]}

	for _, opt := range parts[1:] {
		if opt == "omitempty" || opt == "omitzero" {
			info.OmitEmpty = true
		}
	}

	return info
}
