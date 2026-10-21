package codegen //nolint:testpackage

import (
	"testing"

	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const srcMarshal = `package a

import (
	"encoding/hex"
	"encoding/json"
)

// ObjectID is encoded like bson.ObjectID: a 24 char hex string
type ObjectID [12]byte

func (id ObjectID) MarshalText() ([]byte, error) { return []byte(hex.EncodeToString(id[:])), nil }
func (id *ObjectID) UnmarshalText(b []byte) error { _, err := hex.Decode(id[:], b); return err }
func (id ObjectID) MarshalJSON() ([]byte, error) { return json.Marshal(hex.EncodeToString(id[:])) }
func (id *ObjectID) UnmarshalJSON(b []byte) error { return nil }

// Level is an int enum encoded as text
type Level int

const (
	LevelLow Level = iota
	LevelHigh
)

func (l Level) MarshalText() ([]byte, error) { return []byte("low"), nil }
func (l *Level) UnmarshalText(b []byte) error { return nil }

// Money writes json of an unknown shape
type Money struct{ cents int64 }

func (m Money) MarshalJSON() ([]byte, error) { return []byte("1.5"), nil }
func (m *Money) UnmarshalJSON(b []byte) error { return nil }

// WriteOnly has no unmarshaler, gotsrpc encodes it field by field
type WriteOnly struct{ Value string }

func (w WriteOnly) MarshalText() ([]byte, error) { return []byte(w.Value), nil }

type Doc struct {
	ID       ObjectID            ` + "`json:\"id\"`" + `
	Parent   *ObjectID           ` + "`json:\"parent\"`" + `
	Refs     map[ObjectID]Level  ` + "`json:\"refs\"`" + `
	Price    Money               ` + "`json:\"price\"`" + `
	Note     WriteOnly           ` + "`json:\"note\"`" + `
}

type Service interface{ Get(id ObjectID) *Doc }
`

func TestMarshalers(t *testing.T) {
	m := loadPackagesModel(t, [2]string{pkgA, srcMarshal})
	assert.NotContains(t, m.Consts, pkgA+".Level", "text marshaled constants are not visible on the wire")

	files, err := generateForTest(m, map[string]*config.Mapping{pkgA: {Out: "A.swift"}})
	require.NoError(t, err)

	code := files["A.swift"]
	for _, expected := range []string{
		"// encoded by MarshalText\npublic typealias ObjectID = String\n",
		"// encoded by MarshalText\npublic typealias Level = String\n",
		"// encoded by MarshalJSON\npublic typealias Money = GotsRPCJSONValue\n",
		"public struct WriteOnly: Codable, Sendable, Equatable {",
		"public var id: ObjectID\n",
		"public var parent: ObjectID?\n",
		"public var refs: [ObjectID: Level]?\n",
		"public var price: Money\n",
	} {
		assert.Contains(t, code, expected)
	}

	assert.NotContains(t, code, "Level(rawValue")
	assert.Contains(t, files[pkgA+"/Clients.swift"], "public func get(id: ObjectID) async throws -> Doc? {")
}
