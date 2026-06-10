# protox

Go library of reusable helpers for working with Protobuf reflection and annotations.
Used internally at [StreamingFast](https://streamingfast.io) across Firehose and Substreams tooling.

## Installation

```bash
go get github.com/streamingfast/protox
```

Requires Go 1.22+.

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

## License

Apache 2.0
