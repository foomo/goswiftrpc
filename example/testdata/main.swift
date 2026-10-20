import Foundation

// e2e harness: calls all methods of the example service at the url given as first argument and prints the results
let transport = URLSessionTransport(baseURL: URL(string: CommandLine.arguments[1])!)
let client = ServiceClient(transport: transport)

func millis(_ date: Date?) -> String {
    date.map { String(Int64($0.timeIntervalSince1970 * 1000)) } ?? "nil"
}

func describe(_ leaf: Leaf?) -> String {
    guard let leaf = leaf else {
        return "nil"
    }
    var parts = [
        "id=\(leaf.id.rawValue)",
        "type=\(leaf.type.rawValue)",
        "name=\(leaf.name)",
        "createdAt=\(millis(leaf.createdAt))",
        "createdBy=\(leaf.createdBy.name):\(leaf.createdBy.role.rawValue)",
        "parent=\(leaf.parentLeaf?.id.rawValue ?? "nil")",
    ]
    if let updatedAt = leaf.updatedAt {
        parts.append("updatedAt=\(millis(updatedAt))")
    }
    if let tags = leaf.tags {
        parts.append("tags=\(tags.joined(separator: ","))")
    }
    if let meta = leaf.meta {
        parts.append("meta=\(meta.sorted { $0.key < $1.key }.map { "\($0.key):\($0.value)" }.joined(separator: ","))")
    }
    if let content = leaf.content {
        parts.append("content=\(String(decoding: content, as: UTF8.self))")
    }
    if let owner = leaf.owner {
        parts.append("owner=\(owner.name):\(owner.role == .editor ? "editor" : "other")")
    }
    return parts.joined(separator: " ")
}

func describe(_ error: GotsRPCError?) -> String {
    error.map { "error=\($0.message)" } ?? "error=nil"
}

func describe(_ counts: (total: Int, byType: [LeafType: Int]?)) -> String {
    let byType = (counts.byType ?? [:]).sorted { $0.key.rawValue < $1.key.rawValue }.map { "\($0.key.rawValue):\($0.value)" }
    return "count total=\(counts.total) byType=\(byType.joined(separator: ","))"
}

try await client.ping()
print("ping")

let readme = try await client.getLeaf(id: LeafID(rawValue: "readme"))
print("getLeaf readme: \(describe(readme))")
print("getLeaf readme grandparent: \(readme?.parentLeaf?.parentLeaf?.id == .rootLeafID)")
print("getLeaf missing: \(describe(try await client.getLeaf(id: LeafID(rawValue: "missing"))))")

let tree = try await client.getTree(rootID: .rootLeafID)
print("getTree root: \(tree.ret?.root?.id.rawValue ?? "nil") \(describe(tree.ret1))")
for (parentID, children) in (tree.ret?.children ?? [:]).sorted(by: { $0.key.rawValue < $1.key.rawValue }) {
    print("getTree children \(parentID.rawValue): \((children ?? []).map { $0?.id.rawValue ?? "nil" }.joined(separator: ","))")
}
let missingTree = try await client.getTree(rootID: LeafID(rawValue: "missing"))
print("getTree missing: \(missingTree.ret == nil) \(describe(missingTree.ret1))")

print("listLeaves docs: \((try await client.listLeaves(parentID: LeafID(rawValue: "docs"), types: nil) ?? []).count)")
print("listLeaves root folders: \((try await client.listLeaves(parentID: .rootLeafID, types: [.folder]) ?? []).count)")
print("listLeaves root files: \(try await client.listLeaves(parentID: .rootLeafID, types: [.file]).map { "\($0.count)" } ?? "nil")")

print(describe(try await client.count()))

let docs = Leaf(createdBy: Common.User(name: "", role: .viewer), id: LeafID(rawValue: "docs"), type: .folder, createdAt: Date(), name: "docs")
let created = try await client.createLeaf(leaf: Leaf(
    createdBy: Common.User(name: "swift", role: .editor),
    id: LeafID(rawValue: ""),
    type: .file,
    createdAt: Date(timeIntervalSince1970: 0),
    name: "notes.txt",
    parentLeaf: docs,
    tags: ["swift"],
    meta: ["client": "swift"],
    content: Data("from swift".utf8)
))
print("createLeaf: \(describe(created.ret)) leafError=\(created.ret1.map { "\($0.code):\($0.message)" } ?? "nil")")

let invalid = try await client.createLeaf(leaf: Leaf(createdBy: Common.User(name: "swift", role: .viewer), id: LeafID(rawValue: ""), type: .file, createdAt: Date(), name: ""))
print("createLeaf invalid: \(invalid.ret == nil) leafError=\(invalid.ret1.map { "\($0.code):\($0.message)" } ?? "nil")")

print(describe(try await client.count()))

print("deleteLeaf docs: \(describe(try await client.deleteLeaf(id: LeafID(rawValue: "docs"))))")
print("deleteLeaf created: \(describe(try await client.deleteLeaf(id: created.ret?.id ?? LeafID(rawValue: ""))))")
print("deleteLeaf missing: \(describe(try await client.deleteLeaf(id: LeafID(rawValue: "missing"))))")

print(describe(try await client.count()))
