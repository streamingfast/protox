package protox_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// optionalField builds a field using the proto3 `optional` keyword (real presence tracking,
// via a synthetic oneof — protodesc.NewFile requires that pairing) rather than proto3's
// implicit presence, so that setting a field to its zero value still renders it in output.
// The synthetic oneof itself is added by scalarsMessage, which knows every field's position.
func optionalField(name string, number int32, kind descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:           proto.String(name),
		Number:         proto.Int32(number),
		Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:           kind.Enum(),
		Proto3Optional: proto.Bool(true),
	}
}

// scalarsMessage builds `protox.test.scalars.Scalars`, covering every scalar Protobuf kind
// plus an enum, and returns an empty dynamic instance of it.
func scalarsMessage(t *testing.T) *dynamicpb.Message {
	t.Helper()

	fields := []*descriptorpb.FieldDescriptorProto{
		optionalField("a_bool", 1, descriptorpb.FieldDescriptorProto_TYPE_BOOL),
		optionalField("a_string", 2, descriptorpb.FieldDescriptorProto_TYPE_STRING),
		optionalField("a_bytes", 3, descriptorpb.FieldDescriptorProto_TYPE_BYTES),
		optionalField("an_int32", 4, descriptorpb.FieldDescriptorProto_TYPE_INT32),
		optionalField("an_uint32", 5, descriptorpb.FieldDescriptorProto_TYPE_UINT32),
		optionalField("an_int64", 6, descriptorpb.FieldDescriptorProto_TYPE_INT64),
		optionalField("an_uint64", 7, descriptorpb.FieldDescriptorProto_TYPE_UINT64),
		optionalField("a_double", 8, descriptorpb.FieldDescriptorProto_TYPE_DOUBLE),
		optionalField("a_float", 9, descriptorpb.FieldDescriptorProto_TYPE_FLOAT),
		optionalField("a_sfixed64", 10, descriptorpb.FieldDescriptorProto_TYPE_SFIXED64),
		{
			Name:           proto.String("a_step"),
			Number:         proto.Int32(11),
			Label:          descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:           descriptorpb.FieldDescriptorProto_TYPE_ENUM.Enum(),
			TypeName:       proto.String(".protox.test.scalars.Step"),
			Proto3Optional: proto.Bool(true),
		},
	}

	// Every proto3 `optional` field requires its own synthetic oneof; protoc generates one
	// named "_<field>" per field and points OneofIndex at it. We replicate that here so the
	// fields carry real presence instead of proto3's implicit (zero-value-means-absent) one.
	oneofs := make([]*descriptorpb.OneofDescriptorProto, len(fields))
	for i, field := range fields {
		field.OneofIndex = proto.Int32(int32(i))
		oneofs[i] = &descriptorpb.OneofDescriptorProto{Name: proto.String("_" + field.GetName())}
	}

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("protox/test/scalars.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("protox.test.scalars"),
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: proto.String("Step"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: proto.String("STEP_UNKNOWN"), Number: proto.Int32(0)},
				{Name: proto.String("STEP_NEW"), Number: proto.Int32(1)},
			},
		}},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name:      proto.String("Scalars"),
			Field:     fields,
			OneofDecl: oneofs,
		}},
	}

	built, err := protodesc.NewFile(file, nil)
	require.NoError(t, err)

	return dynamicpb.NewMessage(built.Messages().Get(0))
}

// implicitField builds an ordinary proto3 scalar field — no `optional` keyword, so it has
// implicit presence: protoreflect.Message.Has/Range treat the field as absent whenever it
// holds its zero value. This is the common shape of a bare `bool foo = 1;` declaration, as
// opposed to optionalField's explicit-presence `optional` fields.
func implicitField(name string, number int32, kind descriptorpb.FieldDescriptorProto_Type) *descriptorpb.FieldDescriptorProto {
	return &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(name),
		Number: proto.Int32(number),
		Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		Type:   kind.Enum(),
	}
}

// implicitPresenceMessage builds `protox.test.implicit.Implicit`, a message with ordinary
// (non-`optional`) scalar fields, and returns an empty dynamic instance of it. It exists
// alongside scalarsMessage specifically to exercise implicit-presence semantics: a zero-value
// field must be omitted from JSON output, unlike scalarsMessage's explicit-presence fields.
func implicitPresenceMessage(t *testing.T) *dynamicpb.Message {
	t.Helper()

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("protox/test/implicit.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("protox.test.implicit"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Implicit"),
			Field: []*descriptorpb.FieldDescriptorProto{
				implicitField("name", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING),
				implicitField("count", 2, descriptorpb.FieldDescriptorProto_TYPE_INT32),
			},
		}},
	}

	built, err := protodesc.NewFile(file, nil)
	require.NoError(t, err)

	return dynamicpb.NewMessage(built.Messages().Get(0))
}

// setField sets a field by name on a dynamic message, failing the test when the field is
// absent from the descriptor.
func setField(t *testing.T, msg *dynamicpb.Message, name string, value protoreflect.Value) {
	t.Helper()

	field := msg.Descriptor().Fields().ByName(protoreflect.Name(name))
	require.NotNil(t, field, "field %q not found on %s", name, msg.Descriptor().FullName())

	msg.Set(field, value)
}
