package example

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/foomo/goswiftrpc/example/common"
)

// Now is the fixed clock of the example service to keep its replies stable
var Now = time.UnixMilli(1700000000000).UTC()

type entry struct {
	leaf     Leaf
	parentID LeafID
}

type service struct {
	mu      sync.Mutex
	entries map[LeafID]*entry
	order   []LeafID
	nextID  int
}

// NewService returns an in memory service seeded with root/docs/readme
func NewService() Service {
	s := &service{entries: map[LeafID]*entry{}}
	admin := common.User{Name: "admin", Role: common.RoleOwner}

	s.add("", Leaf{ID: RootLeafID, Type: LeafTypeFolder, Name: "root", CreatedAt: time.UnixMilli(0).UTC(), Audit: common.Audit{CreatedBy: admin}})
	s.add(RootLeafID, Leaf{ID: "docs", Type: LeafTypeFolder, Name: "docs", CreatedAt: Now, Audit: common.Audit{CreatedBy: admin}})
	s.add("docs", Leaf{
		ID:        "readme",
		Type:      LeafTypeFile,
		Name:      "README.md",
		CreatedAt: Now,
		Tags:      []string{"doc", "md"},
		Meta:      map[string]string{"lang": "en"},
		Content:   []byte("hello swift"),
		Owner:     &common.User{Name: "jan", Role: common.RoleEditor},
		Audit:     common.Audit{CreatedBy: admin, UpdatedAt: &Now},
	})

	return s
}

func (s *service) add(parentID LeafID, leaf Leaf) {
	leaf.ParentLeaf = nil
	s.entries[leaf.ID] = &entry{leaf: leaf, parentID: parentID}
	s.order = append(s.order, leaf.ID)
}

// resolve returns a copy of the leaf with its parent chain
func (s *service) resolve(id LeafID) *Leaf {
	e, ok := s.entries[id]
	if !ok {
		return nil
	}

	leaf := e.leaf
	if e.parentID != "" {
		leaf.ParentLeaf = s.resolve(e.parentID)
	}

	return &leaf
}

func (s *service) children(parentID LeafID) []*Leaf {
	var ret []*Leaf

	for _, id := range s.order {
		if e := s.entries[id]; e.parentID == parentID {
			leaf := e.leaf
			ret = append(ret, &leaf)
		}
	}

	return ret
}

func (s *service) GetLeaf(id LeafID) *Leaf {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.resolve(id)
}

func (s *service) GetTree(ctx context.Context, rootID LeafID) (*Tree, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	root := s.resolve(rootID)
	if root == nil {
		return nil, errors.New("leaf not found: " + string(rootID))
	}

	tree := &Tree{Root: root, Children: map[LeafID][]*Leaf{}}
	pending := []LeafID{rootID}

	for len(pending) > 0 {
		id := pending[0]
		pending = pending[1:]

		if children := s.children(id); len(children) > 0 {
			tree.Children[id] = children
			for _, child := range children {
				pending = append(pending, child.ID)
			}
		}
	}

	return tree, nil
}

func (s *service) ListLeaves(parentID LeafID, types []LeafType) []*Leaf {
	s.mu.Lock()
	defer s.mu.Unlock()

	var ret []*Leaf

	for _, leaf := range s.children(parentID) {
		if len(types) == 0 || slices.Contains(types, leaf.Type) {
			ret = append(ret, leaf)
		}
	}

	return ret
}

func (s *service) CreateLeaf(w http.ResponseWriter, r *http.Request, leaf Leaf) (*Leaf, *LeafError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if leaf.Name == "" {
		return nil, &LeafError{Code: http.StatusBadRequest, Message: "name must not be empty"}
	}

	parentID := RootLeafID
	if leaf.ParentLeaf != nil {
		parentID = leaf.ParentLeaf.ID
	}

	if _, ok := s.entries[parentID]; !ok {
		return nil, &LeafError{Code: http.StatusNotFound, Message: "parent not found: " + string(parentID)}
	}

	s.nextID++
	leaf.ID = LeafID("leaf-" + strconv.Itoa(s.nextID))
	leaf.CreatedAt = Now

	if leaf.Type == "" {
		leaf.Type = LeafTypeFile
	}

	s.add(parentID, leaf)

	return s.resolve(leaf.ID), nil
}

func (s *service) DeleteLeaf(id LeafID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entries[id]; !ok {
		return errors.New("leaf not found: " + string(id))
	}

	if len(s.children(id)) > 0 {
		return errors.New("leaf has children: " + string(id))
	}

	delete(s.entries, id)
	s.order = slices.DeleteFunc(s.order, func(v LeafID) bool { return v == id })

	return nil
}

func (s *service) Count() (total int, byType map[LeafType]int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	byType = map[LeafType]int{}
	for _, e := range s.entries {
		byType[e.leaf.Type]++
	}

	return len(s.entries), byType
}

func (s *service) Ping() {}
