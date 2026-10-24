package bencode

import (
	"bytes"
	"reflect"
	"testing"
)

func TestBencodeInt(t *testing.T) {
	data := []byte("i42e")
	val, err := DecodeBytes(data)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if val != int64(42) {
		t.Fatalf("Expected 42, got %v", val)
	}

	enc, err := EncodeBytes(int64(42))
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if !bytes.Equal(enc, data) {
		t.Fatalf("Expected %s, got %s", data, enc)
	}
}

func TestBencodeString(t *testing.T) {
	data := []byte("4:spam")
	val, err := DecodeBytes(data)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if val != "spam" {
		t.Fatalf("Expected 'spam', got %v", val)
	}

	enc, err := EncodeBytes("spam")
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if !bytes.Equal(enc, data) {
		t.Fatalf("Expected %s, got %s", data, enc)
	}
}

func TestBencodeList(t *testing.T) {
	data := []byte("l4:spami42ee")
	val, err := DecodeBytes(data)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	expected := []any{"spam", int64(42)}
	if !reflect.DeepEqual(val, expected) {
		t.Fatalf("Expected %v, got %v", expected, val)
	}

	enc, err := EncodeBytes(expected)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if !bytes.Equal(enc, data) {
		t.Fatalf("Expected %s, got %s", data, enc)
	}
}

func TestBencodeDict(t *testing.T) {
	data := []byte("d3:bar4:spam3:fooi42ee")
	val, err := DecodeBytes(data)
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	expected := map[string]any{
		"bar": "spam",
		"foo": int64(42),
	}
	if !reflect.DeepEqual(val, expected) {
		t.Fatalf("Expected %v, got %v", expected, val)
	}

	enc, err := EncodeBytes(expected)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if !bytes.Equal(enc, data) {
		t.Fatalf("Expected %s, got %s", data, enc)
	}
}
