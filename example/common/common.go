package common

import (
	"encoding/hex"
	"errors"
	"time"
)

type Role int

const (
	RoleViewer Role = iota
	RoleEditor
	RoleOwner
)

// UserID is encoded like bson.ObjectID: a 24 char hex string, not as an array of bytes
type UserID [12]byte

func (id UserID) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(id[:])), nil
}

func (id *UserID) UnmarshalText(b []byte) error {
	if len(b) != 2*len(id) {
		return errors.New("invalid user id: " + string(b))
	}

	_, err := hex.Decode(id[:], b)

	return err
}

func MustUserID(s string) UserID {
	var id UserID
	if err := id.UnmarshalText([]byte(s)); err != nil {
		panic(err)
	}

	return id
}

type User struct {
	ID   UserID `json:"id"`
	Name string `json:"name"`
	Role Role   `json:"role"`
}

// Audit is embedded into other structs, its fields are flattened in json
type Audit struct {
	CreatedBy User       `json:"createdBy"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}
