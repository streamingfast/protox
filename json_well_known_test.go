package protox_test

import (
	"math"
	"testing"
	"time"

	"github.com/streamingfast/protox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
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
			// The last second+nanos combination that still fits in a time.Duration's int64
			// nanosecond range (math.MaxInt64 nanoseconds).
			"duration at the positive overflow boundary, still representable",
			&durationpb.Duration{Seconds: 9223372036, Nanos: 854775807},
			`"2562047h47m16.854775807s"`,
			`{"seconds":9223372036,"nanos":854775807}`,
		},
		{
			// One nanosecond past the boundary above: the same seconds value, but a nanos
			// value that overflows int64 nanoseconds, must take the "%ds" fallback rather than
			// wrap into a nonsensical duration.
			"duration one nanosecond past the positive overflow boundary",
			&durationpb.Duration{Seconds: 9223372036, Nanos: 854775808},
			`"9223372036s"`,
			`{"seconds":9223372036,"nanos":854775808}`,
		},
		{
			// The last second+nanos combination that still fits in a time.Duration's int64
			// nanosecond range on the negative side (math.MinInt64 nanoseconds).
			"duration at the negative overflow boundary, still representable",
			&durationpb.Duration{Seconds: -9223372036, Nanos: -854775808},
			`"-2562047h47m16.854775808s"`,
			`{"seconds":-9223372036,"nanos":-854775808}`,
		},
		{
			// One nanosecond past the boundary above on the negative side.
			"duration one nanosecond past the negative overflow boundary",
			&durationpb.Duration{Seconds: -9223372036, Nanos: -854775809},
			`"-9223372036s"`,
			`{"seconds":-9223372036,"nanos":-854775809}`,
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
	// A well-known type nested inside another well-known type renders humanized too: here a
	// google.protobuf.Value (rendered as a plain JSON scalar/array) sits inside a
	// google.protobuf.Struct (rendered as a plain JSON object), reached through
	// marshalStruct -> marshalMap -> marshalSingular -> marshalMessage -> marshalWellKnown.
	value, err := structpb.NewStruct(map[string]any{
		"at":   "2026-08-25T14:03:11Z",
		"tags": []any{"a"},
	})
	require.NoError(t, err)

	out, err := protox.ToJSONString(value)
	require.NoError(t, err)

	assert.Equal(t, `{"at":"2026-08-25T14:03:11Z","tags":["a"]}`, out)
}

func TestJSONMarshaller_wellKnownNestedInRegularMessage(t *testing.T) {
	// A well-known type nested inside an *ordinary*, non-google.protobuf message renders
	// humanized too — the same marshalSingular -> marshalMessage -> marshalWellKnown path as
	// TestJSONMarshaller_wellKnownNested, but with a regular message as the outer container
	// instead of another well-known type. This is the scenario Task 7's Any handling sits
	// right next to, so it is the one most at risk of a silent regression there.
	msg := wellKnownFieldsMessage(t)

	happenedAt := timestamppb.New(time.Date(2026, 8, 25, 14, 3, 11, 0, time.UTC))
	setField(t, msg, "happened_at", protoreflect.ValueOfMessage(happenedAt.ProtoReflect()))

	// A BoolValue wrapper holding false: the case the whole unwrapping default exists for,
	// since a naive presence check would render it as an empty object instead of `false`.
	isActive := wrapperspb.Bool(false)
	setField(t, msg, "is_active", protoreflect.ValueOfMessage(isActive.ProtoReflect()))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, `{"happened_at":"2026-08-25T14:03:11Z","is_active":false}`, out)
}

func TestIsWellKnownGoogleMessage(t *testing.T) {
	assert.True(t, protox.IsWellKnownGoogleMessage(timestamppb.Now().ProtoReflect().Descriptor()))
	assert.False(t, protox.IsWellKnownGoogleMessage(scalarsMessage(t).Descriptor()))
	assert.False(t, protox.IsWellKnownGoogleMessage(nil))
}
