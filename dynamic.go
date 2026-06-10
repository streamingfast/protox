package protox

import (
	"time"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// DynamicAsTimestamppb converts a dynamic protobuf message representing a google.protobuf.Timestamp
// into a *timestamppb.Timestamp. Returns a zero-value timestamp if the message is nil or not a Timestamp.
func DynamicAsTimestamppb(message protoreflect.Message) *timestamppb.Timestamp {
	seconds, nanos := DynamicAsTimestampParts(message)

	return &timestamppb.Timestamp{
		Seconds: seconds,
		Nanos:   int32(nanos),
	}
}

// DynamicAsTimestampTime converts a dynamic protobuf message representing a google.protobuf.Timestamp
// into a time.Time value in UTC. Returns the zero time if the message is nil or not a Timestamp.
func DynamicAsTimestampTime(message protoreflect.Message) (out time.Time) {
	seconds, nanos := DynamicAsTimestampParts(message)
	return time.Unix(seconds, nanos).UTC()
}

// DynamicAsTimestampParts extracts the seconds and nanoseconds fields from a dynamic protobuf message
// representing a google.protobuf.Timestamp. Returns (0, 0) if the message is nil or not a Timestamp.
//
// Field descriptors are resolved by iterating over populated fields rather than caching descriptors,
// because cross-registry descriptor instances (common in Substreams) cause panics when mixed.
func DynamicAsTimestampParts(message protoreflect.Message) (seconds, nanos int64) {
	if message == nil || message.Descriptor().FullName() != "google.protobuf.Timestamp" {
		return
	}

	var foundSeconds, foundNanos bool
	message.Range(func(f protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		switch f.FullName() {
		case "google.protobuf.Timestamp.seconds":
			seconds = value.Int()
			foundSeconds = true
		case "google.protobuf.Timestamp.nanos":
			nanos = value.Int()
			foundNanos = true
		}

		return !foundSeconds || !foundNanos
	})

	return
}
