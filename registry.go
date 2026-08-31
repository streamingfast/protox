package protox

import (
	"sync"

	"go.uber.org/zap"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

var (
	anyTypesMutex sync.RWMutex
	anyTypes      = new(protoregistry.Types)
)

// RegisterAnyTypes records message types usable as `google.protobuf.Any` payloads by the
// JSON marshaller and by anyone calling AnyTypeResolver.
//
// The registry is private to protox: it is never protoregistry.GlobalTypes, so registering
// here has no effect on ordinary proto unmarshalling elsewhere in the binary.
//
// Registering the same message type twice is a no-op. Registering two different descriptors
// under one full name keeps the first and logs a warning — this is a display library, a
// rendering defect beats a crash. Nil types are ignored.
//
// Intended to be called from an init function:
//
//	func init() { protox.RegisterAnyTypes(myTypes...) }
func RegisterAnyTypes(mts ...protoreflect.MessageType) {
	anyTypesMutex.Lock()
	defer anyTypesMutex.Unlock()

	for _, mt := range mts {
		if mt == nil {
			continue
		}

		descriptor := mt.Descriptor()
		name := descriptor.FullName()

		if previous, err := anyTypes.FindMessageByName(name); err == nil {
			if previous.Descriptor() != descriptor {
				zlog.Warn("conflicting Any type registration, keeping the first one",
					zap.String("full_name", string(name)),
					zap.String("kept_from", descriptorFilePath(previous.Descriptor())),
					zap.String("ignored_from", descriptorFilePath(descriptor)),
				)
			}

			continue
		}

		if err := anyTypes.RegisterMessage(mt); err != nil {
			zlog.Warn("unable to register Any type",
				zap.String("full_name", string(name)),
				zap.Error(err),
			)
		}
	}
}

// RegisterAnyFiles registers every message descriptor found in files, including nested
// messages, as a dynamicpb message type. Map entry pseudo-messages are skipped.
//
// This is the entry point for callers holding descriptors parsed from a FileDescriptorSet
// with no generated Go type available.
func RegisterAnyFiles(files *protoregistry.Files) {
	if files == nil {
		return
	}

	var types []protoreflect.MessageType
	files.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		collectMessageTypes(file.Messages(), &types)
		return true
	})

	RegisterAnyTypes(types...)
}

func collectMessageTypes(messages protoreflect.MessageDescriptors, out *[]protoreflect.MessageType) {
	for i := range messages.Len() {
		message := messages.Get(i)
		if message.IsMapEntry() {
			continue
		}

		*out = append(*out, dynamicpb.NewMessageType(message))
		collectMessageTypes(message.Messages(), out)
	}
}

// AnyTypeResolver returns a concurrency-safe view over the types registered through
// RegisterAnyTypes and RegisterAnyFiles.
//
// It is the standard protoregistry.MessageTypeResolver interface, so it composes with
// anything in google.golang.org/protobuf. To resolve an Any outside of JSON rendering:
//
//	message, err := anypb.UnmarshalNew(any, proto.UnmarshalOptions{})
//
// or, to look the type up first:
//
//	messageType, err := protox.AnyTypeResolver().FindMessageByURL(any.TypeUrl)
func AnyTypeResolver() protoregistry.MessageTypeResolver {
	return anyTypeResolver{}
}

type anyTypeResolver struct{}

func (anyTypeResolver) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
	anyTypesMutex.RLock()
	defer anyTypesMutex.RUnlock()

	return anyTypes.FindMessageByName(name)
}

func (anyTypeResolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	anyTypesMutex.RLock()
	defer anyTypesMutex.RUnlock()

	return anyTypes.FindMessageByURL(url)
}

// ChainTypeResolvers returns a resolver consulting each received resolver in order,
// returning the first match and protoregistry.NotFound when none matches. Nil resolvers are
// skipped.
//
// google.golang.org/protobuf provides no chaining implementation of its own.
func ChainTypeResolvers(resolvers ...protoregistry.MessageTypeResolver) protoregistry.MessageTypeResolver {
	out := make(chainedTypeResolver, 0, len(resolvers))
	for _, resolver := range resolvers {
		if resolver != nil {
			out = append(out, resolver)
		}
	}

	return out
}

type chainedTypeResolver []protoregistry.MessageTypeResolver

func (c chainedTypeResolver) FindMessageByName(name protoreflect.FullName) (protoreflect.MessageType, error) {
	for _, resolver := range c {
		if found, err := resolver.FindMessageByName(name); err == nil {
			return found, nil
		}
	}

	return nil, protoregistry.NotFound
}

func (c chainedTypeResolver) FindMessageByURL(url string) (protoreflect.MessageType, error) {
	for _, resolver := range c {
		if found, err := resolver.FindMessageByURL(url); err == nil {
			return found, nil
		}
	}

	return nil, protoregistry.NotFound
}

func descriptorFilePath(descriptor protoreflect.Descriptor) string {
	if file := descriptor.ParentFile(); file != nil {
		return file.Path()
	}

	return "<unknown>"
}
