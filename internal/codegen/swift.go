package codegen

import (
	"errors"
	"fmt"
	"go/constant"
	"go/types"
	"sort"
	"strconv"
	"strings"
	"unicode"

	stringsx "github.com/foomo/go/strings"
	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/foomo/goswiftrpc/internal/model"
)

// swiftTypeOverrides maps go types to types provided by swift or the gotsrpc swift runtime
var swiftTypeOverrides = map[string]string{
	"time.Time":                         "Date",
	"encoding/json.RawMessage":          "GotsRPCJSONValue",
	"encoding/json/jsontext.Value":      "GotsRPCJSONValue",
	"github.com/foomo/gotsrpc/v3.Error": "GotsRPCError",
	"github.com/foomo/gotsrpc/v2.Error": "GotsRPCError",
	"error":                             "GotsRPCError",
}

// swiftNilableOverrides lists overridden types that are nullable on the wire
var swiftNilableOverrides = map[string]bool{
	"github.com/foomo/gotsrpc/v3.Error": true,
	"github.com/foomo/gotsrpc/v2.Error": true,
	"error":                             true,
}

var swiftGoScalarTypes = map[string]string{
	"string":  "String",
	"bool":    "Bool",
	"byte":    "UInt8",
	"rune":    "Int32",
	"int":     "Int",
	"int8":    "Int8",
	"int16":   "Int16",
	"int32":   "Int32",
	"int64":   "Int64",
	"uint":    "UInt",
	"uint8":   "UInt8",
	"uint16":  "UInt16",
	"uint32":  "UInt32",
	"uint64":  "UInt64",
	"float32": "Float",
	"float64": "Double",
}

var swiftKeywords = map[string]bool{
	"associatedtype": true, "class": true, "deinit": true, "enum": true, "extension": true, "fileprivate": true,
	"func": true, "import": true, "init": true, "inout": true, "internal": true, "let": true, "open": true,
	"operator": true, "private": true, "precedencegroup": true, "protocol": true, "public": true, "rethrows": true,
	"static": true, "struct": true, "subscript": true, "typealias": true, "var": true, "break": true, "case": true,
	"catch": true, "continue": true, "default": true, "defer": true, "do": true, "else": true, "fallthrough": true,
	"for": true, "guard": true, "if": true, "in": true, "repeat": true, "return": true, "throw": true, "switch": true,
	"where": true, "while": true, "Any": true, "as": true, "await": true, "false": true, "is": true, "nil": true,
	"self": true, "Self": true, "super": true, "throws": true, "true": true, "try": true, "Type": true,
}

// swiftGen renders swift code for go types
type swiftGen struct {
	model    *model.Model
	mappings map[string]*config.Mapping
	// classes lists structs that recursively contain themselves and are rendered as final classes
	classes map[string]bool
	// namespace is the swift namespace of the code currently rendered
	namespace string
	// flatRefs records references to types without namespace from within a namespace: namespace => swift name => go type
	flatRefs map[string]map[string]string
}

func newSwiftGen(m *model.Model, mappings map[string]*config.Mapping) *swiftGen {
	return &swiftGen{
		model:    m,
		mappings: mappings,
		classes:  recursiveStructs(m.Types),
		flatRefs: map[string]map[string]string{},
	}
}

// packageNamespace returns the swift namespace of a go package or an empty string
func (g *swiftGen) packageNamespace(goPackage string) string {
	if mapping := g.mappings[goPackage]; mapping != nil {
		return mapping.Namespace
	}

	return ""
}

func ucfirst(s string) string {
	if s == "" {
		return s
	}

	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])

	return string(r)
}

// swiftLowerCamel converts go and json names to swift lowerCamelCase identifiers: ID => id, JSONRawMessage => jsonRawMessage
func swiftLowerCamel(name string) string {
	var parts []string

	var current strings.Builder

	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			parts = append(parts, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	if len(parts) == 0 {
		return ""
	}

	first := []rune(parts[0])
	upper := 0

	for upper < len(first) && unicode.IsUpper(first[upper]) {
		upper++
	}

	switch {
	case upper == len(first):
		first = []rune(strings.ToLower(string(first)))
	case upper > 1:
		first = append([]rune(strings.ToLower(string(first[:upper-1]))), first[upper-1:]...)
	case upper == 1:
		first[0] = unicode.ToLower(first[0])
	}

	ident := string(first)
	for _, part := range parts[1:] {
		ident += ucfirst(part)
	}

	if unicode.IsDigit([]rune(ident)[0]) {
		ident = "_" + ident
	}

	return ident
}

// swiftEscape wraps swift keywords in backticks
func swiftEscape(ident string) string {
	if swiftKeywords[ident] {
		return "`" + ident + "`"
	}

	return ident
}

func swiftStringLiteral(s string) string {
	var b strings.Builder

	b.WriteString(`"`)

	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if unicode.IsPrint(r) {
				b.WriteRune(r)
			} else {
				fmt.Fprintf(&b, `\u{%x}`, r)
			}
		}
	}

	b.WriteString(`"`)

	return b.String()
}

// swiftConstLiteral converts a go constant value into a swift literal
func swiftConstLiteral(v constant.Value) (string, error) {
	switch v.Kind() { //nolint:exhaustive
	case constant.String:
		return swiftStringLiteral(constant.StringVal(v)), nil
	case constant.Int:
		return v.ExactString(), nil
	case constant.Float:
		f, _ := constant.Float64Val(v)
		return strconv.FormatFloat(f, 'g', -1, 64), nil
	case constant.Bool:
		return strconv.FormatBool(constant.BoolVal(v)), nil
	default:
		return "", errors.New("unsupported constant " + v.String())
	}
}

func typeName(named *types.Named) (goPackage, name string) {
	if named.Obj().Pkg() == nil {
		return "", named.Obj().Name()
	}

	return named.Obj().Pkg().Path(), named.Obj().Name()
}

// qualifiedName returns the swift type name for a named go type
func (g *swiftGen) qualifiedName(named *types.Named) string {
	if override, ok := swiftTypeOverrides[model.FullName(named)]; ok {
		return override
	}

	goPackage, name := typeName(named)
	if namespace := g.packageNamespace(goPackage); namespace != "" {
		return namespace + "." + name
	}

	// unqualified names are looked up in the current namespace first, record them to detect shadowing
	if g.namespace != "" {
		if g.flatRefs[g.namespace] == nil {
			g.flatRefs[g.namespace] = map[string]string{}
		}

		g.flatRefs[g.namespace][name] = model.FullName(named)
	}

	return name
}

// isNilable tells if a value can be null in json
func (g *swiftGen) isNilable(t types.Type) bool {
	switch t := types.Unalias(t).(type) {
	case *types.Pointer, *types.Map, *types.Slice:
		return true
	case *types.Named:
		fullName := model.FullName(t)
		if _, ok := swiftTypeOverrides[fullName]; ok {
			return swiftNilableOverrides[fullName]
		}

		if _, isInterface := t.Underlying().(*types.Interface); isInterface || model.MarshalingOf(t) != model.MarshalDefault {
			return false
		}

		return g.isNilable(t.Underlying())
	}

	return false
}

func isByte(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == types.Uint8
}

// swiftBasicType maps a go basic type to a swift type
func swiftBasicType(b *types.Basic) string {
	if t, ok := swiftGoScalarTypes[b.Name()]; ok {
		return t
	}

	return "GotsRPCJSONValue"
}

// valueType returns the non optional swift type of a value, anonName is used for inline anonymous structs
func (g *swiftGen) valueType(t types.Type, anonName string) string {
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		if _, isInterface := t.Underlying().(*types.Interface); isInterface && t.Obj().Pkg() != nil {
			if _, isOverride := swiftTypeOverrides[model.FullName(t)]; !isOverride {
				return "GotsRPCJSONValue"
			}
		}

		return g.qualifiedName(t)
	case *types.Pointer:
		return g.valueType(t.Elem(), anonName)
	case *types.Map:
		return "[" + g.valueType(t.Key(), "") + ": " + g.elementType(t.Elem()) + "]"
	case *types.Slice:
		if isByte(t.Elem()) {
			return "Data"
		}

		return "[" + g.elementType(t.Elem()) + "]"
	case *types.Array:
		if isByte(t.Elem()) {
			return "Data"
		}

		return "[" + g.elementType(t.Elem()) + "]"
	case *types.Struct:
		return anonName
	case *types.Basic:
		return swiftBasicType(t)
	default:
		return "GotsRPCJSONValue"
	}
}

// elementType returns the swift type of a nested collection element
func (g *swiftGen) elementType(t types.Type) string {
	st := g.valueType(t, "GotsRPCJSONValue")
	if g.isNilable(t) {
		st += "?"
	}

	return st
}

// swiftField is a resolved struct property
type swiftField struct {
	ident    string
	jsonName string
	typ      string
	optional bool
	anon     *types.Struct
	anonName string
}

func anonStruct(t types.Type) *types.Struct {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}

	str, _ := types.Unalias(t).(*types.Struct)

	return str
}

func (g *swiftGen) resolveField(f *types.Var, info model.JSONInfo, forceOptional bool) swiftField {
	jsonName := info.Name
	if jsonName == "" {
		jsonName = f.Name()
	}

	sf := swiftField{
		jsonName: jsonName,
		ident:    swiftLowerCamel(jsonName),
		optional: forceOptional || g.isNilable(f.Type()) || info.OmitEmpty,
	}

	if anon := anonStruct(f.Type()); anon != nil {
		sf.anon = anon
		sf.anonName = ucfirst(swiftLowerCamel(f.Name()))
	}

	sf.typ = g.valueType(f.Type(), sf.anonName)

	return sf
}

// embeddedStruct returns the struct of an embedded field and whether it is embedded by pointer
func embeddedStruct(t types.Type) (*types.Struct, bool) {
	isPtr := false
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t, isPtr = p.Elem(), true
	}

	named, ok := types.Unalias(t).(*types.Named)
	if !ok || swiftTypeOverrides[model.FullName(named)] != "" {
		return nil, false
	}

	return plainStruct(named), isPtr
}

// plainStruct returns the underlying struct of a named type that is encoded field by field, or nil
func plainStruct(named *types.Named) *types.Struct {
	str, ok := named.Underlying().(*types.Struct)
	if !ok || model.MarshalingOf(named) != model.MarshalDefault {
		return nil
	}

	return str
}

// collectFields flattens a struct including its inline embedded structs, embedded fields come first
// and are shadowed by fields of the embedding struct
func (g *swiftGen) collectFields(str *types.Struct, forceOptional bool, seen map[string]bool) []swiftField {
	var own []swiftField

	type embedding struct {
		str   *types.Struct
		isPtr bool
	}

	var embeddings []embedding

	for i := range str.NumFields() {
		f := str.Field(i)
		info := model.JSONTag(str.Tag(i))

		if info.Ignore {
			continue
		}

		if f.Embedded() && info.Name == "" {
			if embedded, isPtr := embeddedStruct(f.Type()); embedded != nil {
				embeddings = append(embeddings, embedding{str: embedded, isPtr: isPtr})
				continue
			}
		}

		if !f.Exported() {
			continue
		}

		sf := g.resolveField(f, info, forceOptional)
		if seen[sf.jsonName] {
			continue
		}

		seen[sf.jsonName] = true

		own = append(own, sf)
	}

	var embedded []swiftField
	for _, e := range embeddings {
		embedded = append(embedded, g.collectFields(e.str, forceOptional || e.isPtr, seen)...)
	}

	return append(embedded, own...)
}

func (g *swiftGen) renderStructBody(name string, str *types.Struct, asClass bool, sw *Code) {
	fields := g.collectFields(str, false, map[string]bool{})

	// make identifiers unique
	idents := map[string]bool{}
	for i := range fields {
		for idents[fields[i].ident] {
			fields[i].ident += "_"
		}

		idents[fields[i].ident] = true
	}

	property := "public var "
	if asClass {
		// recursive structs become immutable final classes
		property = "public let "
		sw.L("public final class " + name + ": Codable, Sendable, Equatable {").Ind(1)
	} else {
		sw.L("public struct " + name + ": Codable, Sendable, Equatable {").Ind(1)
	}

	for _, f := range fields {
		if f.anon != nil {
			g.renderStructBody(f.anonName, f.anon, false, sw)
			sw.L("")
		}
	}

	for _, f := range fields {
		t := f.typ
		if f.optional {
			t += "?"
		}

		sw.L(property + swiftEscape(f.ident) + ": " + t)
	}

	if len(fields) > 0 {
		sw.L("")
	}

	params := make([]string, 0, len(fields))

	for _, f := range fields {
		param := swiftEscape(f.ident) + ": " + f.typ
		if f.optional {
			param += "? = nil"
		}

		params = append(params, param)
	}

	sw.L("public init(" + strings.Join(params, ", ") + ") {").Ind(1)

	for _, f := range fields {
		sw.L("self." + f.ident + " = " + swiftEscape(f.ident))
	}

	sw.Ind(-1).L("}")

	if asClass {
		comparisons := make([]string, 0, len(fields))
		for _, f := range fields {
			comparisons = append(comparisons, "lhs."+f.ident+" == rhs."+f.ident)
		}

		if len(comparisons) == 0 {
			comparisons = append(comparisons, "true")
		}

		sw.L("")
		sw.L("public static func == (lhs: " + name + ", rhs: " + name + ") -> Bool {").Ind(1)
		sw.L(strings.Join(comparisons, " && "))
		sw.Ind(-1).L("}")
	}

	if len(fields) > 0 {
		sw.L("")
		sw.L("enum CodingKeys: String, CodingKey {").Ind(1)

		for _, f := range fields {
			if f.ident == f.jsonName {
				sw.L("case " + swiftEscape(f.ident))
			} else {
				sw.L("case " + swiftEscape(f.ident) + " = " + swiftStringLiteral(f.jsonName))
			}
		}

		sw.Ind(-1).L("}")
	}

	sw.Ind(-1).L("}")
}

// enumConstants returns the sorted swift constant names and literals of a go enum type
func (g *swiftGen) enumConstants(named *types.Named) (names, literals []string, err error) {
	name := named.Obj().Name()

	for _, c := range g.model.Consts[model.FullName(named)] {
		literal, litErr := swiftConstLiteral(c.Val())
		if litErr != nil {
			return nil, nil, litErr
		}

		constName := swiftLowerCamel(strings.TrimPrefix(stringsx.ToCamel(c.Name()), name))
		if constName == "" {
			constName = swiftLowerCamel(c.Name())
		}

		names = append(names, constName)
		literals = append(literals, literal)
	}

	return names, literals, nil
}

func renderSwiftRawRepresentable(name, rawType string, constNames, literals []string, sw *Code) {
	conformances := "RawRepresentable, Codable, Sendable, Hashable"
	if rawType == "String" || rawType == "Int" {
		conformances += ", CodingKeyRepresentable"
	}

	sw.L("public struct " + name + ": " + conformances + " {").Ind(1)
	sw.L("public let rawValue: " + rawType)
	sw.L("")
	sw.L("public init(rawValue: " + rawType + ") {").Ind(1)
	sw.L("self.rawValue = rawValue")
	sw.Ind(-1).L("}")

	if len(constNames) > 0 {
		sw.L("")
	}

	for i, constName := range constNames {
		sw.L("public static let " + swiftEscape(constName) + " = " + name + "(rawValue: " + literals[i] + ")")
	}

	sw.Ind(-1).L("}")
}

func (g *swiftGen) renderEnum(named *types.Named, basic *types.Basic, sw *Code) error {
	constNames, literals, err := g.enumConstants(named)
	if err != nil {
		return err
	}

	name := named.Obj().Name()
	if constNames == nil {
		sw.L("public typealias " + name + " = " + swiftBasicType(basic))
		return nil
	}

	renderSwiftRawRepresentable(name, swiftBasicType(basic), constNames, literals, sw)

	return nil
}

func (g *swiftGen) renderNamed(named *types.Named, sw *Code) error {
	sw.L("// " + model.FullName(named))

	name := named.Obj().Name()

	switch model.MarshalingOf(named) {
	case model.MarshalText:
		sw.L("// encoded by MarshalText")
		sw.L("public typealias " + name + " = String")

		return nil
	case model.MarshalJSON:
		sw.L("// encoded by MarshalJSON")
		sw.L("public typealias " + name + " = GotsRPCJSONValue")

		return nil
	case model.MarshalDefault:
	}

	switch u := named.Underlying().(type) {
	case *types.Struct:
		g.renderStructBody(name, u, g.classes[model.FullName(named)], sw)
	case *types.Basic:
		return g.renderEnum(named, u, sw)
	case *types.Slice, *types.Array, *types.Map, *types.Pointer:
		sw.L("public typealias " + name + " = " + g.valueType(u, "GotsRPCJSONValue"))
	default:
		return fmt.Errorf("unsupported type %s", model.FullName(named))
	}

	return nil
}

// recursiveStructs returns the structs that contain themselves by value or pointer, swift structs can not
// do that and have to become classes
func recursiveStructs(named map[string]*types.Named) map[string]bool {
	// direct returns the named structs that are stored inline (or behind a pointer) in t
	var direct func(t types.Type, acc map[string]bool)
	direct = func(t types.Type, acc map[string]bool) {
		switch t := types.Unalias(t).(type) {
		case *types.Pointer:
			direct(t.Elem(), acc)
		case *types.Named:
			if _, ok := named[model.FullName(t)]; ok && plainStruct(t) != nil {
				acc[model.FullName(t)] = true
			}
		case *types.Struct:
			for i := range t.NumFields() {
				direct(t.Field(i).Type(), acc)
			}
		}
	}

	edges := map[string]map[string]bool{}

	for fullName, n := range named {
		if str := plainStruct(n); str != nil {
			edges[fullName] = map[string]bool{}
			direct(str, edges[fullName])
		}
	}

	ret := map[string]bool{}

	for start := range edges {
		visited := map[string]bool{}
		stack := []string{start}

		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			for next := range edges[current] {
				if next == start {
					ret[start] = true
				}

				if !visited[next] {
					visited[next] = true
					stack = append(stack, next)
				}
			}
		}
	}

	return ret
}

// RenderTypes renders all named types of the model into the swift files of their package mappings,
// returns file contents by output path
func RenderTypes(m *model.Model, mappings map[string]*config.Mapping) (map[string]string, error) {
	return newSwiftGen(m, mappings).renderTypes()
}

func (g *swiftGen) renderTypes() (map[string]string, error) {
	mappings := g.mappings
	mappedSwift := map[string]map[string]*Code{}

	for fullName, named := range g.model.Types {
		goPackage, name := typeName(named)
		if mappings[goPackage] == nil {
			return nil, errors.New("missing mapping for go package " + goPackage + " (used by " + fullName + ") => add it to the mappings in your goswiftrpc.yaml")
		}

		g.namespace = mappings[goPackage].Namespace
		sw := NewCode("    ")
		if err := g.renderNamed(named, sw); err != nil {
			return nil, err
		}

		if mappedSwift[goPackage] == nil {
			mappedSwift[goPackage] = map[string]*Code{}
		}

		mappedSwift[goPackage][name] = sw
	}

	g.namespace = ""

	files := map[string]string{}

	for goPackage, codeMap := range mappedSwift {
		out := mappings[goPackage].Out
		if _, ok := files[out]; ok {
			return nil, errors.New("multiple mappings render to " + out)
		}

		files[out] = RenderSwiftPackage(goPackage, mappings[goPackage], codeMap)
	}

	return files, nil
}

// RenderSwiftPackage renders the swift file of a mapped go package
func RenderSwiftPackage(goPackage string, mapping *config.Mapping, codeMap map[string]*Code) string {
	names := make([]string, 0, len(codeMap))
	for name := range codeMap {
		names = append(names, name)
	}

	sort.Strings(names)

	sw := NewCode("    ")
	sw.L("import Foundation")
	sw.L("")

	indent := 0

	if mapping.Namespace != "" {
		sw.L("// " + goPackage)
		sw.L("public enum " + mapping.Namespace + " {}")
		sw.L("")
		sw.L("extension " + mapping.Namespace + " {")

		indent = 1
	}

	for i, name := range names {
		if i > 0 {
			sw.L("")
		}

		for _, line := range codeMap[name].lines {
			if strings.TrimSpace(line) == "" {
				sw.L("")
			} else {
				sw.L(strings.Repeat("    ", indent) + line)
			}
		}
	}

	if mapping.Namespace != "" {
		sw.L("}")
	}

	return GetSwiftHeaderComment() + swiftCodeString(sw)
}

// swiftCodeString returns the code without trailing whitespace on blank lines
func swiftCodeString(sw *Code) string {
	lines := strings.Split(sw.String(), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}

	return strings.Join(lines, "\n") + "\n"
}

func GetSwiftHeaderComment() string {
	return "// Code generated by goswiftrpc https://github.com/foomo/goswiftrpc - DO NOT EDIT.\n"
}
