package protox_test

import (
	"testing"

	"github.com/streamingfast/protox"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func benchmarkMessage() *descriptorpb.FileDescriptorProto {
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("bench.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("bench"),
	}

	for i := range 100 {
		file.MessageType = append(file.MessageType, &descriptorpb.DescriptorProto{
			Name: proto.String("Message" + string(rune('A'+i%26))),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:   proto.String("field"),
				Number: proto.Int32(int32(i + 1)),
				Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:   descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
			}},
		})
	}

	return file
}

func BenchmarkJSONMarshaller(b *testing.B) {
	msg := benchmarkMessage()
	marshaller := protox.NewJSONMarshaller()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := marshaller.Marshal(msg); err != nil {
			b.Fatalf("marshalling failed: %s", err)
		}
	}
}

func BenchmarkProtojson(b *testing.B) {
	msg := benchmarkMessage()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := protojson.Marshal(msg); err != nil {
			b.Fatalf("marshalling failed: %s", err)
		}
	}
}
