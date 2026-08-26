package protox_test

import (
	"math"
	"testing"
	"time"

	"github.com/streamingfast/protox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestJSONMarshaller_wellKnown(t *testing.T) {
	structValue, err := structpb.NewStruct(map[string]any{"chain": "eth", "depth": 3})
	require.NoError(t, err)

	listValue, err := structpb.NewList([]any{"a", "b"})
	require.NoError(t, err)

	tests := []struct {
		name        string
		in          any
		expect      string
		expectProto string
	}{
		{
			"timestamp",
			timestamppb.New(time.Date(2026, 8, 25, 14, 3, 11, 0, time.UTC)),
			`"2026-08-25T14:03:11Z"`,
			`{"seconds":1787666591}`,
		},
		{
			"timestamp with nanos",
			timestamppb.New(time.Date(2026, 8, 25, 14, 3, 11, 500000000, time.UTC)),
			`"2026-08-25T14:03:11.5Z"`,
			`{"seconds":1787666591,"nanos":500000000}`,
		},
		{
			"duration",
			durationpb.New(1500 * time.Millisecond),
			`"1.5s"`,
			`{"seconds":1,"nanos":500000000}`,
		},
		{
			"negative duration",
			durationpb.New(-2 * time.Minute),
			`"-2m0s"`,
			`{"seconds":-120}`,
		},
		{
			"string wrapper",
			wrapperspb.String("gopher"),
			`"gopher"`,
			`{"value":"gopher"}`,
		},
		{
			"bool wrapper holding false",
			wrapperspb.Bool(false),
			`false`,
			`{}`,
		},
		{
			"int64 wrapper",
			wrapperspb.Int64(42),
			`42`,
			`{"value":42}`,
		},
		{
			"bytes wrapper",
			wrapperspb.Bytes([]byte{0xab, 0xcd}),
			`"abcd"`,
			`{"value":"abcd"}`,
		},
		{
			"double wrapper holding NaN",
			wrapperspb.Double(math.NaN()),
			`"NaN"`,
			`{"value":"NaN"}`,
		},
		{
			"struct",
			structValue,
			`{"chain":"eth","depth":3}`,
			`{"fields":{"chain":{"string_value":"eth"},"depth":{"number_value":3}}}`,
		},
		{
			"empty struct",
			&structpb.Struct{},
			`{}`,
			`{}`,
		},
		{
			"value holding a string",
			structpb.NewStringValue("hello"),
			`"hello"`,
			`{"string_value":"hello"}`,
		},
		{
			"value holding null",
			structpb.NewNullValue(),
			`null`,
			`{"null_value":"NULL_VALUE"}`,
		},
		{
			"value holding a bool",
			structpb.NewBoolValue(true),
			`true`,
			`{"bool_value":true}`,
		},
		{
			"list value",
			listValue,
			`["a","b"]`,
			`{"values":[{"string_value":"a"},{"string_value":"b"}]}`,
		},
		{
			"empty",
			&emptypb.Empty{},
			`{}`,
			`{}`,
		},
		{
			"field mask",
			&fieldmaskpb.FieldMask{Paths: []string{"block.number", "header"}},
			`["block.number","header"]`,
			`{"paths":["block.number","header"]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := protox.ToJSONString(test.in)
			require.NoError(t, err)
			assert.Equal(t, test.expect, out, "humanized form")

			outProto, err := protox.ToJSONString(test.in, protox.WithJSONWellKnownAsProto())
			require.NoError(t, err)
			assert.Equal(t, test.expectProto, outProto, "proto-shaped form")
		})
	}
}

func TestJSONMarshaller_wellKnownNested(t *testing.T) {
	// A well-known type nested inside another message renders humanized too.
	value, err := structpb.NewStruct(map[string]any{
		"at":   "2026-08-25T14:03:11Z",
		"tags": []any{"a"},
	})
	require.NoError(t, err)

	out, err := protox.ToJSONString(value)
	require.NoError(t, err)

	assert.Equal(t, `{"at":"2026-08-25T14:03:11Z","tags":["a"]}`, out)
}

func TestIsWellKnownGoogleMessage(t *testing.T) {
	assert.True(t, protox.IsWellKnownGoogleMessage(timestamppb.Now().ProtoReflect().Descriptor()))
	assert.False(t, protox.IsWellKnownGoogleMessage(scalarsMessage(t).Descriptor()))
	assert.False(t, protox.IsWellKnownGoogleMessage(nil))
}
