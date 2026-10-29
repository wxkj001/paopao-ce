package tracker

import (
	"encoding/hex"
	"testing"
)

func TestParseInfoHash(t *testing.T) {
	expectedHex := "f1bd2f44418e845c8f3fb7a687ece396cc74fab6"
	expectedBytes, _ := hex.DecodeString(expectedHex)

	// Case 1: 40-character hex string
	b, h, err := parseInfoHash(expectedHex)
	if err != nil || h != expectedHex || string(b) != string(expectedBytes) {
		t.Fatalf("Case 1 hex failed: err=%v, h=%s", err, h)
	}

	// Case 2: 20-character binary string from JS (String.fromCharCode / Buffer.toString('binary'))
	// In JS, binary string characters have codepoints 0-255.
	// When JSON unmarshaled into a Go string, they are runes with value equal to the byte value.
	runes := make([]rune, 20)
	for i, bVal := range expectedBytes {
		runes[i] = rune(bVal)
	}
	jsBinaryString := string(runes)

	b2, h2, err2 := parseInfoHash(jsBinaryString)
	if err2 != nil || h2 != expectedHex || string(b2) != string(expectedBytes) {
		t.Fatalf("Case 2 js binary string failed: err=%v, h=%s", err2, h2)
	}
}
