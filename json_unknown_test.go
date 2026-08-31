package protox_test

import (
	"testing"

	"github.com/streamingfast/protox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// unknownFieldsBytes builds a raw unknown-fields buffer holding varint field 100 = 1 and
// varint field 101 = 2.
func unknownFieldsBytes() protoreflect.RawFields {
	var out []byte
	out = protowire.AppendTag(out, 100, protowire.VarintType)
	out = protowire.AppendVarint(out, 1)
	out = protowire.AppendTag(out, 101, protowire.VarintType)
	out = protowire.AppendVarint(out, 2)

	return out
}

func TestJSONMarshaller_unknownFields_allEmitted(t *testing.T) {
	msg := scalarsMessage(t)
	msg.SetUnknown(unknownFieldsBytes())
	setField(t, msg, "a_bool", protoreflect.ValueOfBool(true))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t,
		`{"__unknown_fields_100_with_type_0__":"a00601","__unknown_fields_101_with_type_0__":"a80602","a_bool":true}`,
		out)
}

func TestJSONMarshaller_unknownFields_disabled(t *testing.T) {
	msg := scalarsMessage(t)
	msg.SetUnknown(unknownFieldsBytes())
	setField(t, msg, "a_bool", protoreflect.ValueOfBool(true))

	out, err := protox.ToJSONString(msg, protox.WithoutJSONUnknownFields())
	require.NoError(t, err)

	assert.Equal(t, `{"a_bool":true}`, out)
}

func TestJSONMarshaller_unknownFields_useBytesEncoding(t *testing.T) {
	msg := scalarsMessage(t)
	msg.SetUnknown(unknownFieldsBytes())

	out, err := protox.ToJSONString(msg, protox.WithJSONBytesEncoding(protox.BytesEncodingBase64))
	require.NoError(t, err)

	assert.Contains(t, out, `"__unknown_fields_100_with_type_0__":"oAYB"`)
}

func TestJSONMarshaller_unknownFields_repeatedFieldNumber(t *testing.T) {
	var raw []byte
	raw = protowire.AppendTag(raw, 100, protowire.VarintType)
	raw = protowire.AppendVarint(raw, 1)
	raw = protowire.AppendTag(raw, 100, protowire.VarintType)
	raw = protowire.AppendVarint(raw, 2)

	msg := scalarsMessage(t)
	msg.SetUnknown(raw)

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t,
		`{"__unknown_fields_100_with_type_0__":"a00601","__unknown_fields_100_with_type_0_1__":"a00602"}`,
		out,
		"duplicate field numbers must produce distinct member names")
}

func TestJSONMarshaller_unknownFields_malformed(t *testing.T) {
	msg := scalarsMessage(t)
	// A tag announcing a length-delimited field, followed by a length larger than the
	// remaining buffer.
	msg.SetUnknown(protoreflect.RawFields{0xa2, 0x06, 0x7f})

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Contains(t, out, `"__unknown_fields_error__"`)
}

func TestJSONMarshaller_unknownFields_empty(t *testing.T) {
	msg := scalarsMessage(t)
	setField(t, msg, "a_bool", protoreflect.ValueOfBool(true))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, `{"a_bool":true}`, out)
}
