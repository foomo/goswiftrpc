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
		sw.L("struct Args: Encodable {").Ind(1)

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

		argsValue = "Args(" + strings.Join(values, ", ") + ")"
	}

	resultType := "GotsRPCVoid"

	if len(rets) > 0 {
		resultType = "Result"

		sw.L("struct Result: Decodable {").Ind(1)

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

// RenderSwiftServices renders the swift clients of services
func RenderSwiftServices(m *model.Model, services []*model.Service, mappings map[string]*config.Mapping) string {
	g := newSwiftGen(m, mappings)

	sw := NewCode("    ")
	sw.L("import Foundation")

	for _, service := range services {
		sw.L("")
		g.renderSwiftClient(service, sw)
	}

	return GetSwiftHeaderComment() + swiftCodeString(sw)
}
