package protox_test

import (
	"testing"

	"github.com/streamingfast/protox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestWalkMessageInstanceFields_flat(t *testing.T) {
	ts := &timestamppb.Timestamp{Seconds: 100, Nanos: 200}

	type result struct {
		msgType string
		field   string
	}
	var got []result
	for msg, field := range protox.WalkMessageInstanceFields(ts.ProtoReflect(), nil) {
		got = append(got, result{
			msgType: string(msg.Descriptor().FullName()),
			field:   string(field.Name()),
		})
	}

	assert.Equal(t, []result{
		{msgType: "google.protobuf.Timestamp", field: "seconds"},
		{msgType: "google.protobuf.Timestamp", field: "nanos"},
	}, got)
}

func TestWalkMessageInstanceFields_filter(t *testing.T) {
	ts := &timestamppb.Timestamp{Seconds: 1, Nanos: 2}
	var msgFields []protoreflect.FullName
	var fieldNames []string
	for msg, field := range protox.WalkMessageInstanceFields(ts.ProtoReflect(), func(fd protoreflect.FieldDescriptor) bool {
		return fd.Name() == "nanos" // skip nanos
	}) {
		msgFields = append(msgFields, msg.Descriptor().FullName())
		fieldNames = append(fieldNames, string(field.Name()))
	}

	assert.Equal(t, []protoreflect.FullName{"google.protobuf.Timestamp"}, msgFields)
	assert.Equal(t, []string{"seconds"}, fieldNames)
}

func TestWalkMessageInstanceFields_nested(t *testing.T) {
	// Build dynamic descriptors: Outer { Inner inner = 1; }, Inner { string value = 1; }
	innerFDP := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("inner.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("test"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Inner"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("value"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						JsonName: proto.String("value"),
					},
				},
			},
		},
	}
	outerFDP := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("outer.proto"),
		Syntax:     proto.String("proto3"),
		Package:    proto.String("test"),
		Dependency: []string{"inner.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Outer"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     proto.String("inner"),
						Number:   proto.Int32(1),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						TypeName: proto.String(".test.Inner"),
						JsonName: proto.String("inner"),
					},
				},
			},
		},
	}

	files, err := protodesc.NewFiles(&descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{innerFDP, outerFDP},
	})
	require.NoError(t, err)

	outerDesc, err := files.FindDescriptorByName("test.Outer")
	require.NoError(t, err)
	innerDesc, err := files.FindDescriptorByName("test.Inner")
	require.NoError(t, err)

	outerType := dynamicpb.NewMessageType(outerDesc.(protoreflect.MessageDescriptor))
	innerType := dynamicpb.NewMessageType(innerDesc.(protoreflect.MessageDescriptor))

	// Populate outer with an inner instance
	outer := outerType.New()
	inner := innerType.New()
	inner.Set(inner.Descriptor().Fields().ByName("value"), protoreflect.ValueOfString("hello"))
	outer.Set(outer.Descriptor().Fields().ByName("inner"), protoreflect.ValueOfMessage(inner))

	type result struct{ msgType, field string }
	var got []result
	for msg, field := range protox.WalkMessageInstanceFields(outer, nil) {
		got = append(got, result{string(msg.Descriptor().Name()), string(field.Name())})
	}

	assert.Equal(t, []result{
		{msgType: "Outer", field: "inner"},
		{msgType: "Inner", field: "value"},
	}, got)
}
