# goswiftrpc

> Swift clients for [gotsrpc](https://github.com/foomo/gotsrpc) services.

> [!WARNING]
> goswiftrpc is in **alpha**. The configuration format, the generated Swift API and the runtime may change in breaking ways between releases. Review the generated code before you rely on it in production.

`goswiftrpc` reads Go service interfaces and generates Swift clients that talk to gotsrpc v3 proxies over JSON. These are the same proxies (`gotsrpc_gen.go`) that serve the generated TypeScript and Go clients.

It uses [gocontemplate](https://github.com/foomo/gocontemplate) to load the Go packages. It then walks `go/types` from the service methods to find every type that has to exist in Swift. It is a standalone project and does not extend gotsrpc itself.

## Usage

```shell
go run github.com/foomo/goswiftrpc/cmd/goswiftrpc@latest path/to/goswiftrpc.yaml
```

or, inside this repository:

```shell
go run ./cmd/goswiftrpc example/goswiftrpc.yaml
```

### Configuration

All paths are relative to the yaml file.

```yaml
# directory containing the go.mod of the services
module: ..
# shared swift runtime (transport, json coders, GotsRPCError, GotsRPCJSONValue)
runtime: swift/GotsRPC.swift

# go package => clients to generate
services:
  github.com/foomo/goswiftrpc/example:
    out: swift/ExampleServiceClients.swift
    services:
      # endpoint: interface name (same as in gotsrpc.yaml)
      /service: Service

# go package => swift file for its types
mappings:
  github.com/foomo/goswiftrpc/example:
    out: swift/ExampleVOs.swift
  github.com/foomo/goswiftrpc/example/common:
    out: swift/CommonVOs.swift
    # optional: nest the types in `public enum Common {}`
    namespace: Common
```

Every Go package that contains a type reachable from a service needs a mapping. A missing mapping is an error.

### Using the generated client

Add all generated `.swift` files to the same Swift target. They only depend on Foundation, and they compile in Swift 6 language mode.

```swift
let transport = URLSessionTransport(
    baseURL: URL(string: "https://example.com")!,
    headers: ["Authorization": "Bearer ..."]
)
let client = ServiceClient(transport: transport) // endpoint defaults to "/service"

let leaf = try await client.getLeaf(id: LeafID(rawValue: "readme"))
let (tree, err) = try await client.getTree(rootID: .rootLeafID)
```

- Every method is `async throws`. It throws on transport problems: network failures, a non-200 status (`GotsRPCHTTPError`), or a reply that cannot be decoded.
- Errors returned by the Go service are not thrown. They are values: an `error` return becomes `GotsRPCError?`, and a typed error struct becomes its own Swift type, as in the TypeScript client.
- Leading `context.Context`, `http.ResponseWriter` and `*http.Request` arguments are handled by the server and do not appear in Swift.
- Methods with several return values return a labeled tuple.
- Implement `GotsRPCTransport` to plug in your own networking, for example for retries, auth refresh or testing.

## Type mapping

| Go                                                   | Swift                                              |
|------------------------------------------------------|----------------------------------------------------|
| `string`, `bool`                                     | `String`, `Bool`                                   |
| `int`, `int8` … `uint64`                             | `Int`, `Int8` … `UInt64`                           |
| `float32`, `float64`                                 | `Float`, `Double`                                  |
| `time.Time`                                          | `Date` (unix milliseconds on the wire)             |
| `[]byte`                                             | `Data` (base64)                                    |
| `[]T`, `map[K]V`                                     | `[T]?`, `[K: V]?`                                  |
| `*T`                                                 | `T?`                                               |
| `any`, interfaces, `json.RawMessage`                 | `GotsRPCJSONValue`                                 |
| `error`, `gotsrpc.Error`                             | `GotsRPCError?`                                    |
| `struct`                                             | `struct: Codable, Sendable, Equatable`             |
| self referencing `struct`                            | `final class` with `let` properties                |
| named scalar with constants (`type Role int` + iota) | `RawRepresentable` struct with `static let` values |
| named scalar without constants                       | `typealias`                                        |
| named slice / map                                    | `typealias`                                        |

How struct fields are generated:
- `json` tags set the property names (`CodingKeys`). `json:"-"` and unexported fields are skipped.
- Fields that are pointers, slices or maps, or are tagged `omitempty`, become optionals.
- Embedded structs are flattened, as in `encoding/json`.
- Anonymous struct fields become nested Swift structs.
- Swift keywords are escaped with backticks.

Enum constants lose the type name prefix: `LeafTypeFolder` becomes `LeafType.folder`.

### Not supported yet

- gotsrpc unions (`gotsrpc:"union"`)
- generic types
- variadic methods

These fail with an error that says what is not supported.

### Out of scope

The generated clients only speak JSON. gotsrpc also supports msgpack, but Swift has no first-class msgpack support, so goswiftrpc will not support it.

## Example

`example/` contains a gotsrpc service. It is an in-memory tree of leaves that exercises the supported features. The directory has:
- the gotsrpc Go proxy and client, generated with `gotsrpc example/gotsrpc.yaml`
- the generated Swift code in `example/swift/`
- an end-to-end test

The end-to-end test, `example/swift_e2e_test.go`:
1. generates the Swift code
2. compiles it together with `example/testdata/main.swift`
3. starts the Go proxy with `httptest`
4. calls every method from Swift and compares the output

## Development

```shell
go test ./...                                   # unit and e2e tests (e2e skipped without swiftc)
go run ./cmd/goswiftrpc example/goswiftrpc.yaml  # regenerate example/swift
```

| Package                | Purpose                                                    |
|------------------------|------------------------------------------------------------|
| `cmd/goswiftrpc`       | CLI                                                        |
| `internal/config`      | loads `goswiftrpc.yaml`                                    |
| `internal/model`       | loads packages, collects services and reachable named types |
| `internal/codegen`     | renders Swift types, clients and the runtime               |

## License

[MIT](LICENSE)
