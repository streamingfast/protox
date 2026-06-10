## Unreleased

### Added

- Initial release as a standalone Go module (`github.com/streamingfast/protox`)
- `FindMessageByNameInFiles`: find a message descriptor by full name across a set of file descriptors
- `FindMessageRepeatedFields` / `FindMessageFirstRepeatedField`: shallow lookup of repeated fields on a message descriptor
- `MessageRepeatedFieldCount` / `MessageRepeatedFieldNames`: count and enumerate repeated field names on a message descriptor
- `WalkMessageDescriptors`: depth-first walk of a message descriptor tree with cycle detection
- `WalkMessageFields`: iterator-based depth-first walk of all fields in a message hierarchy, with optional field filter
- `DynamicAsTimestamppb` / `DynamicAsTimestampTime` / `DynamicAsTimestampParts`: convert dynamic `google.protobuf.Timestamp` messages to Go types
- `IsWellKnownTimestampField` / `IsWellKnownGoogleField`: helpers to identify well-known `google.protobuf.*` fields
- `EnumValueToString` / `EnumKnownValuesDebugString`: enum value utilities
- `GetMessageExtensionValue` / `GetFieldExtensionValue`: typed extension value retrieval from message and field descriptors
