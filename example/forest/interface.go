package forest

type (
	TreeType  string
	LeafShape string

	Leaf struct {
		Weight float64
		Shape  LeafShape
	}

	Tree struct {
		Name     string
		TreeType TreeType
		Leafs    []Leaf
	}

	Service interface {
		GetTree(treeType TreeType) *Tree
	}
)

const (
	TreeTypeOak       TreeType = "oak"
	TreeTypeMaple     TreeType = "maple"
	TreeTypeAspen     TreeType = "aspen"
	TreeTypeBirch     TreeType = "birch"
	TreeTypeGinkgo    TreeType = "ginkgo"
	TreeTypeHolly     TreeType = "holly"
	TreeTypeLinden    TreeType = "linden"
	TreeTypePine      TreeType = "pine"
	TreeTypeSassafras TreeType = "sassafras"
	TreeTypeSweetgum  TreeType = "sweetgum"
	TreeTypeWillow    TreeType = "willow"

	LeafShapeRound   LeafShape = "round"
	LeafShapeJagged  LeafShape = "jagged"
	LeafShapeLobed   LeafShape = "lobed"
	LeafShapePalmate LeafShape = "palmate"
	LeafShapeFan     LeafShape = "fan"
	LeafShapeHeart   LeafShape = "heart"
	LeafShapeNeedle  LeafShape = "needle"
	LeafShapeLance   LeafShape = "lance"
	LeafShapeStar    LeafShape = "star"
	LeafShapeSpiny   LeafShape = "spiny"
	LeafShapeOval    LeafShape = "oval"
	LeafShapeMitten  LeafShape = "mitten"
)
