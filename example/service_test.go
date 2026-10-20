package example_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/foomo/gocontemplate/pkg/contemplate"
	"github.com/foomo/goswiftrpc/example"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	proxy := example.NewDefaultServiceGoTSRPCProxy(example.NewService())
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}))
	defer testServer.Close()
	client := example.NewServiceGoTSRPCClient(testServer.URL, "/service")

	leaf, errTransport := client.GetLeaf(context.Background(), "readme")
	require.NoError(t, errTransport)
	require.NotNil(t, leaf)
	require.NotNil(t, leaf.ParentLeaf)
	assert.Equal(t, example.LeafID("docs"), leaf.ParentLeaf.ID)
	assert.Equal(t, []byte("hello swift"), leaf.Content)
	assert.True(t, example.Now.Equal(leaf.CreatedAt))

	_, errTree, errTransport := client.GetTree(context.Background(), "missing")
	require.NoError(t, errTransport)
	require.Error(t, errTree)
	assert.Contains(t, errTree.Error(), "leaf not found: missing")

	created, leafErr, errTransport := client.CreateLeaf(context.Background(), example.Leaf{})
	require.NoError(t, errTransport)
	assert.Nil(t, created)
	require.NotNil(t, leafErr)
	assert.Equal(t, http.StatusBadRequest, leafErr.Code)
}

func TestGoContemplate(t *testing.T) {
	const treePath = "github.com/foomo/goswiftrpc/example"
	ctpl, err := contemplate.Load(context.Background(), &contemplate.Config{
		Packages: []*contemplate.PackageConfig{
			{
				Path:  "github.com/foomo/goswiftrpc/example",
				Types: []string{"Service"},
			},
		},
	})
	require.NoError(t, err)
	pkg := ctpl.Package(treePath)
	require.NotNil(t, pkg)

	fmt.Println(ctpl, err)
}
