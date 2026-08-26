package protox_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	_ "google.golang.org/protobuf/types/known/wrapperspb"
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

// wellKnownFieldsMessage builds `protox.test.wellknown.Carrier`, an ordinary message — not
// itself under the google.protobuf package — carrying two fields whose types are google.protobuf
// well-known types: a Timestamp and a BoolValue wrapper. It exists to prove that well-known
// humanization recurses into fields of a regular message, not only into other well-known types
// (a google.protobuf.Struct wrapping a google.protobuf.Value, for instance).
//
// protodesc.NewFile is given protoregistry.GlobalFiles as its resolver so it can look up the
// google/protobuf/timestamp.proto and google/protobuf/wrappers.proto dependencies declared
// below; those files are registered globally as a side effect of importing timestamppb and
// wrapperspb, which this file does.
func wellKnownFieldsMessage(t *testing.T) *dynamicpb.Message {
	t.Helper()

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("protox/test/wellknown.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("protox.test.wellknown"),
		Dependency: []string{
			"google/protobuf/timestamp.proto",
			"google/protobuf/wrappers.proto",
		},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Carrier"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:     proto.String("happened_at"),
					Number:   proto.Int32(1),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
					TypeName: proto.String(".google.protobuf.Timestamp"),
				},
				{
					Name:     proto.String("is_active"),
					Number:   proto.Int32(2),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
					TypeName: proto.String(".google.protobuf.BoolValue"),
				},
			},
		}},
	}

	built, err := protodesc.NewFile(file, protoregistry.GlobalFiles)
	require.NoError(t, err)

	return dynamicpb.NewMessage(built.Messages().Get(0))
}

// compositesMessage builds `protox.test.composites.Composites`, covering repeated scalars,
// repeated messages, a scalar-valued map and a message-valued map, and returns an empty
// dynamic instance of it.
func compositesMessage(t *testing.T) *dynamicpb.Message {
	t.Helper()

	mapEntry := func(name string, valueType descriptorpb.FieldDescriptorProto_Type, valueTypeName string) *descriptorpb.DescriptorProto {
		value := optionalField("value", 2, valueType)
		if valueTypeName != "" {
			value.TypeName = proto.String(valueTypeName)
		}

		return &descriptorpb.DescriptorProto{
			Name:    proto.String(name),
			Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
			Field: []*descriptorpb.FieldDescriptorProto{
				optionalField("key", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING),
				value,
			},
		}
	}

	repeated := func(name string, number int32, kind descriptorpb.FieldDescriptorProto_Type, typeName string) *descriptorpb.FieldDescriptorProto {
		field := &descriptorpb.FieldDescriptorProto{
			Name:   proto.String(name),
			Number: proto.Int32(number),
			Label:  descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
			Type:   kind.Enum(),
		}
		if typeName != "" {
			field.TypeName = proto.String(typeName)
		}

		return field
	}

	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("protox/test/composites.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("protox.test.composites"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Leaf"),
				Field: []*descriptorpb.FieldDescriptorProto{
					optionalField("label", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING),
				},
			},
			{
				Name: proto.String("Composites"),
				NestedType: []*descriptorpb.DescriptorProto{
					mapEntry("ScoresEntry", descriptorpb.FieldDescriptorProto_TYPE_INT32, ""),
					mapEntry("LeafByNameEntry", descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".protox.test.composites.Leaf"),
				},
				Field: []*descriptorpb.FieldDescriptorProto{
					repeated("names", 1, descriptorpb.FieldDescriptorProto_TYPE_STRING, ""),
					repeated("leaves", 2, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".protox.test.composites.Leaf"),
					repeated("scores", 3, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".protox.test.composites.Composites.ScoresEntry"),
					repeated("leaf_by_name", 4, descriptorpb.FieldDescriptorProto_TYPE_MESSAGE, ".protox.test.composites.Composites.LeafByNameEntry"),
					{
						Name:     proto.String("child"),
						Number:   proto.Int32(5),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: proto.String(".protox.test.composites.Leaf"),
					},
				},
			},
		},
	}

	built, err := protodesc.NewFile(file, nil)
	require.NoError(t, err)

	return dynamicpb.NewMessage(built.Messages().ByName("Composites"))
}

// newLeaf builds a `protox.test.composites.Leaf` sharing msg's descriptor pool.
func newLeaf(t *testing.T, msg *dynamicpb.Message, label string) protoreflect.Message {
	t.Helper()

	descriptor := msg.Descriptor().ParentFile().Messages().ByName("Leaf")
	require.NotNil(t, descriptor)

	leaf := dynamicpb.NewMessage(descriptor)
	leaf.Set(descriptor.Fields().ByName("label"), protoreflect.ValueOfString(label))

	return leaf
}

// dynamicpbMessageType returns a dynamic message type for the first message of file.
func dynamicpbMessageType(t *testing.T, file protoreflect.FileDescriptor) protoreflect.MessageType {
	t.Helper()

	return dynamicpb.NewMessageType(file.Messages().Get(0))
}
