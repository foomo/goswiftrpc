package forest_test

import (
	"testing"

	"github.com/foomo/goswiftrpc/example/forest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
	s := forest.NewService()

	oak := s.GetTree(forest.TreeTypeOak)
	require.NotNil(t, oak)
	assert.Equal(t, forest.TreeTypeOak, oak.TreeType)
	assert.Equal(t, "Quercus robur, the old oak by the creek", oak.Name)
	assert.Len(t, oak.Leafs, 8)
	assert.Equal(t, forest.Leaf{Weight: 2.45, Shape: forest.LeafShapeLobed}, oak.Leafs[0])

	oak.Leafs[0].Weight = 100
	assert.InDelta(t, 2.45, s.GetTree(forest.TreeTypeOak).Leafs[0].Weight, 0.001, "returned trees are copies")

	assert.Nil(t, s.GetTree("baobab"))
}

func TestServiceShapes(t *testing.T) {
	s := forest.NewService()

	shapes := func(treeType forest.TreeType) []forest.LeafShape {
		tree := s.GetTree(treeType)
		require.NotNil(t, tree, treeType)

		ret := make([]forest.LeafShape, 0, len(tree.Leafs))
		for _, leaf := range tree.Leafs {
			ret = append(ret, leaf.Shape)
		}

		return ret
	}

	assert.Equal(t, []forest.LeafShape{"spiny", "spiny", "spiny", "oval", "spiny", "spiny"}, shapes(forest.TreeTypeHolly))
	assert.Equal(t, []forest.LeafShape{"oval", "mitten", "lobed", "oval", "mitten", "lobed"}, shapes(forest.TreeTypeSassafras))

	for _, treeType := range []forest.TreeType{
		forest.TreeTypeOak, forest.TreeTypeMaple, forest.TreeTypeAspen, forest.TreeTypeBirch,
		forest.TreeTypeGinkgo, forest.TreeTypeHolly, forest.TreeTypeLinden, forest.TreeTypePine,
		forest.TreeTypeSassafras, forest.TreeTypeSweetgum, forest.TreeTypeWillow,
	} {
		for _, shape := range shapes(treeType) {
			assert.NotEmpty(t, shape, treeType)
		}
	}
}
