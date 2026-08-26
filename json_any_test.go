package protox_test

import (
	"testing"
	"time"

	"github.com/streamingfast/protox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestJSONMarshaller_any_resolvedInline(t *testing.T) {
	payload := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String("block_num"),
		Number: proto.Int32(3),
	}

	wrapped, err := anypb.New(payload)
	require.NoError(t, err)

	out, err := protox.ToJSONString(wrapped)
	require.NoError(t, err)

	assert.Equal(t,
		`{"@type":"type.googleapis.com/google.protobuf.FieldDescriptorProto","name":"block_num","number":3}`,
		out)
}

func TestJSONMarshaller_any_resolvedScalarNests(t *testing.T) {
	wrapped, err := anypb.New(timestamppb.New(time.Date(2026, 8, 25, 14, 3, 11, 0, time.UTC)))
	require.NoError(t, err)

	out, err := protox.ToJSONString(wrapped)
	require.NoError(t, err)

	assert.Equal(t,
		`{"@type":"type.googleapis.com/google.protobuf.Timestamp","value":"2026-08-25T14:03:11Z"}`,
		out)
}

func TestJSONMarshaller_any_withoutTypeURL(t *testing.T) {
	payload := &descriptorpb.FieldDescriptorProto{Name: proto.String("block_num")}

	wrapped, err := anypb.New(payload)
	require.NoError(t, err)

	out, err := protox.ToJSONString(wrapped, protox.WithoutJSONAnyTypeURL())
	require.NoError(t, err)

	assert.Equal(t, `{"name":"block_num"}`, out)
}

func TestJSONMarshaller_any_unresolvedDegrades(t *testing.T) {
	wrapped := &anypb.Any{
		TypeUrl: "type.googleapis.com/protox.test.absent.Missing",
		Value:   []byte{0x0a, 0x02, 0x68, 0x69},
	}

	out, err := protox.ToJSONString(wrapped)
	require.NoError(t, err)

	assert.Equal(t,
		`{"@type":"type.googleapis.com/protox.test.absent.Missing","@error":"no type registered for URL","value":"0a026869"}`,
		out)
}

func TestJSONMarshaller_any_unresolvedStrictErrors(t *testing.T) {
	wrapped := &anypb.Any{
		TypeUrl: "type.googleapis.com/protox.test.absent.Missing",
		Value:   []byte{0x0a},
	}

	_, err := protox.ToJSONString(wrapped, protox.WithJSONStrictAny())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protox.test.absent.Missing")
}

func TestJSONMarshaller_any_usesProtoxRegistry(t *testing.T) {
	file := testFileDescriptor(t, "protox/test/any_registry.proto", "protox.test.anyregistry", "Sample", "value")
	files := new(protoregistry.Files)
	require.NoError(t, files.RegisterFile(file))

	protox.RegisterAnyFiles(files)

	// `value` is field 1, a string: 0x0a length-delimited, length 2, "hi".
	wrapped := &anypb.Any{
		TypeUrl: "type.googleapis.com/protox.test.anyregistry.Sample",
		Value:   []byte{0x0a, 0x02, 0x68, 0x69},
	}

	out, err := protox.ToJSONString(wrapped)
	require.NoError(t, err)

	assert.Equal(t,
		`{"@type":"type.googleapis.com/protox.test.anyregistry.Sample","value":"hi"}`,
		out)
}

func TestJSONMarshaller_any_resolverOverrideDropsDefaults(t *testing.T) {
	// descriptorpb self-registers into protoregistry.GlobalTypes, so an empty override
	// resolver must make this Any unresolvable.
	wrapped, err := anypb.New(&descriptorpb.FieldDescriptorProto{Name: proto.String("x")})
	require.NoError(t, err)

	out, err := protox.ToJSONString(wrapped, protox.WithJSONAnyResolverOverride(new(protoregistry.Types)))
	require.NoError(t, err)

	assert.Contains(t, out, `"@error":"no type registered for URL"`)
}

func TestJSONMarshaller_any_extraResolverIsConsulted(t *testing.T) {
	file := testFileDescriptor(t, "protox/test/any_extra.proto", "protox.test.anyextra", "Sample", "value")
	extra := new(protoregistry.Types)
	require.NoError(t, extra.RegisterMessage(dynamicpbMessageType(t, file)))

	wrapped := &anypb.Any{
		TypeUrl: "type.googleapis.com/protox.test.anyextra.Sample",
		Value:   []byte{0x0a, 0x02, 0x68, 0x69},
	}

	out, err := protox.ToJSONString(wrapped, protox.WithJSONAnyResolver(extra))
	require.NoError(t, err)

	assert.Equal(t,
		`{"@type":"type.googleapis.com/protox.test.anyextra.Sample","value":"hi"}`,
		out)
}

// TestJSONMarshaller_any_bothResolverOptionsSet exercises buildResolver's third branch,
// left untested by Task 3: when both WithJSONAnyResolver and WithJSONAnyResolverOverride are
// supplied, the override must win outright — the additive resolver, the protox registry, and
// protoregistry.GlobalTypes must all be bypassed — and construction must not panic.
func TestJSONMarshaller_any_bothResolverOptionsSet(t *testing.T) {
	file := testFileDescriptor(t, "protox/test/any_both.proto", "protox.test.anyboth", "Sample", "value")
	additive := new(protoregistry.Types)
	require.NoError(t, additive.RegisterMessage(dynamicpbMessageType(t, file)))

	override := new(protoregistry.Types)

	wrapped := &anypb.Any{
		TypeUrl: "type.googleapis.com/protox.test.anyboth.Sample",
		Value:   []byte{0x0a, 0x02, 0x68, 0x69},
	}

	var out string
	var err error
	assert.NotPanics(t, func() {
		out, err = protox.ToJSONString(wrapped,
			protox.WithJSONAnyResolver(additive),
			protox.WithJSONAnyResolverOverride(override),
		)
	})
	require.NoError(t, err)

	// The override is empty, so even though additive holds the type, resolution must fail:
	// the override replaces the whole chain rather than being consulted alongside it.
	assert.Contains(t, out, `"@error":"no type registered for URL"`)
}

// TestJSONMarshaller_any_nestedAny proves that an Any resolving to another Any terminates
// and renders sensibly: the outer Any nests the inner Any under "value" (an Any is itself a
// humanized well-known type), and marshalAny recurses to resolve and render the inner Any in
// turn. Recursion depth tracks the actual nesting present in the data, so it terminates for
// any finite input the same way ordinary nested-message recursion does.
func TestJSONMarshaller_any_nestedAny(t *testing.T) {
	inner, err := anypb.New(timestamppb.New(time.Date(2026, 8, 25, 14, 3, 11, 0, time.UTC)))
	require.NoError(t, err)

	outer, err := anypb.New(inner)
	require.NoError(t, err)

	out, err := protox.ToJSONString(outer)
	require.NoError(t, err)

	assert.Equal(t,
		`{"@type":"type.googleapis.com/google.protobuf.Any","value":{"@type":"type.googleapis.com/google.protobuf.Timestamp","value":"2026-08-25T14:03:11Z"}}`,
		out)
}

func TestJSONMarshaller_any_wellKnownAsProto(t *testing.T) {
	wrapped, err := anypb.New(&descriptorpb.FieldDescriptorProto{Name: proto.String("x")})
	require.NoError(t, err)

	out, err := protox.ToJSONString(wrapped, protox.WithJSONWellKnownAsProto())
	require.NoError(t, err)

	assert.Equal(t,
		`{"type_url":"type.googleapis.com/google.protobuf.FieldDescriptorProto","value":"0a0178"}`,
		out)
}
