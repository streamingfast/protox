package protox_test

import (
	"testing"

	"github.com/streamingfast/protox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// testFileDescriptor builds a single-message file descriptor. Passing a different
// fieldName produces a descriptor that conflicts with a previous one of the same
// message full name.
func testFileDescriptor(t *testing.T, path, packageName, messageName, fieldName string) protoreflect.FileDescriptor {
	t.Helper()

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String(path),
		Syntax:  proto.String("proto3"),
		Package: proto.String(packageName),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String(messageName),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:   proto.String(fieldName),
				Number: proto.Int32(1),
				Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			}},
		}},
	}

	out, err := protodesc.NewFile(file, nil)
	require.NoError(t, err)

	return out
}

func TestRegisterAnyTypes_resolves(t *testing.T) {
	file := testFileDescriptor(t, "protox/test/registry_resolves.proto", "protox.test.resolves", "Sample", "value")
	messageType := dynamicpb.NewMessageType(file.Messages().Get(0))

	protox.RegisterAnyTypes(messageType)

	resolved, err := protox.AnyTypeResolver().FindMessageByURL("type.googleapis.com/protox.test.resolves.Sample")
	require.NoError(t, err)
	assert.Equal(t, protoreflect.FullName("protox.test.resolves.Sample"), resolved.Descriptor().FullName())
}

func TestRegisterAnyTypes_identicalIsNoop(t *testing.T) {
	file := testFileDescriptor(t, "protox/test/registry_identical.proto", "protox.test.identical", "Sample", "value")
	messageType := dynamicpb.NewMessageType(file.Messages().Get(0))

	protox.RegisterAnyTypes(messageType)
	protox.RegisterAnyTypes(messageType)

	resolved, err := protox.AnyTypeResolver().FindMessageByName("protox.test.identical.Sample")
	require.NoError(t, err)
	assert.Equal(t, protoreflect.FullName("protox.test.identical.Sample"), resolved.Descriptor().FullName())
}

func TestRegisterAnyTypes_conflictKeepsFirst(t *testing.T) {
	first := testFileDescriptor(t, "protox/test/registry_conflict_a.proto", "protox.test.conflict", "Sample", "first_field")
	second := testFileDescriptor(t, "protox/test/registry_conflict_b.proto", "protox.test.conflict", "Sample", "second_field")

	protox.RegisterAnyTypes(dynamicpb.NewMessageType(first.Messages().Get(0)))
	protox.RegisterAnyTypes(dynamicpb.NewMessageType(second.Messages().Get(0)))

	resolved, err := protox.AnyTypeResolver().FindMessageByName("protox.test.conflict.Sample")
	require.NoError(t, err)
	assert.Equal(t, protoreflect.Name("first_field"), resolved.Descriptor().Fields().Get(0).Name(),
		"first registration must win on conflict")
}

func TestRegisterAnyTypes_ignoresNil(t *testing.T) {
	assert.NotPanics(t, func() { protox.RegisterAnyTypes(nil) })
}

func TestRegisterAnyFiles_registersNested(t *testing.T) {
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("protox/test/registry_nested.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("protox.test.nested"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Outer"),
			NestedType: []*descriptorpb.DescriptorProto{{
				Name: proto.String("Inner"),
			}},
		}},
	}

	built, err := protodesc.NewFile(file, nil)
	require.NoError(t, err)

	files := new(protoregistry.Files)
	require.NoError(t, files.RegisterFile(built))

	protox.RegisterAnyFiles(files)

	for _, name := range []protoreflect.FullName{"protox.test.nested.Outer", "protox.test.nested.Outer.Inner"} {
		resolved, err := protox.AnyTypeResolver().FindMessageByName(name)
		require.NoError(t, err, "expected %s to be registered", name)
		assert.Equal(t, name, resolved.Descriptor().FullName())
	}
}

func TestRegisterAnyFiles_nilIsNoop(t *testing.T) {
	assert.NotPanics(t, func() { protox.RegisterAnyFiles(nil) })
}

func TestAnyTypeResolver_unknownIsNotFound(t *testing.T) {
	_, err := protox.AnyTypeResolver().FindMessageByURL("type.googleapis.com/protox.test.definitely.Missing")
	assert.ErrorIs(t, err, protoregistry.NotFound)
}

func TestChainTypeResolvers(t *testing.T) {
	file := testFileDescriptor(t, "protox/test/registry_chain.proto", "protox.test.chain", "Sample", "value")
	local := new(protoregistry.Types)
	require.NoError(t, local.RegisterMessage(dynamicpb.NewMessageType(file.Messages().Get(0))))

	chain := protox.ChainTypeResolvers(local, protoregistry.GlobalTypes)

	t.Run("finds in first resolver", func(t *testing.T) {
		resolved, err := chain.FindMessageByName("protox.test.chain.Sample")
		require.NoError(t, err)
		assert.Equal(t, protoreflect.FullName("protox.test.chain.Sample"), resolved.Descriptor().FullName())
	})

	t.Run("falls through to second resolver", func(t *testing.T) {
		// timestamppb self-registers into protoregistry.GlobalTypes at init.
		_ = timestamppb.Now()

		resolved, err := chain.FindMessageByURL("type.googleapis.com/google.protobuf.Timestamp")
		require.NoError(t, err)
		assert.Equal(t, protoreflect.FullName("google.protobuf.Timestamp"), resolved.Descriptor().FullName())
	})

	t.Run("not found in any resolver", func(t *testing.T) {
		_, err := chain.FindMessageByName("protox.test.chain.Absent")
		assert.ErrorIs(t, err, protoregistry.NotFound)
	})

	t.Run("nil resolvers are skipped", func(t *testing.T) {
		withNil := protox.ChainTypeResolvers(nil, local, nil)

		resolved, err := withNil.FindMessageByName("protox.test.chain.Sample")
		require.NoError(t, err)
		assert.Equal(t, protoreflect.FullName("protox.test.chain.Sample"), resolved.Descriptor().FullName())
	})

	t.Run("empty chain is not found", func(t *testing.T) {
		_, err := protox.ChainTypeResolvers().FindMessageByName("anything")
		assert.ErrorIs(t, err, protoregistry.NotFound)
	})
}
