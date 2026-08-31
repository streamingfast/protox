package protox

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// JSONMarshallerOption customizes a JSONMarshaller built through NewJSONMarshaller.
type JSONMarshallerOption func(*jsonMarshallerConfig)

type jsonMarshallerConfig struct {
	bytesEncoder        JSONBytesEncoderFunc
	fieldCamelCase      bool
	alphabeticalFields  bool
	enumsAsNumbers      bool
	int64AsString       bool
	wellKnownAsProto    bool
	anyTypeURL          bool
	strictAny           bool
	unknownFields       bool
	anyResolver         protoregistry.MessageTypeResolver
	anyResolverOverride protoregistry.MessageTypeResolver
	indent              string
	hasIndent           bool
	encodeOptions       []json.Options
}

func newJSONMarshallerConfig() jsonMarshallerConfig {
	return jsonMarshallerConfig{
		bytesEncoder:  JSONBytesToHex,
		anyTypeURL:    true,
		unknownFields: true,
	}
}

// WithJSONBytesEncoding renders Protobuf `bytes` values using the received encoding.
// Defaults to BytesEncodingHex.
func WithJSONBytesEncoding(encoding BytesEncoding) JSONMarshallerOption {
	return WithJSONBytesEncoder(jsonBytesEncoderFor(encoding))
}

// WithJSONBytesEncoder renders Protobuf `bytes` values using a caller-supplied function.
// The function must write exactly one JSON value.
func WithJSONBytesEncoder(encoder JSONBytesEncoderFunc) JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) {
		if encoder != nil {
			config.bytesEncoder = encoder
		}
	}
}

// WithJSONFieldCamelCase names fields using the Protobuf JSON name (`blockNum`) rather than
// the declared name (`block_num`).
func WithJSONFieldCamelCase() JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.fieldCamelCase = true }
}

// WithJSONAlphabeticalFields orders fields by name rather than by declaration order.
func WithJSONAlphabeticalFields() JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.alphabeticalFields = true }
}

// WithJSONEnumsAsNumbers renders enum values as their number rather than their name.
func WithJSONEnumsAsNumbers() JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.enumsAsNumbers = true }
}

// WithJSONInt64AsString renders 64-bit integers as JSON strings, avoiding precision loss in
// consumers using IEEE-754 doubles for all numbers, JavaScript in particular.
func WithJSONInt64AsString() JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.int64AsString = true }
}

// WithJSONWellKnownAsProto renders google.protobuf.* messages using their literal Protobuf
// fields instead of their humanized form: a Timestamp becomes {"seconds":…,"nanos":…}
// rather than an RFC 3339 string.
func WithJSONWellKnownAsProto() JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.wellKnownAsProto = true }
}

// WithoutJSONAnyTypeURL omits the "@type" member when rendering a resolved
// google.protobuf.Any.
func WithoutJSONAnyTypeURL() JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.anyTypeURL = false }
}

// WithJSONStrictAny fails marshalling when a google.protobuf.Any cannot be resolved,
// instead of rendering a degraded object carrying the raw payload.
func WithJSONStrictAny() JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.strictAny = true }
}

// WithJSONAnyResolver consults the received resolver before the default chain, which is the
// protox registry followed by protoregistry.GlobalTypes. Use WithJSONAnyResolverOverride to
// replace the chain entirely.
func WithJSONAnyResolver(resolver protoregistry.MessageTypeResolver) JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.anyResolver = resolver }
}

// WithJSONAnyResolverOverride replaces the whole default resolver chain, so neither the
// protox registry nor protoregistry.GlobalTypes is consulted.
func WithJSONAnyResolverOverride(resolver protoregistry.MessageTypeResolver) JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.anyResolverOverride = resolver }
}

// WithoutJSONUnknownFields omits unknown fields, which are otherwise rendered as
// "__unknown_fields_<number>_with_type_<wire type>__" members.
func WithoutJSONUnknownFields() JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.unknownFields = false }
}

// WithJSONIndent pretty-prints the output using the received indentation, which must be
// composed only of spaces and tabs — the same rule jsontext.WithIndent enforces internally,
// except we degrade instead of panicking: an invalid indent is ignored (a warning is logged)
// and the output stays compact.
func WithJSONIndent(indent string) JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) {
		if strings.Trim(indent, " \t") != "" {
			zlog.Warn("invalid JSON indent, must contain only spaces and tabs, ignoring and keeping compact output",
				zap.String("indent", indent),
			)

			return
		}

		config.indent = indent
		config.hasIndent = true
	}
}

// WithJSONEncodeOptions forwards raw encoding/json/v2 and encoding/json/jsontext options to
// the underlying encoder. Applied after the protox defaults, so they win on conflict.
func WithJSONEncodeOptions(opts ...json.Options) JSONMarshallerOption {
	return func(config *jsonMarshallerConfig) { config.encodeOptions = append(config.encodeOptions, opts...) }
}

// compile-time assertion that jsontext options are usable as json options; both alias
// jsonopts.Options.
var _ json.Options = jsontext.AllowInvalidUTF8(true)
