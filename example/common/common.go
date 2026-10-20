package common

import "time"

type Role int

const (
	RoleViewer Role = iota
	RoleEditor
	RoleOwner
)

type User struct {
	Name string `json:"name"`
	Role Role   `json:"role"`
}

// Audit is embedded into other structs, its fields are flattened in json
type Audit struct {
	CreatedBy User       `json:"createdBy"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}
