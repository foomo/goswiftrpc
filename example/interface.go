package example

import (
	"context"
	"net/http"
	"time"

	"github.com/foomo/goswiftrpc/example/common"
)

type (
	LeafID   string
	LeafType string
	Leaf     struct {
		common.Audit
		ID         LeafID            `json:"id"`
		Type       LeafType          `json:"type"`
		CreatedAt  time.Time         `json:"createdAt"`
		Name       string            `json:"name"`
		ParentLeaf *Leaf             `json:"parentLeaf"`
		Tags       []string          `json:"tags,omitempty"`
		Meta       map[string]string `json:"meta,omitempty"`
		Content    []byte            `json:"content,omitempty"`
		Owner      *common.User      `json:"owner,omitempty"`
	}
	Tree struct {
		Root     *Leaf              `json:"root"`
		Children map[LeafID][]*Leaf `json:"children"`
	}
	LeafError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
)

const (
	RootLeafID LeafID = "root"

	LeafTypeFolder LeafType = "folder"
	LeafTypeFile   LeafType = "file"
)

type Service interface {
	// GetLeaf returns the leaf with its parents or nil
	GetLeaf(id LeafID) *Leaf
	// GetTree returns the leaf and all its descendants grouped by parent
	GetTree(ctx context.Context, rootID LeafID) (*Tree, error)
	// ListLeaves returns the children of parentID, filtered by types if given
	ListLeaves(parentID LeafID, types []LeafType) []*Leaf
	// CreateLeaf validates and stores a new leaf
	CreateLeaf(w http.ResponseWriter, r *http.Request, leaf Leaf) (*Leaf, *LeafError)
	// DeleteLeaf removes a leaf without children
	DeleteLeaf(id LeafID) error
	// Count returns the number of leaves
	Count() (total int, byType map[LeafType]int)
	Ping()
}
