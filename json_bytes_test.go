package protox_test

import (
	"bytes"
	"encoding/json/jsontext"
	"testing"

	"github.com/streamingfast/protox"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBytesEncoding(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		expect      protox.BytesEncoding
		expectError string
	}{
		{"hex lowercase", "hex", protox.BytesEncodingHex, ""},
		{"hex mixed case", "HeX", protox.BytesEncodingHex, ""},
		{"base58", "base58", protox.BytesEncodingBase58, ""},
		{"base64", "BASE64", protox.BytesEncodingBase64, ""},
		{"unknown", "rot13", "", `unsupported bytes encoding "rot13", accepted values are "hex", "base58" and "base64"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := protox.ParseBytesEncoding(test.in)
			if test.expectError != "" {
				require.Error(t, err)
				assert.Equal(t, test.expectError, err.Error())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expect, out)
		})
	}
}

func TestEncodeBytes(t *testing.T) {
	data := []byte{0x00, 0x01, 0xff}

	assert.Equal(t, "0001ff", protox.EncodeBytes(protox.BytesEncodingHex, data))
	assert.Equal(t, "AAH/", protox.EncodeBytes(protox.BytesEncodingBase64, data))
	assert.Equal(t, "19p", protox.EncodeBytes(protox.BytesEncodingBase58, data))
	assert.Equal(t, "0001ff", protox.EncodeBytes(protox.BytesEncoding("nonsense"), data), "unknown encoding falls back to hex")
	assert.Equal(t, "", protox.EncodeBytes(protox.BytesEncodingHex, nil))
}

func TestJSONBytesEncoders(t *testing.T) {
	data := []byte{0xde, 0xad, 0xbe, 0xef}

	tests := []struct {
		name   string
		fn     protox.JSONBytesEncoderFunc
		expect string
	}{
		// jsontext.Encoder appends a trailing newline once a complete top-level value has been
		// written (it supports streaming a sequence of top-level values); these encoder funcs
		// write a single top-level string value here, so the newline is expected.
		{"hex", protox.JSONBytesToHex, "\"deadbeef\"\n"},
		{"base64", protox.JSONBytesToBase64, "\"3q2+7w==\"\n"},
		{"base58", protox.JSONBytesToBase58, "\"6h8cQN\"\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := bytes.NewBuffer(nil)
			enc := jsontext.NewEncoder(buf)

			require.NoError(t, test.fn(enc, data))
			assert.Equal(t, test.expect, buf.String())
		})
	}
}
