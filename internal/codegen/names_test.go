package codegen //nolint:testpackage

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/foomo/goswiftrpc/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pkgA = "example.com/a"
	pkgB = "example.com/b"
)

// mapImporter resolves already checked packages and falls back to the source importer
type mapImporter struct {
	pkgs     map[string]*types.Package
	fallback types.Importer
}

func (i mapImporter) Import(path string) (*types.Package, error) {
	if pkg, ok := i.pkgs[path]; ok {
		return pkg, nil
	}

	return i.fallback.Import(path)
}

// loadPackagesModel type checks the sources in order and adds the Service of every package that has one
func loadPackagesModel(t *testing.T, sources ...[2]string) *model.Model {
	t.Helper()

	fset := token.NewFileSet()
	imp := mapImporter{pkgs: map[string]*types.Package{}, fallback: importer.ForCompiler(fset, "source", nil)}
	m := model.New()

	for _, source := range sources {
		file, err := parser.ParseFile(fset, source[0]+".go", source[1], 0)
		require.NoError(t, err)

		pkg, err := (&types.Config{Importer: imp}).Check(source[0], fset, []*ast.File{file}, nil)
		require.NoError(t, err)

		imp.pkgs[source[0]] = pkg

		if pkg.Scope().Lookup("Service") != nil {
			require.NoError(t, m.AddService(pkg, "Service", "/"+pkg.Name()))
		}
	}

	return m
}

func generateForTest(m *model.Model, mappings map[string]*config.Mapping) (map[string]string, error) {
	cfg := &config.Config{Runtime: "GotsRPC.swift", Services: map[string]*config.ServiceTarget{}, Mappings: mappings}
	for goPackage := range m.Services {
		cfg.Services[goPackage] = &config.ServiceTarget{Out: goPackage + "/Clients.swift"}
	}

	return Generate(cfg, m)
}

const (
	srcA = `package a

type Leaf struct{ Name string }

type Service interface{ GetLeaf() *Leaf }
`
	srcB = `package b

type Leaf struct{ Weight float64 }

type Service interface{ GetLeaf() *Leaf }
`
)

func TestNamesCollision(t *testing.T) {
	m := loadPackagesModel(t, [2]string{pkgA, srcA}, [2]string{pkgB, srcB})

	_, err := generateForTest(m, map[string]*config.Mapping{pkgA: {Out: "A.swift"}, pkgB: {Out: "B.swift"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "swift name collision: Leaf\n  example.com/a.Leaf\n  example.com/b.Leaf")
	assert.Contains(t, err.Error(), "swift name collision: ServiceClient")
}

func TestNamesNamespace(t *testing.T) {
	m := loadPackagesModel(t, [2]string{pkgA, srcA}, [2]string{pkgB, srcB})

	files, err := generateForTest(m, map[string]*config.Mapping{pkgA: {Out: "A.swift"}, pkgB: {Out: "B.swift", Namespace: "B"}})
	require.NoError(t, err)

	clients := files[pkgB+"/Clients.swift"]
	assert.Contains(t, clients, "extension B {\n    // example.com/b.Service\n    public struct ServiceClient: Sendable {")
	assert.Contains(t, clients, "public func getLeaf() async throws -> B.Leaf? {")
	assert.NotContains(t, clients, "public enum B {}", "declared by the types file")
	assert.Contains(t, files["B.swift"], "public enum B {}")
	assert.Contains(t, files[pkgA+"/Clients.swift"], "public func getLeaf() async throws -> Leaf? {")
}

func TestNamesNamespaceWithoutTypes(t *testing.T) {
	m := loadPackagesModel(t, [2]string{pkgB, `package b

type Service interface{ Ping() }
`})

	files, err := generateForTest(m, map[string]*config.Mapping{pkgB: {Out: "B.swift", Namespace: "B"}})
	require.NoError(t, err)
	assert.Contains(t, files[pkgB+"/Clients.swift"], "public enum B {}\n\nextension B {")
}

func TestNamesSharedNamespace(t *testing.T) {
	m := loadPackagesModel(t, [2]string{pkgA, srcA}, [2]string{pkgB, srcB})

	_, err := generateForTest(m, map[string]*config.Mapping{pkgA: {Out: "A.swift", Namespace: "Shared"}, pkgB: {Out: "B.swift", Namespace: "Shared"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "swift namespace Shared is used by multiple go packages: example.com/a, example.com/b")
}

func TestNamesNamespaceCollidesWithType(t *testing.T) {
	m := loadPackagesModel(t, [2]string{pkgA, srcA}, [2]string{pkgB, srcB})

	_, err := generateForTest(m, map[string]*config.Mapping{pkgA: {Out: "A.swift"}, pkgB: {Out: "B.swift", Namespace: "Leaf"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "swift name collision: Leaf\n  example.com/a.Leaf\n  namespace of example.com/b")
	assert.Contains(t, err.Error(), "swift name Leaf.Leaf (example.com/b.Leaf) shadows the namespace Leaf")
}

func TestNamesShadowedFlatReference(t *testing.T) {
	// b.Tree references a.Leaf, which is flat and resolves to B.Leaf within the namespace B
	m := loadPackagesModel(t, [2]string{pkgA, srcA}, [2]string{pkgB, `package b

import "example.com/a"

type Leaf struct{ Weight float64 }

type Tree struct {
	Leafs  []Leaf
	Origin a.Leaf
}

type Service interface{ GetTree() *Tree }
`})

	_, err := generateForTest(m, map[string]*config.Mapping{pkgA: {Out: "A.swift"}, pkgB: {Out: "B.swift", Namespace: "B"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "swift name Leaf (example.com/a.Leaf) is used inside the namespace B, where it resolves to B.Leaf (example.com/b.Leaf)\n  => set a namespace for example.com/a")

	_, err = generateForTest(m, map[string]*config.Mapping{pkgA: {Out: "A.swift", Namespace: "A"}, pkgB: {Out: "B.swift", Namespace: "B"}})
	require.NoError(t, err)
}

func TestNamesReserved(t *testing.T) {
	m := loadPackagesModel(t, [2]string{pkgA, `package a

type Date struct{ Day int }
type GotsRPCThing string
type Result struct{ OK bool }
type Error struct{ Msg string }

type Service interface{ Get(d Date, t GotsRPCThing, e Error) Result }
`})

	// within a namespace only names used by generated types and clients are reserved
	_, err := generateForTest(m, map[string]*config.Mapping{pkgA: {Out: "A.swift", Namespace: "A"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "swift name A.Date is reserved by swift or the goswiftrpc runtime")
	assert.Contains(t, err.Error(), "swift name A.GotsRPCThing is reserved by swift or the goswiftrpc runtime")
	assert.NotContains(t, err.Error(), "A.Result")
	assert.NotContains(t, err.Error(), "A.Error")

	// at the top level names used by the runtime are reserved too
	_, err = generateForTest(m, map[string]*config.Mapping{pkgA: {Out: "A.swift"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "swift name Error is reserved by swift or the goswiftrpc runtime")
	assert.NotContains(t, err.Error(), "swift name Result")
}

func TestNamesNamespacedResult(t *testing.T) {
	m := loadPackagesModel(t, [2]string{pkgA, `package a

type Result struct{ Hits int }

type Service interface{ Search(q string) (Result, error) }
`})

	files, err := generateForTest(m, map[string]*config.Mapping{pkgA: {Out: "A.swift", Namespace: "Search"}})
	require.NoError(t, err)
	assert.Contains(t, files["A.swift"], "public struct Result: Codable, Sendable, Equatable {")
	assert.Contains(t, files[pkgA+"/Clients.swift"], "public func search(q: String) async throws -> (ret: Search.Result, ret1: GotsRPCError?) {")
}
