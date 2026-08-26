package protox_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/streamingfast/protox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// TestJSONMarshaller_dynamicGeneratedParity is the headline property: a generated Go message
// and a dynamicpb message built from the same descriptor and the same wire bytes must render
// to byte-identical JSON. firehose-core's marshaller cannot satisfy this because it only
// intercepts *dynamicpb.Message and lets generated types fall through to Go struct
// reflection.
func TestJSONMarshaller_dynamicGeneratedParity(t *testing.T) {
	tests := []struct {
		name string
		in   proto.Message
	}{
		{
			"file descriptor proto with enums and nested messages",
			&descriptorpb.FileDescriptorProto{
				Name:    proto.String("sample.proto"),
				Syntax:  proto.String("proto3"),
				Package: proto.String("sample"),
				MessageType: []*descriptorpb.DescriptorProto{{
					Name: proto.String("Block"),
					Field: []*descriptorpb.FieldDescriptorProto{{
						Name:   proto.String("number"),
						Number: proto.Int32(1),
						Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:   descriptorpb.FieldDescriptorProto_TYPE_UINT64.Enum(),
					}},
				}},
			},
		},
		{"timestamp", timestamppb.New(time.Date(2026, 8, 25, 14, 3, 11, 250000000, time.UTC))},
		{"duration", durationpb.New(90 * time.Second)},
		{"string wrapper", wrapperspb.String("gopher")},
		{"bytes wrapper", wrapperspb.Bytes([]byte{0x01, 0x02})},
	}

	optionSets := map[string][]protox.JSONMarshallerOption{
		"defaults":            nil,
		"camel case":          {protox.WithJSONFieldCamelCase()},
		"alphabetical":        {protox.WithJSONAlphabeticalFields()},
		"enums as numbers":    {protox.WithJSONEnumsAsNumbers()},
		"int64 as string":     {protox.WithJSONInt64AsString()},
		"base58 bytes":        {protox.WithJSONBytesEncoding(protox.BytesEncodingBase58)},
		"well known as proto": {protox.WithJSONWellKnownAsProto()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := proto.Marshal(test.in)
			require.NoError(t, err)

			dynamic := dynamicpb.NewMessage(test.in.ProtoReflect().Descriptor())
			require.NoError(t, proto.Unmarshal(raw, dynamic))

			for optionName, opts := range optionSets {
				t.Run(optionName, func(t *testing.T) {
					marshaller := protox.NewJSONMarshaller(opts...)

					fromGenerated, err := marshaller.MarshalToString(test.in)
					require.NoError(t, err)

					fromDynamic, err := marshaller.MarshalToString(dynamic)
					require.NoError(t, err)

					assert.Equal(t, fromGenerated, fromDynamic,
						"generated and dynamic rendering must be identical")
				})
			}
		})
	}
}

// TestJSONMarshaller_nonStringMapKeyParity closes a coverage gap flagged in Task 4's review:
// every other map fixture in this package (see compositesMessage in json_testdata_test.go)
// uses a string-typed key, so marshalMap's reliance on protoreflect.MapKey.String() to build
// JSON object member names was never exercised for int32 or bool keys.
//
// There is no protoc-gen-go generated type in this module's dependency graph with a
// non-string-keyed map field, so this cannot pair a generated message against a dynamicpb
// message the way TestJSONMarshaller_dynamicGeneratedParity does. Instead both sides are
// dynamicpb: one built and populated directly ("source"), the other produced by marshalling
// it to wire bytes and unmarshalling into a fresh instance of the same descriptor
// ("roundTripped"), mirroring the wire round trip the headline parity test performs.
// marshalMap never switches on the concrete Go type — it only calls protoreflect.Map and
// protoreflect.MapKey methods — so this still exercises the exact code path a real generated
// type would hit.
//
// The assertion is against a literal expected string, not just equality between the two
// renderings: two identically wrong renderings would still agree with each other.
func TestJSONMarshaller_nonStringMapKeyParity(t *testing.T) {
	source := mapKeysMessage(t)

	byInt32 := source.Mutable(source.Descriptor().Fields().ByName("by_int32")).Map()
	byInt32.Set(protoreflect.ValueOfInt32(9).MapKey(), protoreflect.ValueOfString("nine"))
	byInt32.Set(protoreflect.ValueOfInt32(10).MapKey(), protoreflect.ValueOfString("ten"))
	byInt32.Set(protoreflect.ValueOfInt32(2).MapKey(), protoreflect.ValueOfString("two"))

	byBool := source.Mutable(source.Descriptor().Fields().ByName("by_bool")).Map()
	byBool.Set(protoreflect.ValueOfBool(true).MapKey(), protoreflect.ValueOfString("yes"))
	byBool.Set(protoreflect.ValueOfBool(false).MapKey(), protoreflect.ValueOfString("no"))

	raw, err := proto.Marshal(source)
	require.NoError(t, err)

	roundTripped := dynamicpb.NewMessage(source.Descriptor())
	require.NoError(t, proto.Unmarshal(raw, roundTripped))

	marshaller := protox.NewJSONMarshaller()

	fromSource, err := marshaller.MarshalToString(source)
	require.NoError(t, err)

	fromRoundTripped, err := marshaller.MarshalToString(roundTripped)
	require.NoError(t, err)

	require.Equal(t, fromSource, fromRoundTripped, "wire round trip must not change rendering")

	// Integer keys sort lexicographically by their stringified form, not numerically:
	// "10" < "2" < "9". Bool keys stringify to "false"/"true", which happens to also sort in
	// the natural order. Documenting this explicitly because it is surprising and not
	// something the implementation should silently change without a deliberate decision.
	assert.Equal(t,
		`{"by_int32":{"10":"ten","2":"two","9":"nine"},"by_bool":{"false":"no","true":"yes"}}`,
		fromSource)
}

func TestJSONMarshaller_concurrentUse(t *testing.T) {
	marshaller := protox.NewJSONMarshaller()

	// testFileDescriptor calls t.Helper() and require, neither of which is safe to call from
	// a non-test goroutine, so the descriptors are built here on the test goroutine and the
	// background goroutine below only calls protox.RegisterAnyTypes. Each of the 50 types
	// gets a genuinely unique full name (index-suffixed, not letter-cycled modulo 26) so none
	// of the registrations collides with an earlier one from the same run — a collision would
	// silently exercise RegisterAnyTypes' keep-first-and-warn branch by accident rather than
	// by intent, and that branch already has dedicated coverage in registry_test.go.
	messageTypes := make([]protoreflect.MessageType, 50)
	for i := range 50 {
		file := testFileDescriptor(t,
			fmt.Sprintf("protox/test/concurrent_%02d.proto", i),
			fmt.Sprintf("protox.test.concurrent%02d", i),
			"Sample", "value")
		messageTypes[i] = dynamicpb.NewMessageType(file.Messages().Get(0))
	}

	// The value every marshalling goroutine renders is a google.protobuf.Any pointing at the
	// *last-registered* type's full name — the same protox registry the writer goroutine below
	// is populating. This is what actually exercises anyTypesMutex under contention:
	// marshalAny -> resolveAny -> m.resolver.FindMessageByURL takes the registry's read lock,
	// concurrently with RegisterAnyTypes' write lock, for every one of the 50 marshal calls
	// below. Targeting the *last* registered type (rather than the first) gives resolution a
	// chance to depend on the writer's progress instead of succeeding trivially from the
	// writer's very first loop iteration.
	//
	// An earlier version of this test marshalled a plain *structpb.Struct, which never reaches
	// the resolver at all — marshalStruct has no reason to consult m.resolver — so the
	// registry mutex only ever saw sequential single-goroutine writes and that test would have
	// kept passing even if the mutex were deleted from registry.go outright.
	//
	// Whether a given marshal call observes the type as already registered is a race against
	// the writer goroutine's progress that this test deliberately does not control or force:
	// depending on how fast the write loop runs relative to goroutine scheduling on a given
	// machine, every read may consistently land on one side or actually split between both.
	// Either way is a legitimate outcome, so the assertion below accepts either rendering
	// rather than pinning one exact string, which would risk flaking under -race or on a
	// different machine.
	typeURL := "type.googleapis.com/" + string(messageTypes[len(messageTypes)-1].Descriptor().FullName())
	// Field 1, a string named "value", wire-encoded as a length-delimited "hi" — the same
	// encoding json_any_test.go uses for its "Sample"/"value" fixture.
	payload := []byte{0x0a, 0x02, 0x68, 0x69}
	value := &anypb.Any{TypeUrl: typeURL, Value: payload}

	resolvedJSON := fmt.Sprintf(`{"@type":%q,"value":"hi"}`, typeURL)
	degradedJSON := fmt.Sprintf(`{"@type":%q,"@error":"no type registered for URL","value":"0a026869"}`, typeURL)
	acceptable := []string{resolvedJSON, degradedJSON}

	done := make(chan struct{})

	// Registering types concurrently with marshalling exercises the registry mutex.
	go func() {
		defer close(done)
		for _, messageType := range messageTypes {
			protox.RegisterAnyTypes(messageType)
		}
	}()

	results := make(chan string, 50)
	for range 50 {
		go func() {
			out, err := marshaller.MarshalToString(value)
			assert.NoError(t, err)
			results <- out
		}()
	}

	for range 50 {
		assert.Contains(t, acceptable, <-results)
	}

	<-done
}
