package protox_test

import (
	"testing"

	"github.com/streamingfast/protox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestJSONMarshaller_repeatedScalars(t *testing.T) {
	msg := compositesMessage(t)
	names := msg.Mutable(msg.Descriptor().Fields().ByName("names")).List()
	names.Append(protoreflect.ValueOfString("alice"))
	names.Append(protoreflect.ValueOfString("bob"))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, `{"names":["alice","bob"]}`, out)
}

func TestJSONMarshaller_repeatedMessages(t *testing.T) {
	msg := compositesMessage(t)
	leaves := msg.Mutable(msg.Descriptor().Fields().ByName("leaves")).List()
	leaves.Append(protoreflect.ValueOfMessage(newLeaf(t, msg, "first")))
	leaves.Append(protoreflect.ValueOfMessage(newLeaf(t, msg, "second")))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, `{"leaves":[{"label":"first"},{"label":"second"}]}`, out)
}

func TestJSONMarshaller_emptyRepeatedIsOmitted(t *testing.T) {
	msg := compositesMessage(t)
	msg.Mutable(msg.Descriptor().Fields().ByName("names"))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	// protoreflect.Message.Has documents that "a repeated field is populated if it is
	// non-empty" — Mutable() lazily allocates the list but does not flip presence, so an
	// empty repeated field never reaches Range() and is omitted, just like an untouched one.
	assert.Equal(t, `{}`, out, "an explicitly mutated but still-empty list is omitted")
}

func TestJSONMarshaller_scalarMapSortsKeys(t *testing.T) {
	msg := compositesMessage(t)
	scores := msg.Mutable(msg.Descriptor().Fields().ByName("scores")).Map()
	scores.Set(protoreflect.ValueOfString("zulu").MapKey(), protoreflect.ValueOfInt32(1))
	scores.Set(protoreflect.ValueOfString("alpha").MapKey(), protoreflect.ValueOfInt32(2))
	scores.Set(protoreflect.ValueOfString("mike").MapKey(), protoreflect.ValueOfInt32(3))

	// Run repeatedly: Go map iteration order is randomized, so a single pass could pass by
	// luck if sorting were missing.
	for range 20 {
		out, err := protox.ToJSONString(msg)
		require.NoError(t, err)
		assert.Equal(t, `{"scores":{"alpha":2,"mike":3,"zulu":1}}`, out)
	}
}

func TestJSONMarshaller_messageMap(t *testing.T) {
	msg := compositesMessage(t)
	leaves := msg.Mutable(msg.Descriptor().Fields().ByName("leaf_by_name")).Map()
	leaves.Set(protoreflect.ValueOfString("b").MapKey(), protoreflect.ValueOfMessage(newLeaf(t, msg, "beta")))
	leaves.Set(protoreflect.ValueOfString("a").MapKey(), protoreflect.ValueOfMessage(newLeaf(t, msg, "alpha")))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, `{"leaf_by_name":{"a":{"label":"alpha"},"b":{"label":"beta"}}}`, out)
}

func TestJSONMarshaller_nestedMessage(t *testing.T) {
	msg := compositesMessage(t)
	msg.Set(msg.Descriptor().Fields().ByName("child"), protoreflect.ValueOfMessage(newLeaf(t, msg, "kid")))

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Equal(t, `{"child":{"label":"kid"}}`, out)
}

func TestJSONMarshaller_deeplyNested(t *testing.T) {
	msg := compositesMessage(t)
	leaves := msg.Mutable(msg.Descriptor().Fields().ByName("leaves")).List()
	for i := range 50 {
		leaves.Append(protoreflect.ValueOfMessage(newLeaf(t, msg, string(rune('a'+i%26)))))
	}

	out, err := protox.ToJSONString(msg)
	require.NoError(t, err)

	assert.Contains(t, out, `{"label":"a"}`)
}
