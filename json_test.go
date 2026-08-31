package protox_test

import (
	"bytes"
	"encoding/json/jsontext"
	"math"
	"testing"

	"github.com/streamingfast/protox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestJSONMarshaller_scalars(t *testing.T) {
	msg := scalarsMessage(t)
	setField(t, msg, "a_bool", protoreflect.ValueOfBool(true))
	setField(t, msg, "a_string", protoreflect.ValueOfString("hello"))
	setField(t, msg, "a_bytes", protoreflect.ValueOfBytes([]byte{0xde, 0xad}))
	setField(t, msg, "an_int32", protoreflect.ValueOfInt32(-7))
	setField(t, msg, "an_uint32", protoreflect.ValueOfUint32(7))
	setField(t, msg, "an_int64", protoreflect.ValueOfInt64(math.MaxInt64))
	setField(t, msg, "an_uint64", protoreflect.ValueOfUint64(math.MaxUint64))
	setField(t, msg, "a_double", protoreflect.ValueOfFloat64(1.5))
	setField(t, msg, "a_step", protoreflect.ValueOfEnum(1))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t,
		`{"a_bool":true,"a_string":"hello","a_bytes":"dead","an_int32":-7,"an_uint32":7,`+
			`"an_int64":9223372036854775807,"an_uint64":18446744073709551615,"a_double":1.5,"a_step":"STEP_NEW"}`,
		out)
}

func TestJSONMarshaller_emptyMessage(t *testing.T) {
	out, err := protox.ToJSONString(scalarsMessage(t))
	require.NoError(t, err)

	assert.Equal(t, `{}`, out)
}

func TestJSONMarshaller_nilMessage(t *testing.T) {
	var msg *descriptorpb.FileDescriptorProto

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, `null`, out)
}

func TestJSONMarshaller_fieldOrdering(t *testing.T) {
	msg := scalarsMessage(t)
	setField(t, msg, "a_step", protoreflect.ValueOfEnum(0))
	setField(t, msg, "a_bool", protoreflect.ValueOfBool(false))
	setField(t, msg, "a_string", protoreflect.ValueOfString(""))

	t.Run("declaration order by default", func(t *testing.T) {
		out, err := protox.ToJSONString(msg)
		require.NoError(t, err)
		assert.Equal(t, `{"a_bool":false,"a_string":"","a_step":"STEP_UNKNOWN"}`, out)
	})

	t.Run("alphabetical when requested", func(t *testing.T) {
		out, err := protox.ToJSONString(msg, protox.WithJSONAlphabeticalFields())
		require.NoError(t, err)
		assert.Equal(t, `{"a_bool":false,"a_step":"STEP_UNKNOWN","a_string":""}`, out)
	})
}

func TestJSONMarshaller_implicitPresenceOmitsZeroValue(t *testing.T) {
	// Ordinary (non-`optional`) proto3 scalar fields have implicit presence: a field left at
	// its zero value is indistinguishable from an unset one and protoreflect.Message.Range
	// skips it. This must hold end to end through the marshaller, not just at the descriptor
	// level, since scalarsMessage's fixture uses explicit-presence fields everywhere else.
	msg := implicitPresenceMessage(t)
	setField(t, msg, "count", protoreflect.ValueOfInt32(7))
	// "name" is left at its zero value ("") and must be omitted.

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, `{"count":7}`, out)
}

func TestJSONMarshaller_options(t *testing.T) {
	msg := scalarsMessage(t)
	setField(t, msg, "a_bytes", protoreflect.ValueOfBytes([]byte{0x00, 0x01, 0xff}))
	setField(t, msg, "an_int64", protoreflect.ValueOfInt64(42))
	setField(t, msg, "a_step", protoreflect.ValueOfEnum(1))

	tests := []struct {
		name   string
		opts   []protox.JSONMarshallerOption
		expect string
	}{
		{
			"defaults",
			nil,
			`{"a_bytes":"0001ff","an_int64":42,"a_step":"STEP_NEW"}`,
		},
		{
			"camel case field names",
			[]protox.JSONMarshallerOption{protox.WithJSONFieldCamelCase()},
			`{"aBytes":"0001ff","anInt64":42,"aStep":"STEP_NEW"}`,
		},
		{
			"base58 bytes",
			[]protox.JSONMarshallerOption{protox.WithJSONBytesEncoding(protox.BytesEncodingBase58)},
			`{"a_bytes":"19p","an_int64":42,"a_step":"STEP_NEW"}`,
		},
		{
			"base64 bytes",
			[]protox.JSONMarshallerOption{protox.WithJSONBytesEncoding(protox.BytesEncodingBase64)},
			`{"a_bytes":"AAH/","an_int64":42,"a_step":"STEP_NEW"}`,
		},
		{
			"custom bytes encoder",
			[]protox.JSONMarshallerOption{protox.WithJSONBytesEncoder(func(enc *jsontext.Encoder, data []byte) error {
				return enc.WriteToken(jsontext.Int(int64(len(data))))
			})},
			`{"a_bytes":3,"an_int64":42,"a_step":"STEP_NEW"}`,
		},
		{
			"int64 as string",
			[]protox.JSONMarshallerOption{protox.WithJSONInt64AsString()},
			`{"a_bytes":"0001ff","an_int64":"42","a_step":"STEP_NEW"}`,
		},
		{
			"enums as numbers",
			[]protox.JSONMarshallerOption{protox.WithJSONEnumsAsNumbers()},
			`{"a_bytes":"0001ff","an_int64":42,"a_step":1}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := protox.ToJSONString(msg, test.opts...)
			require.NoError(t, err)
			assert.Equal(t, test.expect, out)
		})
	}
}

func TestJSONMarshaller_unknownEnumNumber(t *testing.T) {
	msg := scalarsMessage(t)
	setField(t, msg, "a_step", protoreflect.ValueOfEnum(404))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, `{"a_step":404}`, out)
}

func TestJSONMarshaller_specialFloats(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		expect string
	}{
		{"nan", math.NaN(), `{"a_double":"NaN"}`},
		{"positive infinity", math.Inf(1), `{"a_double":"Infinity"}`},
		{"negative infinity", math.Inf(-1), `{"a_double":"-Infinity"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			msg := scalarsMessage(t)
			setField(t, msg, "a_double", protoreflect.ValueOfFloat64(test.value))

			out, err := protox.ToJSONString(msg)
			require.NoError(t, err)
			assert.Equal(t, test.expect, out)
		})
	}
}

func TestJSONMarshaller_invalidUTF8(t *testing.T) {
	msg := scalarsMessage(t)
	setField(t, msg, "a_string", protoreflect.ValueOfString("bad\xff"))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, "{\"a_string\":\"bad�\"}", out)
}

func TestJSONMarshaller_entryPoints(t *testing.T) {
	msg := scalarsMessage(t)
	setField(t, msg, "a_bool", protoreflect.ValueOfBool(true))

	marshaller := protox.NewJSONMarshaller()

	t.Run("Marshal", func(t *testing.T) {
		out, err := marshaller.Marshal(msg)
		require.NoError(t, err)
		assert.Equal(t, `{"a_bool":true}`, string(out))
	})

	t.Run("MarshalToString", func(t *testing.T) {
		out, err := marshaller.MarshalToString(msg)
		require.NoError(t, err)
		assert.Equal(t, `{"a_bool":true}`, out)
	})

	t.Run("MarshalWrite", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		require.NoError(t, marshaller.MarshalWrite(buf, msg))
		assert.Equal(t, `{"a_bool":true}`, buf.String())
	})

	t.Run("MarshalEncode", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		require.NoError(t, marshaller.MarshalEncode(jsontext.NewEncoder(buf), msg))
		assert.Equal(t, "{\"a_bool\":true}\n", buf.String())
	})

	t.Run("ToJSON", func(t *testing.T) {
		out, err := protox.ToJSON(msg)
		require.NoError(t, err)
		assert.Equal(t, `{"a_bool":true}`, string(out))
	})
}

func TestJSONMarshaller_indent(t *testing.T) {
	msg := scalarsMessage(t)
	setField(t, msg, "a_bool", protoreflect.ValueOfBool(true))

	out, err := protox.ToJSONString(msg, protox.WithJSONIndent("  "))
	require.NoError(t, err)

	assert.Equal(t, "{\n  \"a_bool\": true\n}", out)
}

func TestJSONMarshaller_invalidIndent(t *testing.T) {
	msg := scalarsMessage(t)
	setField(t, msg, "a_bool", protoreflect.ValueOfBool(true))

	// "- " contains a non-space, non-tab character; jsontext.WithIndent would panic on it,
	// so WithJSONIndent must reject it and fall back to compact output instead of crashing.
	out, err := protox.ToJSONString(msg, protox.WithJSONIndent("- "))
	require.NoError(t, err)

	assert.Equal(t, `{"a_bool":true}`, out)
}

func TestJSONMarshaller_nonProtoValues(t *testing.T) {
	// Raw []byte outside of a proto message still uses the configured bytes encoder.
	out, err := protox.ToJSONString(map[string][]byte{"key": {0xab}})
	require.NoError(t, err)

	assert.Equal(t, `{"key":"ab"}`, out)
}

func TestJSONMarshaller_generatedMessage(t *testing.T) {
	// descriptorpb types are ordinary generated Go types; they must render through the same
	// protoreflect path, not through Go struct reflection.
	msg := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String("block_num"),
		Number: proto.Int32(3),
		Type:   descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum(),
	}

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, `{"name":"block_num","number":3,"type":"TYPE_INT64"}`, out)
}
