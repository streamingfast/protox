package protox

import (
	"cmp"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// JSONMarshaller renders Protobuf messages as human-readable JSON.
//
// It is not a protojson replacement: the output is deliberately lossy and not
// round-trippable — bytes are hex-encoded, enums are names, unknown fields are raw blobs.
// Reach for protojson when the proto3 JSON specification matters.
//
// A JSONMarshaller is immutable once built and safe for concurrent use.
type JSONMarshaller struct {
	config     jsonMarshallerConfig
	marshalers *json.Marshalers
	resolver   protoregistry.MessageTypeResolver
	options    []json.Options
}

// NewJSONMarshaller builds a marshaller. Build one and reuse it; each call performs setup
// work not worth repeating per message.
func NewJSONMarshaller(opts ...JSONMarshallerOption) *JSONMarshaller {
	config := newJSONMarshallerConfig()
	for _, opt := range opts {
		opt(&config)
	}

	m := &JSONMarshaller{config: config}
	m.resolver = m.buildResolver()

	m.marshalers = json.JoinMarshalers(
		json.MarshalToFunc(m.marshalProtoMessage),
		json.MarshalToFunc(m.marshalRawBytes),
	)

	m.options = []json.Options{
		json.WithMarshalers(m.marshalers),
		jsontext.AllowInvalidUTF8(true),
	}
	if config.hasIndent {
		m.options = append(m.options, jsontext.Multiline(true), jsontext.WithIndent(config.indent))
	}
	m.options = append(m.options, config.encodeOptions...)

	return m
}

func (m *JSONMarshaller) buildResolver() protoregistry.MessageTypeResolver {
	if m.config.anyResolverOverride != nil {
		if m.config.anyResolver != nil {
			zlog.Warn("both WithJSONAnyResolver and WithJSONAnyResolverOverride were provided, the override wins")
		}

		return m.config.anyResolverOverride
	}

	resolvers := make([]protoregistry.MessageTypeResolver, 0, 3)
	if m.config.anyResolver != nil {
		resolvers = append(resolvers, m.config.anyResolver)
	}

	return ChainTypeResolvers(append(resolvers, AnyTypeResolver(), protoregistry.GlobalTypes)...)
}

// Marshal renders v as JSON bytes.
func (m *JSONMarshaller) Marshal(v any) ([]byte, error) {
	return json.Marshal(v, m.options...)
}

// MarshalToString renders v as a JSON string.
func (m *JSONMarshaller) MarshalToString(v any) (string, error) {
	out, err := m.Marshal(v)
	if err != nil {
		return "", err
	}

	return string(out), nil
}

// MarshalWrite renders v as JSON onto w.
func (m *JSONMarshaller) MarshalWrite(w io.Writer, v any) error {
	return json.MarshalWrite(w, v, m.options...)
}

// MarshalEncode renders v onto a caller-owned encoder, letting the caller control
// formatting options such as indentation and invalid UTF-8 handling.
func (m *JSONMarshaller) MarshalEncode(enc *jsontext.Encoder, v any) error {
	return json.MarshalEncode(enc, v, m.options...)
}

// ToJSON renders v as JSON bytes using a single-use marshaller. Build a JSONMarshaller with
// NewJSONMarshaller and reuse it on hot paths.
func ToJSON(v any, opts ...JSONMarshallerOption) ([]byte, error) {
	return NewJSONMarshaller(opts...).Marshal(v)
}

// ToJSONString renders v as a JSON string using a single-use marshaller. Build a
// JSONMarshaller with NewJSONMarshaller and reuse it on hot paths.
func ToJSONString(v any, opts ...JSONMarshallerOption) (string, error) {
	return NewJSONMarshaller(opts...).MarshalToString(v)
}

func (m *JSONMarshaller) marshalProtoMessage(enc *jsontext.Encoder, message proto.Message) error {
	if message == nil {
		return enc.WriteToken(jsontext.Null)
	}

	reflected := message.ProtoReflect()
	if !reflected.IsValid() {
		return enc.WriteToken(jsontext.Null)
	}

	return m.marshalMessage(enc, reflected)
}

func (m *JSONMarshaller) marshalRawBytes(enc *jsontext.Encoder, data []byte) error {
	return m.config.bytesEncoder(enc, data)
}

func (m *JSONMarshaller) marshalMessage(enc *jsontext.Encoder, msg protoreflect.Message) error {
	if !m.config.wellKnownAsProto {
		if handled, err := m.marshalWellKnown(enc, msg); handled {
			return err
		}
	}

	return m.marshalRegularMessage(enc, msg)
}

func (m *JSONMarshaller) marshalRegularMessage(enc *jsontext.Encoder, msg protoreflect.Message) error {
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}

	if err := m.marshalRegularMessageMembers(enc, msg); err != nil {
		return err
	}

	return enc.WriteToken(jsontext.EndObject)
}

func (m *JSONMarshaller) marshalRegularMessageMembers(enc *jsontext.Encoder, msg protoreflect.Message) error {
	if m.config.unknownFields {
		if err := m.marshalUnknownFields(enc, msg.GetUnknown()); err != nil {
			return err
		}
	}

	for _, field := range m.orderedFields(msg) {
		if err := enc.WriteToken(jsontext.String(m.fieldName(field.descriptor))); err != nil {
			return err
		}

		if err := m.marshalValue(enc, field.descriptor, field.value); err != nil {
			return err
		}
	}

	return nil
}

// marshalUnknownFields writes every unknown field in wire order as its own member. A
// malformed buffer stops the scan and emits a single "__unknown_fields_error__" member
// rather than failing the whole document.
func (m *JSONMarshaller) marshalUnknownFields(enc *jsontext.Encoder, unknown protoreflect.RawFields) error {
	if len(unknown) == 0 {
		return nil
	}

	seen := make(map[string]int)
	remaining := []byte(unknown)

	for len(remaining) > 0 {
		number, wireType, length := protowire.ConsumeField(remaining)
		if length < 0 {
			if err := enc.WriteToken(jsontext.String("__unknown_fields_error__")); err != nil {
				return err
			}

			return enc.WriteToken(jsontext.String(fmt.Sprintf("malformed unknown fields buffer: %s", protowire.ParseError(length))))
		}

		name := fmt.Sprintf("__unknown_fields_%d_with_type_%d__", number, wireType)
		if occurrence := seen[name]; occurrence > 0 {
			name = fmt.Sprintf("__unknown_fields_%d_with_type_%d_%d__", number, wireType, occurrence)
		}
		seen[fmt.Sprintf("__unknown_fields_%d_with_type_%d__", number, wireType)]++

		if err := enc.WriteToken(jsontext.String(name)); err != nil {
			return err
		}

		if err := m.config.bytesEncoder(enc, remaining[:length]); err != nil {
			return err
		}

		remaining = remaining[length:]
	}

	return nil
}

type jsonField struct {
	descriptor protoreflect.FieldDescriptor
	value      protoreflect.Value
}

func (m *JSONMarshaller) orderedFields(msg protoreflect.Message) []jsonField {
	var fields []jsonField
	msg.Range(func(descriptor protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		fields = append(fields, jsonField{descriptor: descriptor, value: value})
		return true
	})

	if m.config.alphabeticalFields {
		slices.SortFunc(fields, func(a, b jsonField) int {
			return strings.Compare(m.fieldName(a.descriptor), m.fieldName(b.descriptor))
		})
	} else {
		slices.SortFunc(fields, func(a, b jsonField) int {
			return cmp.Compare(a.descriptor.Number(), b.descriptor.Number())
		})
	}

	return fields
}

func (m *JSONMarshaller) fieldName(field protoreflect.FieldDescriptor) string {
	if m.config.fieldCamelCase {
		return field.JSONName()
	}

	return string(field.Name())
}

func (m *JSONMarshaller) marshalValue(enc *jsontext.Encoder, field protoreflect.FieldDescriptor, value protoreflect.Value) error {
	switch {
	case field.IsMap():
		return m.marshalMap(enc, field, value.Map())
	case field.IsList():
		return m.marshalList(enc, field, value.List())
	default:
		return m.marshalSingular(enc, field, value)
	}
}

func (m *JSONMarshaller) marshalList(enc *jsontext.Encoder, field protoreflect.FieldDescriptor, list protoreflect.List) error {
	if err := enc.WriteToken(jsontext.BeginArray); err != nil {
		return err
	}

	for i := range list.Len() {
		if err := m.marshalSingular(enc, field, list.Get(i)); err != nil {
			return err
		}
	}

	return enc.WriteToken(jsontext.EndArray)
}

func (m *JSONMarshaller) marshalMap(enc *jsontext.Encoder, field protoreflect.FieldDescriptor, mapping protoreflect.Map) error {
	type mapEntry struct {
		key   string
		value protoreflect.Value
	}

	entries := make([]mapEntry, 0, mapping.Len())
	mapping.Range(func(key protoreflect.MapKey, value protoreflect.Value) bool {
		entries = append(entries, mapEntry{key: key.String(), value: value})
		return true
	})

	slices.SortFunc(entries, func(a, b mapEntry) int { return strings.Compare(a.key, b.key) })

	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}

	for _, entry := range entries {
		if err := enc.WriteToken(jsontext.String(entry.key)); err != nil {
			return err
		}

		if err := m.marshalSingular(enc, field.MapValue(), entry.value); err != nil {
			return err
		}
	}

	return enc.WriteToken(jsontext.EndObject)
}

func (m *JSONMarshaller) marshalSingular(enc *jsontext.Encoder, field protoreflect.FieldDescriptor, value protoreflect.Value) error {
	switch field.Kind() {
	case protoreflect.BoolKind:
		return enc.WriteToken(jsontext.Bool(value.Bool()))

	case protoreflect.StringKind:
		return enc.WriteToken(jsontext.String(value.String()))

	case protoreflect.BytesKind:
		return m.config.bytesEncoder(enc, value.Bytes())

	case protoreflect.EnumKind:
		return m.marshalEnum(enc, field.Enum(), value.Enum())

	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return enc.WriteToken(jsontext.Int(value.Int()))

	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return enc.WriteToken(jsontext.Uint(value.Uint()))

	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		if m.config.int64AsString {
			return enc.WriteToken(jsontext.String(strconv.FormatInt(value.Int(), 10)))
		}

		return enc.WriteToken(jsontext.Int(value.Int()))

	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if m.config.int64AsString {
			return enc.WriteToken(jsontext.String(strconv.FormatUint(value.Uint(), 10)))
		}

		return enc.WriteToken(jsontext.Uint(value.Uint()))

	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return m.marshalFloat(enc, value.Float())

	case protoreflect.MessageKind, protoreflect.GroupKind:
		return m.marshalMessage(enc, value.Message())
	}

	return fmt.Errorf("unsupported field kind %s on field %s", field.Kind(), field.FullName())
}

func (m *JSONMarshaller) marshalEnum(enc *jsontext.Encoder, enum protoreflect.EnumDescriptor, number protoreflect.EnumNumber) error {
	if !m.config.enumsAsNumbers {
		if value := enum.Values().ByNumber(number); value != nil {
			return enc.WriteToken(jsontext.String(EnumValueToString(value)))
		}
	}

	return enc.WriteToken(jsontext.Int(int64(number)))
}

func (m *JSONMarshaller) marshalFloat(enc *jsontext.Encoder, value float64) error {
	switch {
	case math.IsNaN(value):
		return enc.WriteToken(jsontext.String("NaN"))
	case math.IsInf(value, 1):
		return enc.WriteToken(jsontext.String("Infinity"))
	case math.IsInf(value, -1):
		return enc.WriteToken(jsontext.String("-Infinity"))
	}

	return enc.WriteToken(jsontext.Float(value))
}
