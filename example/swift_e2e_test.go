package example_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/foomo/goswiftrpc/example"
	"github.com/foomo/goswiftrpc/example/forest"
	"github.com/foomo/goswiftrpc/internal/codegen"
	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/foomo/goswiftrpc/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const expectedSwiftOutput = `ping
getLeaf readme: id=readme type=file name=README.md createdAt=1700000000000 createdBy=admin:2 parent=docs updatedAt=1700000000000 tags=doc,md meta=lang:en content=hello swift owner=jan:editor
getLeaf readme grandparent: true
getLeaf missing: nil
getTree root: root error=nil
getTree children docs: readme
getTree children root: docs
getTree missing: true error=leaf not found: missing
listLeaves docs: 1
listLeaves root folders: 1
listLeaves root files: nil
count total=3 byType=file:1,folder:2
createLeaf: id=leaf-1 type=file name=notes.txt createdAt=1700000000000 createdBy=swift:1 parent=docs tags=swift meta=client:swift content=from swift leafError=nil
createLeaf invalid: true leafError=400:name must not be empty
count total=4 byType=file:2,folder:2
deleteLeaf docs: error=leaf has children: docs
deleteLeaf created: error=nil
deleteLeaf missing: error=leaf not found: missing
count total=3 byType=file:1,folder:2
forest oak: Quercus robur, the old oak by the creek leafs=8 weight=17.45 shapes=lobed,lobed,lobed,lobed,lobed,lobed,lobed,lobed
forest holly: Ilex aquifolium, the holly guarding the path leafs=6 weight=7.17 shapes=spiny,spiny,spiny,oval,spiny,spiny
forest sassafras: Sassafras albidum, the indecisive sassafras leafs=6 weight=9.57 shapes=oval,mitten,lobed,oval,mitten,lobed
forest pine: Pinus sylvestris, the lonely pine on the ridge leafs=12 weight=0.24 shapes=needle,needle,needle,needle,needle,needle,needle,needle,needle,needle,needle,needle
forest baobab: nil
namespaces: Leaf Forest.Leaf
`

// TestSwiftE2E generates the swift clients, compiles them with a harness and calls the go proxies of both services
func TestSwiftE2E(t *testing.T) {
	swiftc, err := exec.LookPath("swiftc")
	if err != nil {
		t.Skip("swiftc not found")
	}

	dir := t.TempDir()

	cfg, err := config.Load("goswiftrpc.yaml")
	require.NoError(t, err)

	cfg.Runtime = filepath.Join(dir, filepath.Base(cfg.Runtime))
	for _, target := range cfg.Services {
		target.Out = filepath.Join(dir, filepath.Base(target.Out))
	}

	for _, mapping := range cfg.Mappings {
		mapping.Out = filepath.Join(dir, filepath.Base(mapping.Out))
	}

	m, err := model.Load(t.Context(), cfg)
	require.NoError(t, err)

	files, err := codegen.Generate(cfg, m)
	require.NoError(t, err)

	binary := filepath.Join(dir, "harness")
	args := []string{"-swift-version", "6", "-module-cache-path", filepath.Join(dir, "cache"), "-o", binary, "testdata/main.swift"}

	for out, code := range files {
		require.NoError(t, os.WriteFile(out, []byte(code), 0o600))
		args = append(args, out)
	}

	output, err := exec.CommandContext(t.Context(), swiftc, args...).CombinedOutput()
	require.NoError(t, err, string(output))

	treeProxy := example.NewDefaultServiceGoTSRPCProxy(example.NewService())
	forestProxy := forest.NewDefaultServiceGoTSRPCProxy(forest.NewService())

	mux := http.NewServeMux()
	mux.Handle(treeProxy.EndPoint+"/", treeProxy)
	mux.Handle(forestProxy.EndPoint+"/", forestProxy)

	server := httptest.NewServer(mux)
	defer server.Close()

	output, err = exec.CommandContext(t.Context(), binary, server.URL).CombinedOutput()
	require.NoError(t, err, string(output))

	assert.Equal(t, strings.Split(expectedSwiftOutput, "\n"), strings.Split(string(output), "\n"))
}
