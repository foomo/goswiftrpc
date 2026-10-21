package codegen

import (
	"strconv"
	"strings"

	"github.com/foomo/goswiftrpc/internal/config"
	"github.com/foomo/goswiftrpc/internal/model"
)

type swiftParam struct {
	ident string
	typ   string
}

func (g *swiftGen) params(params []model.Param, fallback string) []swiftParam {
	ret := make([]swiftParam, 0, len(params))
	seen := map[string]bool{}

	for index, p := range params {
		ident := swiftLowerCamel(p.Name)
		if ident == "" || ident == "_" {
			ident = fallback
			if index > 0 {
				ident += strconv.Itoa(index)
			}
		}

		for seen[ident] {
			ident += "_"
		}

		seen[ident] = true

		ret = append(ret, swiftParam{ident: ident, typ: g.elementType(p.Type)})
	}

	return ret
}

func (g *swiftGen) renderSwiftClientMethod(method *model.Method, sw *Code) {
	args := g.params(method.Args, "arg")
	rets := g.params(method.Returns, "ret")

	signature := make([]string, 0, len(args))
	for _, arg := range args {
		signature = append(signature, swiftEscape(arg.ident)+": "+arg.typ)
	}

	var returnType string

	switch len(rets) {
	case 0:
	case 1:
		returnType = " -> " + rets[0].typ
	default:
		labels := make([]string, 0, len(rets))
		for _, ret := range rets {
			labels = append(labels, swiftEscape(ret.ident)+": "+ret.typ)
		}

		returnType = " -> (" + strings.Join(labels, ", ") + ")"
	}

	sw.L("public func " + swiftEscape(swiftLowerCamel(method.Name)) + "(" + strings.Join(signature, ", ") + ") async throws" + returnType + " {").Ind(1)

	argsValue := "GotsRPCNoArgs()"

	if len(args) > 0 {
		sw.L("struct _Args: Encodable {").Ind(1)

		values := make([]string, 0, len(args))

		for i, arg := range args {
			sw.L("let a" + strconv.Itoa(i) + ": " + arg.typ)
			values = append(values, "a"+strconv.Itoa(i)+": "+swiftEscape(arg.ident))
		}

		sw.L("")
		sw.L("func encode(to encoder: Encoder) throws {").Ind(1)
		sw.L("var container = encoder.unkeyedContainer()")

		for i := range args {
			sw.L("try container.encode(a" + strconv.Itoa(i) + ")")
		}

		sw.Ind(-1).L("}")
		sw.Ind(-1).L("}")
		sw.L("")

		argsValue = "_Args(" + strings.Join(values, ", ") + ")"
	}

	resultType := "GotsRPCVoid"

	if len(rets) > 0 {
		resultType = "_Result"

		sw.L("struct _Result: Decodable {").Ind(1)

		for i, ret := range rets {
			sw.L("let r" + strconv.Itoa(i) + ": " + ret.typ)
		}

		sw.L("")
		sw.L("init(from decoder: Decoder) throws {").Ind(1)
		sw.L("var container = try decoder.unkeyedContainer()")

		for i, ret := range rets {
			sw.L("r" + strconv.Itoa(i) + " = try container.decode(" + ret.typ + ".self)")
		}

		sw.Ind(-1).L("}")
		sw.Ind(-1).L("}")
		sw.L("")
	}

	call := "try await transport.call(endpoint: endpoint, method: " + swiftStringLiteral(method.Name) + ", args: " + argsValue + ", result: " + resultType + ".self)"

	switch len(rets) {
	case 0:
		sw.L("_ = " + call)
	case 1:
		sw.L("return " + call + ".r0")
	default:
		sw.L("let result = " + call)

		values := make([]string, 0, len(rets))
		for i := range rets {
			values = append(values, "result.r"+strconv.Itoa(i))
		}

		sw.L("return (" + strings.Join(values, ", ") + ")")
	}

	sw.Ind(-1).L("}")
}

func (g *swiftGen) renderSwiftClient(service *model.Service, sw *Code) {
	clientName := service.Name + "Client"

	sw.L("// " + service.Package + "." + service.Name)
	sw.L("public struct " + clientName + ": Sendable {").Ind(1)
	sw.L("public static let defaultEndpoint = " + swiftStringLiteral(service.Endpoint))
	sw.L("")
	sw.L("public let transport: any GotsRPCTransport")
	sw.L("public let endpoint: String")
	sw.L("")
	sw.L("public init(transport: any GotsRPCTransport, endpoint: String = " + clientName + ".defaultEndpoint) {").Ind(1)
	sw.L("self.transport = transport")
	sw.L("self.endpoint = endpoint")
	sw.Ind(-1).L("}")

	for _, method := range service.Methods {
		sw.L("")
		g.renderSwiftClientMethod(method, sw)
	}

	sw.Ind(-1).L("}")
}

// RenderSwiftServices renders the swift clients of the services of a go package
func RenderSwiftServices(m *model.Model, goPackage string, mappings map[string]*config.Mapping) string {
	return newSwiftGen(m, mappings).renderServices(goPackage)
}

// renderServices renders the clients of a go package into the namespace of its mapping
func (g *swiftGen) renderServices(goPackage string) string {
	g.namespace = g.packageNamespace(goPackage)
	defer func() { g.namespace = "" }()

	sw := NewCode("    ")
	sw.L("import Foundation")

	if g.namespace != "" {
		sw.L("")

		// the namespace is declared by the types file, unless the package has no types
		if !g.hasTypes(goPackage) {
			sw.L("// " + goPackage)
			sw.L("public enum " + g.namespace + " {}")
			sw.L("")
		}

		sw.L("extension " + g.namespace + " {").Ind(1)
	}

	for i, service := range g.model.Services[goPackage] {
		if i > 0 || g.namespace == "" {
			sw.L("")
		}

		g.renderSwiftClient(service, sw)
	}

	if g.namespace != "" {
		sw.Ind(-1).L("}")
	}

	return GetSwiftHeaderComment() + swiftCodeString(sw)
}

func (g *swiftGen) hasTypes(goPackage string) bool {
	for _, named := range g.model.Types {
		if p, _ := typeName(named); p == goPackage {
			return true
		}
	}

	return false
}
