## Unreleased

### Added

- Initial release as a standalone Go module (`github.com/streamingfast/protox`)
- `FindMessageByNameInFiles`: find a message descriptor by full name across a set of file descriptors
- `FindMessageRepeatedFields` / `FindMessageFirstRepeatedField`: shallow lookup of repeated fields on a message descriptor
- `MessageRepeatedFieldCount` / `MessageRepeatedFieldNames`: count and enumerate repeated field names on a message descriptor
- `WalkMessageDescriptors`: depth-first walk of a message descriptor tree with cycle detection
- `WalkMessageFields`: iterator-based depth-first walk of all fields in a message hierarchy, with optional field filter
- `WalkMessageInstanceFields`: iterator-based depth-first walk of all fields in a proto message **instance** (not just descriptors), recursing into sub-messages and repeated message elements; skips `google.protobuf.*` well-known types and prevents infinite recursion via DFS type-stack
- `DynamicAsTimestamppb` / `DynamicAsTimestampTime` / `DynamicAsTimestampParts`: convert dynamic `google.protobuf.Timestamp` messages to Go types
- `IsWellKnownTimestampField` / `IsWellKnownGoogleField`: helpers to identify well-known `google.protobuf.*` fields
- `EnumValueToString` / `EnumKnownValuesDebugString`: enum value utilities
- `GetMessageExtensionValue` / `GetFieldExtensionValue`: typed extension value retrieval from message and field descriptors
- `NewJSONMarshaller` / `ToJSON` / `ToJSONString`: human-oriented Protobuf to JSON rendering built on Go 1.27's `encoding/json/v2`. Not a `protojson` replacement — output is deliberately lossy and not round-trippable. Generated and dynamic messages render identically
- JSON marshaller options: `WithJSONBytesEncoding`, `WithJSONBytesEncoder`, `WithJSONFieldCamelCase`, `WithJSONAlphabeticalFields`, `WithJSONEnumsAsNumbers`, `WithJSONInt64AsString`, `WithJSONWellKnownAsProto`, `WithoutJSONAnyTypeURL`, `WithJSONStrictAny`, `WithJSONAnyResolver`, `WithJSONAnyResolverOverride`, `WithoutJSONUnknownFields`, `WithJSONIndent`, `WithJSONEncodeOptions`
- `RegisterAnyTypes` / `RegisterAnyFiles` / `AnyTypeResolver`: package-level registry of `google.protobuf.Any` payload types, private to protox and never writing to `protoregistry.GlobalTypes`
- `ChainTypeResolvers`: compose several `protoregistry.MessageTypeResolver` values, which `google.golang.org/protobuf` does not provide
- `BytesEncoding` / `ParseBytesEncoding` / `EncodeBytes`: hexadecimal, base58 and base64 bytes encoding, usable outside JSON rendering
- `IsWellKnownGoogleMessage`: message-level counterpart to `IsWellKnownGoogleField`

### Changed

- Minimum Go version is now 1.27, required by `encoding/json/v2`
