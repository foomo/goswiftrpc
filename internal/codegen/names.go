package codegen

import (
	"errors"
	"slices"
	"sort"
	"strings"
)

// swiftReservedNames are swift, foundation and runtime names the generated code uses unqualified,
// a generated type of the same name would shadow them
var swiftReservedNames = map[string]bool{
	// swift standard library
	"String": true, "Bool": true, "Int": true, "Int8": true, "Int16": true, "Int32": true, "Int64": true,
	"UInt": true, "UInt8": true, "UInt16": true, "UInt32": true, "UInt64": true, "Float": true, "Double": true,
	"Error": true, "Result": true, "Codable": true, "Encodable": true, "Decodable": true, "Encoder": true, "Decoder": true,
	"CodingKey": true, "CodingKeys": true, "CodingKeyRepresentable": true, "DecodingError": true, "RawRepresentable": true,
	"Sendable": true, "Equatable": true, "Hashable": true, "CustomStringConvertible": true, "Swift": true,
	// foundation
	"Foundation": true, "FoundationNetworking": true, "Data": true, "Date": true, "URL": true, "URLError": true,
	"URLRequest": true, "URLResponse": true, "HTTPURLResponse": true, "URLSession": true, "JSONEncoder": true,
	"JSONDecoder": true, "UTF8": true,
	// goswiftrpc runtime
	"URLSessionTransport": true,
}

// isReservedName tells if name is used by swift or the runtime, all GotsRPC prefixed names belong to the runtime
func isReservedName(name string) bool {
	return swiftReservedNames[name] || strings.HasPrefix(name, "GotsRPC")
}

// swiftDecl is a generated swift declaration
type swiftDecl struct {
	name      string
	namespace string
	origin    string
}

// declarations returns the swift types, clients and namespace enums that will be generated
func (g *swiftGen) declarations() (decls []swiftDecl, namespaces map[string][]string) {
	namespaces = map[string][]string{}
	addNamespace := func(goPackage string) {
		namespace := g.packageNamespace(goPackage)
		if namespace != "" && !slices.Contains(namespaces[namespace], goPackage) {
			namespaces[namespace] = append(namespaces[namespace], goPackage)
		}
	}

	for fullName, named := range g.model.Types {
		goPackage, name := typeName(named)
		decls = append(decls, swiftDecl{name: name, namespace: g.packageNamespace(goPackage), origin: fullName})
		addNamespace(goPackage)
	}

	for goPackage, services := range g.model.Services {
		for _, service := range services {
			decls = append(decls, swiftDecl{
				name:      service.Name + "Client",
				namespace: g.packageNamespace(goPackage),
				origin:    "client of " + service.Package + "." + service.Name,
			})
		}

		addNamespace(goPackage)
	}

	return decls, namespaces
}

// checkNames returns an error if generated swift names collide or shadow each other
func (g *swiftGen) checkNames() error {
	decls, namespaces := g.declarations()

	var problems []string

	// declarations by namespace, "" holds the top level
	scopes := map[string]map[string][]string{"": {}}

	for namespace, goPackages := range namespaces {
		sort.Strings(goPackages)

		if len(goPackages) > 1 {
			problems = append(problems, "swift namespace "+namespace+" is used by multiple go packages: "+strings.Join(goPackages, ", ")+
				"\n  => every go package needs its own namespace in goswiftrpc.yaml")
		}

		scopes[""][namespace] = append(scopes[""][namespace], "namespace of "+strings.Join(goPackages, ", "))
	}

	for _, decl := range decls {
		if scopes[decl.namespace] == nil {
			scopes[decl.namespace] = map[string][]string{}
		}

		scopes[decl.namespace][decl.name] = append(scopes[decl.namespace][decl.name], decl.origin)
	}

	for namespace, scope := range scopes {
		for name, origins := range scope {
			qualified := name
			if namespace != "" {
				qualified = namespace + "." + name
			}

			sort.Strings(origins)

			switch {
			case isReservedName(name):
				problems = append(problems, "swift name "+qualified+" is reserved by swift or the goswiftrpc runtime: "+strings.Join(origins, ", "))
			case len(origins) > 1:
				problems = append(problems, "swift name collision: "+qualified+"\n  "+strings.Join(origins, "\n  ")+
					"\n  => set a namespace for one of these go packages in goswiftrpc.yaml")
			case namespace != "" && len(namespaces[name]) > 0:
				// within the namespace the type would hide the namespace enum of the same name
				problems = append(problems, "swift name "+qualified+" ("+origins[0]+") shadows the namespace "+name)
			}
		}
	}

	for namespace, refs := range g.flatRefs {
		for name, fullName := range refs {
			if origins, ok := scopes[namespace][name]; ok {
				problems = append(problems, "swift name "+name+" ("+fullName+") is used inside the namespace "+namespace+
					", where it resolves to "+namespace+"."+name+" ("+origins[0]+")\n  => set a namespace for "+packageOf(fullName)+" in goswiftrpc.yaml")
			}
		}
	}

	if len(problems) == 0 {
		return nil
	}

	sort.Strings(problems)

	errs := make([]error, 0, len(problems))
	for _, problem := range problems {
		errs = append(errs, errors.New(problem))
	}

	return errors.Join(errs...)
}

func packageOf(fullName string) string {
	if i := strings.LastIndex(fullName, "."); i > 0 {
		return fullName[:i]
	}

	return fullName
}
