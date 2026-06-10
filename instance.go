package protox

import (
	"iter"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// WalkMessageInstanceFields walks all fields of a proto message instance recursively,
// yielding (parent message, field descriptor) for every field in the tree.
//
// Unlike WalkMessageFields which walks descriptors, this function walks actual message
// instances and recurses into sub-messages and repeated message elements.
//
// Infinite recursion is prevented by tracking which message types are currently on the
// visit stack (DFS type-stack, not permanent-seen). If a type references itself, the
// walk stops at that edge.
//
// Well-known Google types (google.protobuf.*) are not recursed into.
//
// fieldFilter is called for each field; if it returns true the field is skipped (not yielded, not recursed into).
func WalkMessageInstanceFields(root protoreflect.Message, fieldFilter func(protoreflect.FieldDescriptor) bool) iter.Seq2[protoreflect.Message, protoreflect.FieldDescriptor] {
	return func(yield func(protoreflect.Message, protoreflect.FieldDescriptor) bool) {
		typeStack := make(map[protoreflect.FullName]bool)

		var walk func(msg protoreflect.Message) bool
		walk = func(msg protoreflect.Message) bool {
			if !msg.IsValid() {
				return true
			}
			typeName := msg.Descriptor().FullName()
			if typeStack[typeName] {
				return true
			}
			typeStack[typeName] = true
			defer func() { typeStack[typeName] = false }()

			fields := msg.Descriptor().Fields()
			for i := range fields.Len() {
				field := fields.Get(i)

				if fieldFilter != nil && fieldFilter(field) {
					continue
				}

				if !yield(msg, field) {
					return false
				}

				if field.Kind() != protoreflect.MessageKind {
					continue
				}
				if IsWellKnownGoogleField(field) {
					continue
				}
				// Map fields have map-entry message values accessed via Map(), not Message().
				// Recursing into map entries is not supported; yield the field itself but skip recursion.
				if field.IsMap() {
					continue
				}

				if field.IsList() {
					list := msg.Get(field).List()
					for j := range list.Len() {
						if !walk(list.Get(j).Message()) {
							return false
						}
					}
				} else {
					sub := msg.Get(field).Message()
					if sub.IsValid() {
						if !walk(sub) {
							return false
						}
					}
				}
			}
			return true
		}

		walk(root)
	}
}
