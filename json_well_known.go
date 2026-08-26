package protox

import (
	"encoding/json/jsontext"
	"fmt"
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"
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

	// time.Duration is an int64 nanosecond count, overflowing beyond roughly 292 years.
	const maxDurationSeconds = int64(1<<63-1) / int64(time.Second)
	if seconds > maxDurationSeconds || seconds < -maxDurationSeconds {
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
