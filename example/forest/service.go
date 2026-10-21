package forest

import "math"

// grove describes how a tree of a type grows
type grove struct {
	name       string
	leafCount  int
	leafWeight float64     // average leaf weight in grams
	shapes     []LeafShape // shapes the leafs take in turn, from the lowest branch up
}

// groves are the trees standing in the forest
var groves = map[TreeType]grove{
	TreeTypeOak: {
		name: "Quercus robur, the old oak by the creek", leafCount: 8, leafWeight: 2.1,
		shapes: []LeafShape{LeafShapeLobed},
	},
	TreeTypeMaple: {
		name: "Acer rubrum, the red maple at the edge of the clearing", leafCount: 5, leafWeight: 3.4,
		shapes: []LeafShape{LeafShapePalmate},
	},
	TreeTypeAspen: {
		name: "Populus tremula, the aspen that trembles in the faintest breeze", leafCount: 9, leafWeight: 0.9,
		shapes: []LeafShape{LeafShapeRound},
	},
	TreeTypeBirch: {
		name: "Betula pendula, the silver birch glowing in the moonlight", leafCount: 7, leafWeight: 0.4,
		shapes: []LeafShape{LeafShapeJagged},
	},
	TreeTypeGinkgo: {
		name: "Ginkgo biloba, the living fossil that outlived the dinosaurs", leafCount: 6, leafWeight: 0.7,
		shapes: []LeafShape{LeafShapeFan},
	},
	TreeTypeHolly: {
		// holly arms its low leafs against grazing deer, up high they relax
		name: "Ilex aquifolium, the holly guarding the path", leafCount: 6, leafWeight: 1.2,
		shapes: []LeafShape{LeafShapeSpiny, LeafShapeSpiny, LeafShapeSpiny, LeafShapeOval},
	},
	TreeTypeLinden: {
		name: "Tilia cordata, the linden in the village square", leafCount: 6, leafWeight: 1.1,
		shapes: []LeafShape{LeafShapeHeart},
	},
	TreeTypePine: {
		name: "Pinus sylvestris, the lonely pine on the ridge", leafCount: 12, leafWeight: 0.02,
		shapes: []LeafShape{LeafShapeNeedle},
	},
	TreeTypeSassafras: {
		// sassafras can not make up its mind and grows three shapes on one tree
		name: "Sassafras albidum, the indecisive sassafras", leafCount: 6, leafWeight: 1.6,
		shapes: []LeafShape{LeafShapeOval, LeafShapeMitten, LeafShapeLobed},
	},
	TreeTypeSweetgum: {
		name: "Liquidambar styraciflua, the sweetgum full of stars", leafCount: 5, leafWeight: 2.6,
		shapes: []LeafShape{LeafShapeStar},
	},
	TreeTypeWillow: {
		name: "Salix babylonica, the weeping willow over the pond", leafCount: 10, leafWeight: 0.3,
		shapes: []LeafShape{LeafShapeLance},
	},
}

type service struct {
	trees map[TreeType]Tree
}

// NewService returns a forest with one tree of each type
func NewService() Service {
	s := &service{trees: map[TreeType]Tree{}}

	for treeType, g := range groves {
		s.trees[treeType] = Tree{Name: g.name, TreeType: treeType, Leafs: g.grow()}
	}

	return s
}

// grow returns the leafs of a tree, their weights sway gently around the average
func (g grove) grow() []Leaf {
	leafs := make([]Leaf, g.leafCount)

	for i := range leafs {
		weight := g.leafWeight * (1 + 0.2*math.Sin(float64(i+1)))
		leafs[i] = Leaf{
			Weight: math.Round(weight*100) / 100,
			Shape:  g.shapes[i%len(g.shapes)],
		}
	}

	return leafs
}

// GetTree returns a copy of the tree of the given type or nil
func (s *service) GetTree(treeType TreeType) *Tree {
	tree, ok := s.trees[treeType]
	if !ok {
		return nil
	}

	tree.Leafs = append([]Leaf(nil), tree.Leafs...)

	return &tree
}
