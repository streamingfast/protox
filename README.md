# protox

Go library of reusable helpers for working with Protobuf reflection and annotations.
Used internally at [StreamingFast](https://streamingfast.io) across Firehose and Substreams tooling.

## Installation

```bash
go get github.com/streamingfast/protox
```

Requires Go 1.27+, for `encoding/json/v2`. No `GOEXPERIMENT` flag is needed —
`encoding/json/v2` ships enabled by default starting with Go 1.27.

## API

### Descriptor helpers

```go
import "github.com/streamingfast/protox"
```

**`FindMessageByNameInFiles`** — find a message descriptor by full name across a set of `FileDescriptorProto`.
Returns `nil, nil` when not found.

```go
msg, err := protox.FindMessageByNameInFiles(files, "mypackage.MyMessage")
```

**`FindMessageRepeatedFields`** — return all shallow repeated fields on a message descriptor.

```go
fields := protox.FindMessageRepeatedFields(descriptor)
```

**`FindMessageFirstRepeatedField`** — return the first repeated field, or `nil` if none.

```go
field := protox.FindMessageFirstRepeatedField(descriptor)
```

**`MessageRepeatedFieldCount`** / **`MessageRepeatedFieldNames`** — count or enumerate the names of repeated fields (shallow).

```go
count := protox.MessageRepeatedFieldCount(descriptor)
names := protox.MessageRepeatedFieldNames(descriptor) // []string
```

### Walking message trees

**`WalkMessageDescriptors`** — depth-first walk of a message descriptor tree. Visits the root and all nested
message descriptors reachable via fields. Cycle-safe (stops if a node was already visited).

```go
protox.WalkMessageDescriptors(root, logger, tracer, func(md protoreflect.MessageDescriptor) {
    fmt.Println(md.FullName())
})
```

**`WalkMessageFields`** — iterator-based depth-first walk of all fields in a message hierarchy.
Skips `google.protobuf.*` well-known types to avoid recursing into built-in messages.
Accepts an optional `fieldFilter` function; return `true` from it to skip a field.

```go
for field := range protox.WalkMessageFields(root, nil) {
    fmt.Println(field.FullName())
}

// with filter — skip fields named "internal"
for field := range protox.WalkMessageFields(root, func(f protoreflect.FieldDescriptor) bool {
    return f.Name() == "internal"
}) {
    fmt.Println(field.FullName())
}
```

### Dynamic Timestamp helpers

Utilities for converting a dynamically-typed `google.protobuf.Timestamp` message (obtained via
`protoreflect.Message`) into Go types. Safe across descriptor registries — avoids the cross-registry
descriptor panic common in Substreams pipelines.

```go
ts  := protox.DynamicAsTimestamppb(msg)   // *timestamppb.Timestamp
t   := protox.DynamicAsTimestampTime(msg) // time.Time (UTC)
sec, ns := protox.DynamicAsTimestampParts(msg) // int64, int64
```

All three return zero values when `msg` is `nil` or not a `google.protobuf.Timestamp`.

### Well-known type helpers

```go
protox.IsWellKnownTimestampField(field) // true if field.Message() == google.protobuf.Timestamp
protox.IsWellKnownGoogleField(field)    // true if field.Message().FullName() starts with "google.protobuf."
```

### Enum helpers

```go
name := protox.EnumValueToString(enumValue)          // string name of the enum value
desc := protox.EnumKnownValuesDebugString(enumDesc)  // "NAME (0), OTHER (1), ..." — for error messages
```

### Extension helpers

Typed retrieval of proto extension values from message or field descriptor options.

```go
// read a custom extension from a message's options
value, found := protox.GetMessageExtensionValue[string](msgDesc, myExtension, "")

// read a custom extension from a field's options
value, found := protox.GetFieldExtensionValue[bool](fieldDesc, myFieldExtension, false)
```

`found` is `false` when the extension is absent; `value` is then `defaultIfUnset`.

### JSON marshalling

Render Protobuf messages as human-readable JSON, built on Go 1.27's `encoding/json/v2`.

This is **not** a `protojson` replacement — the output is deliberately lossy and not
round-trippable. Reach for `protojson` when the proto3 JSON specification matters, and for
protox when a person is going to read the result. It exists to replace
`github.com/streamingfast/firehose-core`'s JSON marshaller, whose Go-struct-reflection
approach cannot treat a generated message and a `*dynamicpb.Message` built from the same
descriptor identically; protox's marshaller renders both to byte-identical JSON.

```go
out, err := protox.ToJSONString(block)
```

Reuse a marshaller on hot paths — building one does setup work not worth repeating per
message:

```go
marshaller := protox.NewJSONMarshaller(
    protox.WithJSONBytesEncoding(protox.BytesEncodingBase58),
    protox.WithJSONIndent("  "),
)

err := marshaller.MarshalWrite(os.Stdout, block)
```

`JSONMarshaller` also exposes `Marshal`, `MarshalToString` and `MarshalEncode` (the last takes
a caller-owned `*jsontext.Encoder`, for when the caller wants to control formatting options
directly). A `JSONMarshaller` is immutable once built and safe for concurrent use.

`WithJSONIndent` validates its argument: an indent must be composed only of spaces and tabs,
the same rule `jsontext.WithIndent` enforces internally. Passing anything else does not
panic — it is ignored (a warning is logged) and the output stays compact.

**Defaults**

| Aspect | Default | Option |
|---|---|---|
| Field names | declared name (`block_num`) | `WithJSONFieldCamelCase()` (`blockNum`) |
| Field order | ascending field number, i.e. `.proto` declaration order in the common case | `WithJSONAlphabeticalFields()` |
| Enums | value name (`"STEP_NEW"`) | `WithJSONEnumsAsNumbers()` |
| Bytes | hexadecimal | `WithJSONBytesEncoding(...)`, `WithJSONBytesEncoder(...)` |
| 64-bit integers | JSON numbers | `WithJSONInt64AsString()` |
| `Timestamp` / `Duration` / wrappers / `Struct` / `Value` / `ListValue` / `Empty` / `FieldMask` | humanized (e.g. `Timestamp` as an RFC 3339 string) | `WithJSONWellKnownAsProto()` (literal Protobuf fields, e.g. `{"seconds":…,"nanos":…}`) |
| `Any` | resolved and inlined with `@type` (see below) | `WithoutJSONAnyTypeURL()` |
| Unresolvable `Any` | degraded object: `@type` (if enabled), `@error`, and the raw payload under `value` | `WithJSONStrictAny()` (fail instead) |
| Unknown fields | rendered as `__unknown_fields_<field number>_with_type_<wire type>__` | `WithoutJSONUnknownFields()` |
| Unset fields | omitted (proto3 fields without explicit presence are indistinguishable from their zero value and are omitted too) | — |
| `NaN` / `±Inf` | `"NaN"` / `"Infinity"` / `"-Infinity"` | — |

**`Any` rendering in detail**

A resolved `Any` payload is inlined as a sibling of `@type` when it renders as a JSON object:

```json
{"@type":"type.googleapis.com/google.protobuf.FieldDescriptorProto","name":"block_num","number":3}
```

It nests under a `value` member instead when the payload is one of the humanized well-known
types above, whose JSON is not necessarily an object (a `Timestamp`, for instance, renders as
a bare string):

```json
{"@type":"type.googleapis.com/google.protobuf.Timestamp","value":"2026-08-25T14:03:11Z"}
```

Note that `descriptorpb.*` messages (`FileDescriptorProto` and friends) live in the
`google.protobuf` package but are **not** humanized by this marshaller, so they take the
inline form, not the nested one.

**Resolving `Any` payloads**

Register the types your `Any` values can hold, typically once at startup:

```go
func init() {
    protox.RegisterAnyFiles(myDescriptorFiles) // *protoregistry.Files
    // or
    protox.RegisterAnyTypes(myMessageTypes...) // ...protoreflect.MessageType
}
```

This registry is private to protox — it never writes to `protoregistry.GlobalTypes`, so it
has no effect on ordinary proto unmarshalling. Registering the same type twice is a no-op;
two different descriptors under one name keeps the first and logs a warning.

Resolution order is `WithJSONAnyResolver(...)` → the protox registry →
`protoregistry.GlobalTypes`. `WithJSONAnyResolverOverride(...)` replaces the chain entirely.

The registry is reusable outside of JSON:

```go
messageType, err := protox.AnyTypeResolver().FindMessageByURL(value.TypeUrl)
```

`protox.ChainTypeResolvers(...)` composes any number of `protoregistry.MessageTypeResolver`
values, which `google.golang.org/protobuf` does not offer on its own.

**Bytes encoding outside JSON**

```go
encoding, err := protox.ParseBytesEncoding("base58") // case-insensitive
text := protox.EncodeBytes(encoding, data)
```

## License

Apache 2.0
