package protox

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json/jsontext"
	"fmt"
	"strings"

	"github.com/mr-tron/base58"
)

// BytesEncoding is the textual representation used when rendering Protobuf `bytes` values.
//
// It is deliberately not JSON-scoped: it is a general-purpose concept, also usable outside
// of JSON rendering.
type BytesEncoding string

const (
	BytesEncodingHex    BytesEncoding = "hex"
	BytesEncodingBase58 BytesEncoding = "base58"
	BytesEncodingBase64 BytesEncoding = "base64"
)

// ParseBytesEncoding turns a case-insensitive string into a BytesEncoding, returning an
// error for anything unrecognized.
func ParseBytesEncoding(in string) (BytesEncoding, error) {
	switch {
	case strings.EqualFold(in, string(BytesEncodingHex)):
		return BytesEncodingHex, nil
	case strings.EqualFold(in, string(BytesEncodingBase58)):
		return BytesEncodingBase58, nil
	case strings.EqualFold(in, string(BytesEncodingBase64)):
		return BytesEncodingBase64, nil
	}

	return "", fmt.Errorf("unsupported bytes encoding %q, accepted values are %q, %q and %q",
		in, BytesEncodingHex, BytesEncodingBase58, BytesEncodingBase64)
}

// EncodeBytes encodes data using the received encoding, falling back to hexadecimal when the
// encoding is unknown.
func EncodeBytes(encoding BytesEncoding, data []byte) string {
	switch encoding {
	case BytesEncodingBase58:
		return base58.Encode(data)
	case BytesEncodingBase64:
		return base64.StdEncoding.EncodeToString(data)
	default:
		return hex.EncodeToString(data)
	}
}

// JSONBytesEncoderFunc writes data as a single JSON value onto the encoder. Implementations
// must write exactly one value.
type JSONBytesEncoderFunc func(enc *jsontext.Encoder, data []byte) error

// JSONBytesToHex writes data as a hexadecimal JSON string.
func JSONBytesToHex(enc *jsontext.Encoder, data []byte) error {
	return enc.WriteToken(jsontext.String(hex.EncodeToString(data)))
}

// JSONBytesToBase58 writes data as a base58 JSON string.
func JSONBytesToBase58(enc *jsontext.Encoder, data []byte) error {
	return enc.WriteToken(jsontext.String(base58.Encode(data)))
}

// JSONBytesToBase64 writes data as a standard base64 JSON string.
func JSONBytesToBase64(enc *jsontext.Encoder, data []byte) error {
	return enc.WriteToken(jsontext.String(base64.StdEncoding.EncodeToString(data)))
}

func jsonBytesEncoderFor(encoding BytesEncoding) JSONBytesEncoderFunc {
	switch encoding {
	case BytesEncodingBase58:
		return JSONBytesToBase58
	case BytesEncodingBase64:
		return JSONBytesToBase64
	default:
		return JSONBytesToHex
	}
}
