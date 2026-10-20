package codegen //nolint:testpackage

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/foomo/goswiftrpc/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPkg = "example.com/server"

const testSource = `package server

import (
	"context"
	"encoding/json"
	"time"
)

type Status string

const (
	StatusActive  Status = "active"
	StatusDefault Status = "default"
)

type Count int64

type Tags []string

type ID [12]byte

type Base struct {
	CreatedAt time.Time ` + "`json:\"createdAt\"`" + `
}

type Node struct {
	Base
	ID       string          ` + "`json:\"id\"`" + `
	Status   Status          ` + "`json:\"status\"`" + `
	Note     string          ` + "`json:\"@note,omitempty\"`" + `
	Skip     string          ` + "`json:\"-\"`" + `
	hidden   string
	Parent   *Node           ` + "`json:\"parent\"`" + `
	Children []*Node         ` + "`json:\"children\"`" + `
	Raw      json.RawMessage ` + "`json:\"raw\"`" + `
	Data     any             ` + "`json:\"data\"`" + `
	Counts   map[Status]Count
	Bytes    []byte
	Default  bool
	Inline   struct {
		Name string ` + "`json:\"name\"`" + `
	}
}

type Service interface {
	Get(ctx context.Context, id string) (*Node, error)
	Tags(t Tags, i ID) Tags
	Ping()
}
`

func loadTestModel(t *testing.T) *model.Model {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "server.go", testSource, 0)
	require.NoError(t, err)

	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check(testPkg, fset, []*ast.File{file}, nil)
	require.NoError(t, err)

	m := model.New()
	require.NoError(t, m.AddService(pkg, "Service", "/service"))

	return m
}

func testMappings() map[string]*config.Mapping {
	return map[string]*config.Mapping{testPkg: {Out: "VO.swift"}}
}

func TestSwiftLowerCamel(t *testing.T) {
	for in, expected := range map[string]string{
		"Name":           "name",
		"ID":             "id",
		"ObjectID":       "objectID",
		"JSONRawMessage": "jsonRawMessage",
		"URLPath":        "urlPath",
		"@alpha":         "alpha",
		"#hash":          "hash",
		"snake_case":     "snake_case",
		"kebab-case":     "kebabCase",
		"A1":             "a1",
		"1st":            "_1st",
	} {
		assert.Equal(t, expected, swiftLowerCamel(in), in)
	}
}

func TestSwiftEscape(t *testing.T) {
	assert.Equal(t, "`struct`", swiftEscape("struct"))
	assert.Equal(t, "`default`", swiftEscape("default"))
	assert.Equal(t, "value", swiftEscape("value"))
}

func TestSwiftConstLiteral(t *testing.T) {
	for _, tt := range []struct {
		value    constant.Value
		expected string
	}{
		{constant.MakeString("one"), `"one"`},
		{constant.MakeString(`raw"`), `"raw\""`},
		{constant.MakeString("a\nb"), `"a\nb"`},
		{constant.MakeInt64(16), "16"},
		{constant.MakeUint64(18446744073709551615), "18446744073709551615"},
		{constant.MakeFloat64(1.5), "1.5"},
		{constant.MakeBool(true), "true"},
	} {
		actual, err := swiftConstLiteral(tt.value)
		require.NoError(t, err, tt.value.String())
		assert.Equal(t, tt.expected, actual, tt.value.String())
	}
}

func TestModelTypes(t *testing.T) {
	m := loadTestModel(t)

	for _, name := range []string{"Status", "Count", "Tags", "ID", "Base", "Node"} {
		assert.Contains(t, m.Types, testPkg+"."+name)
	}

	assert.NotContains(t, m.Types, "time.Time")
	assert.Len(t, m.Consts[testPkg+".Status"], 2)

	methods := m.Services[testPkg][0].Methods
	require.Len(t, methods, 3)
	assert.Equal(t, "Get", methods[0].Name)
	assert.Len(t, methods[0].Args, 1, "context is skipped")
}

func TestRenderTypes(t *testing.T) {
	m := loadTestModel(t)

	files, err := RenderTypes(m, testMappings())
	require.NoError(t, err)

	code := files["VO.swift"]
	for _, expected := range []string{
		"public final class Node: Codable, Sendable, Equatable {",
		"public let createdAt: Date\n",
		"public let id: String\n",
		"public let status: Status\n",
		"public let note: String?\n",
		`case note = "@note"`,
		"public let parent: Node?\n",
		"public let children: [Node?]?\n",
		"public let raw: GotsRPCJSONValue\n",
		"public let data: GotsRPCJSONValue\n",
		"public let counts: [Status: Count]?\n",
		"public let bytes: Data?\n",
		"public let `default`: Bool\n",
		"public let inline: Inline\n",
		"public struct Inline: Codable, Sendable, Equatable {",
		"public static func == (lhs: Node, rhs: Node) -> Bool {",
		"public struct Base: Codable, Sendable, Equatable {",
		"public var createdAt: Date\n",
		"public struct Status: RawRepresentable, Codable, Sendable, Hashable, CodingKeyRepresentable {",
		`public static let active = Status(rawValue: "active")`,
		"public static let `default` = Status(rawValue: \"default\")",
		"public typealias Count = Int64",
		"public typealias Tags = [String]",
		"public typealias ID = Data",
	} {
		assert.Contains(t, code, expected)
	}

	assert.NotContains(t, code, "skip")
	assert.NotContains(t, code, "hidden")

	for _, line := range strings.Split(code, "\n") {
		assert.Equal(t, strings.TrimRight(line, " "), line, "trailing whitespace")
	}
}

func TestRenderTypesNamespace(t *testing.T) {
	m := loadTestModel(t)

	files, err := RenderTypes(m, map[string]*config.Mapping{testPkg: {Out: "VO.swift", Namespace: "Server"}})
	require.NoError(t, err)

	code := files["VO.swift"]
	assert.Contains(t, code, "public enum Server {}")
	assert.Contains(t, code, "public let parent: Server.Node?\n")
}

func TestRenderTypesMissingMapping(t *testing.T) {
	_, err := RenderTypes(loadTestModel(t), map[string]*config.Mapping{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing mapping")
}

func TestSwiftTypecheck(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not found")
	}

	m := loadTestModel(t)
	dir := t.TempDir()
	cfg := &config.Config{
		Runtime:  filepath.Join(dir, "GotsRPC.swift"),
		Services: map[string]*config.ServiceTarget{testPkg: {Out: filepath.Join(dir, "Clients.swift")}},
		Mappings: map[string]*config.Mapping{testPkg: {Out: filepath.Join(dir, "VO.swift")}},
	}

	files, err := Generate(cfg, m)
	require.NoError(t, err)

	args := []string{"-typecheck", "-swift-version", "6", "-module-cache-path", filepath.Join(dir, "cache")}

	for out, code := range files {
		require.NoError(t, os.WriteFile(out, []byte(code), 0o600))
		args = append(args, out)
	}

	output, err := exec.CommandContext(t.Context(), swiftc, args...).CombinedOutput()
	require.NoError(t, err, string(output))
}

func TestRenderSwiftServices(t *testing.T) {
	m := loadTestModel(t)

	code := RenderSwiftServices(m, m.Services[testPkg], testMappings())
	for _, expected := range []string{
		"public struct ServiceClient: Sendable {",
		`public static let defaultEndpoint = "/service"`,
		"public func get(id: String) async throws -> (ret: Node?, ret1: GotsRPCError?) {",
		"public func tags(t: Tags?, i: ID) async throws -> Tags? {",
		"public func ping() async throws {",
		`_ = try await transport.call(endpoint: endpoint, method: "Ping", args: GotsRPCNoArgs(), result: GotsRPCVoid.self)`,
	} {
		assert.Contains(t, code, expected)
	}
}
