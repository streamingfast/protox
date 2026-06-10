package protox

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// EnumValueToString returns the string representation of the given enum value.
func EnumValueToString(enumValue protoreflect.EnumValueDescriptor) string {
	return string(enumValue.Name())
}

// EnumKnownValuesDebugString returns a human-readable string of all known values for the given enum descriptor,
// formatted as "NAME (NUMBER), ..." — useful for error messages and debugging.
func EnumKnownValuesDebugString(enum protoreflect.EnumDescriptor) string {
	values := make([]string, 0, enum.Values().Len())
	for i := range enum.Values().Len() {
		enumValue := enum.Values().Get(i)
		values = append(values, fmt.Sprintf("%s (%d)", EnumValueToString(enumValue), enumValue.Number()))
	}
	return strings.Join(values, ", ")
}
