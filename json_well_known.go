package protox

import (
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"math"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

const (
	wellKnownAny       protoreflect.FullName = "google.protobuf.Any"
	wellKnownTimestamp protoreflect.FullName = "google.protobuf.Timestamp"
	wellKnownDuration  protoreflect.FullName = "google.protobuf.Duration"
	wellKnownStruct    protoreflect.FullName = "google.protobuf.Struct"
	wellKnownValue     protoreflect.FullName = "google.protobuf.Value"
	wellKnownListValue protoreflect.FullName = "google.protobuf.ListValue"
	wellKnownFieldMask protoreflect.FullName = "google.protobuf.FieldMask"
	wellKnownEmpty     protoreflect.FullName = "google.protobuf.Empty"
)

var wellKnownWrappers = map[protoreflect.FullName]bool{
	"google.protobuf.DoubleValue": true,
	"google.protobuf.FloatValue":  true,
	"google.protobuf.Int64Value":  true,
	"google.protobuf.UInt64Value": true,
	"google.protobuf.Int32Value":  true,
	"google.protobuf.UInt32Value": true,
	"google.protobuf.BoolValue":   true,
	"google.protobuf.StringValue": true,
	"google.protobuf.BytesValue":  true,
}

// marshalWellKnown renders google.protobuf.* messages in their humanized form. It reports
// handled == false when the message is not a well-known type, leaving the caller to use the
// regular message path.
func (m *JSONMarshaller) marshalWellKnown(enc *jsontext.Encoder, msg protoreflect.Message) (handled bool, err error) {
	descriptor := msg.Descriptor()
	if !IsWellKnownGoogleMessage(descriptor) {
		return false, nil
	}

	name := descriptor.FullName()
	if wellKnownWrappers[name] {
		return true, m.marshalWrapper(enc, msg)
	}

	switch name {
	case wellKnownAny:
		return true, m.marshalAny(enc, msg)
	case wellKnownTimestamp:
		return true, m.marshalTimestamp(enc, msg)
	case wellKnownDuration:
		return true, m.marshalDuration(enc, msg)
	case wellKnownStruct:
		return true, m.marshalStruct(enc, msg)
	case wellKnownValue:
		return true, m.marshalStructValue(enc, msg)
	case wellKnownListValue:
		return true, m.marshalListValue(enc, msg)
	case wellKnownFieldMask:
		return true, m.marshalFieldMask(enc, msg)
	case wellKnownEmpty:
		return true, m.marshalEmptyObject(enc)
	}

	return false, nil
}

// marshalWrapper renders the single `value` field of a wrapper type, reading it even when
// unset so a wrapper holding a zero value renders that value rather than an empty object.
func (m *JSONMarshaller) marshalWrapper(enc *jsontext.Encoder, msg protoreflect.Message) error {
	field := msg.Descriptor().Fields().ByNumber(1)
	if field == nil {
		return fmt.Errorf("wrapper %s has no field 1", msg.Descriptor().FullName())
	}

	return m.marshalSingular(enc, field, msg.Get(field))
}

func (m *JSONMarshaller) marshalTimestamp(enc *jsontext.Encoder, msg protoreflect.Message) error {
	return enc.WriteToken(jsontext.String(DynamicAsTimestampTime(msg).Format(time.RFC3339Nano)))
}

func (m *JSONMarshaller) marshalDuration(enc *jsontext.Encoder, msg protoreflect.Message) error {
	descriptor := msg.Descriptor()

	var seconds, nanos int64
	if field := descriptor.Fields().ByNumber(1); field != nil {
		seconds = msg.Get(field).Int()
	}
	if field := descriptor.Fields().ByNumber(2); field != nil {
		nanos = msg.Get(field).Int()
	}

	// time.Duration is an int64 nanosecond count, overflowing beyond roughly 292 years. The
	// budget must be checked against the combined seconds+nanos value, not just whole
	// seconds: at the exact boundary second, only part of the nanosecond range still fits
	// without wrapping the int64 multiplication/addition below.
	const (
		maxDurationSeconds      = math.MaxInt64 / int64(time.Second)
		maxDurationNanosAtBound = math.MaxInt64 - maxDurationSeconds*int64(time.Second)
		minDurationSeconds      = math.MinInt64 / int64(time.Second)
		minDurationNanosAtBound = math.MinInt64 - minDurationSeconds*int64(time.Second)
	)

	overflow := seconds > maxDurationSeconds || seconds < minDurationSeconds ||
		(seconds == maxDurationSeconds && nanos > maxDurationNanosAtBound) ||
		(seconds == minDurationSeconds && nanos < minDurationNanosAtBound)

	if overflow {
		return enc.WriteToken(jsontext.String(fmt.Sprintf("%ds", seconds)))
	}

	return enc.WriteToken(jsontext.String((time.Duration(seconds)*time.Second + time.Duration(nanos)).String()))
}

func (m *JSONMarshaller) marshalStruct(enc *jsontext.Encoder, msg protoreflect.Message) error {
	field := msg.Descriptor().Fields().ByNumber(1)
	if field == nil || !field.IsMap() {
		return m.marshalRegularMessage(enc, msg)
	}

	return m.marshalMap(enc, field, msg.Get(field).Map())
}

// marshalStructValue renders google.protobuf.Value, whose fields form a oneof:
// 1 null_value, 2 number_value, 3 string_value, 4 bool_value, 5 struct_value, 6 list_value.
func (m *JSONMarshaller) marshalStructValue(enc *jsontext.Encoder, msg protoreflect.Message) error {
	fields := msg.Descriptor().Fields()

	for number := 1; number <= 6; number++ {
		field := fields.ByNumber(protoreflect.FieldNumber(number))
		if field == nil || !msg.Has(field) {
			continue
		}

		if number == 1 {
			return enc.WriteToken(jsontext.Null)
		}

		return m.marshalSingular(enc, field, msg.Get(field))
	}

	return enc.WriteToken(jsontext.Null)
}

func (m *JSONMarshaller) marshalListValue(enc *jsontext.Encoder, msg protoreflect.Message) error {
	field := msg.Descriptor().Fields().ByNumber(1)
	if field == nil || !field.IsList() {
		return m.marshalRegularMessage(enc, msg)
	}

	return m.marshalList(enc, field, msg.Get(field).List())
}

func (m *JSONMarshaller) marshalFieldMask(enc *jsontext.Encoder, msg protoreflect.Message) error {
	field := msg.Descriptor().Fields().ByNumber(1)
	if field == nil || !field.IsList() {
		return m.marshalRegularMessage(enc, msg)
	}

	return m.marshalList(enc, field, msg.Get(field).List())
}

func (m *JSONMarshaller) marshalEmptyObject(enc *jsontext.Encoder) error {
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}

	return enc.WriteToken(jsontext.EndObject)
}

// isHumanizedWellKnownMessage reports whether marshalWellKnown renders this message in a
// humanized form, whose JSON is not necessarily an object.
//
// Built from the exact same constants and wellKnownWrappers map that marshalWellKnown
// switches on, so the two cannot drift apart: adding a case to marshalWellKnown without
// adding it here (or vice versa) is the only way to introduce a mismatch, and both live in
// this same file next to each other.
func isHumanizedWellKnownMessage(message protoreflect.MessageDescriptor) bool {
	if message == nil {
		return false
	}

	name := message.FullName()
	if wellKnownWrappers[name] {
		return true
	}

	switch name {
	case wellKnownAny, wellKnownTimestamp, wellKnownDuration, wellKnownStruct, wellKnownValue,
		wellKnownListValue, wellKnownFieldMask, wellKnownEmpty:
		return true
	}

	return false
}

// marshalAny renders a google.protobuf.Any by resolving its payload and inlining the result
// alongside an "@type" member.
//
// When the payload resolves to a humanized well-known type its JSON is not necessarily an
// object, so it nests under a "value" member instead — the rule protojson uses. A
// descriptorpb.* message, for instance, lives in the google.protobuf package but renders as
// an ordinary JSON object, so it inlines rather than nesting.
func (m *JSONMarshaller) marshalAny(enc *jsontext.Encoder, msg protoreflect.Message) error {
	descriptor := msg.Descriptor()

	var typeURL string
	if field := descriptor.Fields().ByNumber(1); field != nil {
		typeURL = msg.Get(field).String()
	}

	var payload []byte
	if field := descriptor.Fields().ByNumber(2); field != nil {
		payload = msg.Get(field).Bytes()
	}

	resolved, err := m.resolveAny(typeURL, payload)
	if err != nil {
		if m.config.strictAny {
			return fmt.Errorf("resolving any %q: %w", typeURL, err)
		}

		return m.marshalDegradedAny(enc, typeURL, payload, err)
	}

	resolvedMessage := resolved.ProtoReflect()

	if !m.config.anyTypeURL {
		return m.marshalMessage(enc, resolvedMessage)
	}

	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}

	if err := enc.WriteToken(jsontext.String("@type")); err != nil {
		return err
	}

	if err := enc.WriteToken(jsontext.String(typeURL)); err != nil {
		return err
	}

	if isHumanizedWellKnownMessage(resolvedMessage.Descriptor()) {
		if err := enc.WriteToken(jsontext.String("value")); err != nil {
			return err
		}

		if err := m.marshalMessage(enc, resolvedMessage); err != nil {
			return err
		}
	} else if err := m.marshalRegularMessageMembers(enc, resolvedMessage); err != nil {
		return err
	}

	return enc.WriteToken(jsontext.EndObject)
}

func (m *JSONMarshaller) resolveAny(typeURL string, payload []byte) (proto.Message, error) {
	messageType, err := m.resolver.FindMessageByURL(typeURL)
	if err != nil {
		if errors.Is(err, protoregistry.NotFound) {
			return nil, errors.New("no type registered for URL")
		}

		return nil, err
	}

	message := messageType.New().Interface()
	if err := proto.Unmarshal(payload, message); err != nil {
		return nil, fmt.Errorf("unmarshalling payload: %w", err)
	}

	return message, nil
}

// marshalDegradedAny renders an unresolvable Any without failing the document, keeping the
// raw payload so no information is silently dropped.
func (m *JSONMarshaller) marshalDegradedAny(enc *jsontext.Encoder, typeURL string, payload []byte, cause error) error {
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}

	if m.config.anyTypeURL {
		if err := enc.WriteToken(jsontext.String("@type")); err != nil {
			return err
		}

		if err := enc.WriteToken(jsontext.String(typeURL)); err != nil {
			return err
		}
	}

	if err := enc.WriteToken(jsontext.String("@error")); err != nil {
		return err
	}

	if err := enc.WriteToken(jsontext.String(cause.Error())); err != nil {
		return err
	}

	if err := enc.WriteToken(jsontext.String("value")); err != nil {
		return err
	}

	if err := m.config.bytesEncoder(enc, payload); err != nil {
		return err
	}

	return enc.WriteToken(jsontext.EndObject)
}
