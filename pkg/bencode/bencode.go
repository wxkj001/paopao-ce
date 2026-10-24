// Package bencode implements encoding and decoding of Bencoded data per BEP 0003.
package bencode

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
)

var (
	ErrInvalidBencode = errors.New("invalid bencode data")
	ErrUnexpectedEOF  = errors.New("unexpected end of file")
)

// Decode reads bencoded data from r and decodes it into Go native types:
// - string or []byte -> string
// - integer -> int64
// - list -> []any
// - dictionary -> map[string]any
func Decode(r io.Reader) (any, error) {
	br := bufio.NewReader(r)
	return decodeValue(br)
}

// DecodeBytes decodes bencoded bytes.
func DecodeBytes(data []byte) (any, error) {
	return Decode(bytes.NewReader(data))
}

func decodeValue(r *bufio.Reader) (any, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	switch b {
	case 'i':
		return decodeInt(r)
	case 'l':
		return decodeList(r)
	case 'd':
		return decodeDict(r)
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if err := r.UnreadByte(); err != nil {
			return nil, err
		}
		return decodeString(r)
	default:
		return nil, fmt.Errorf("%w: unknown type indicator '%c'", ErrInvalidBencode, b)
	}
}

func decodeInt(r *bufio.Reader) (int64, error) {
	s, err := r.ReadString('e')
	if err != nil {
		return 0, err
	}
	s = s[:len(s)-1] // trim 'e'
	return strconv.ParseInt(s, 10, 64)
}

func decodeString(r *bufio.Reader) (string, error) {
	lenStr, err := r.ReadString(':')
	if err != nil {
		return "", err
	}
	lenStr = lenStr[:len(lenStr)-1] // trim ':'
	length, err := strconv.ParseInt(lenStr, 10, 64)
	if err != nil || length < 0 {
		return "", ErrInvalidBencode
	}

	buf := make([]byte, length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func decodeList(r *bufio.Reader) ([]any, error) {
	var list []any
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == 'e' {
			break
		}
		if err := r.UnreadByte(); err != nil {
			return nil, err
		}
		val, err := decodeValue(r)
		if err != nil {
			return nil, err
		}
		list = append(list, val)
	}
	return list, nil
}

func decodeDict(r *bufio.Reader) (map[string]any, error) {
	dict := make(map[string]any)
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b == 'e' {
			break
		}
		if err := r.UnreadByte(); err != nil {
			return nil, err
		}
		key, err := decodeString(r)
		if err != nil {
			return nil, err
		}
		val, err := decodeValue(r)
		if err != nil {
			return nil, err
		}
		dict[key] = val
	}
	return dict, nil
}

// Encode writes the bencoded representation of val to w.
// Dictionary keys are strictly sorted in lexicographical order per BitTorrent spec.
func Encode(w io.Writer, val any) error {
	switch v := val.(type) {
	case string:
		return encodeString(w, []byte(v))
	case []byte:
		return encodeString(w, v)
	case int:
		return encodeInt(w, int64(v))
	case int8:
		return encodeInt(w, int64(v))
	case int16:
		return encodeInt(w, int64(v))
	case int32:
		return encodeInt(w, int64(v))
	case int64:
		return encodeInt(w, v)
	case uint:
		return encodeInt(w, int64(v))
	case uint8:
		return encodeInt(w, int64(v))
	case uint16:
		return encodeInt(w, int64(v))
	case uint32:
		return encodeInt(w, int64(v))
	case uint64:
		return encodeInt(w, int64(v))
	case bool:
		if v {
			return encodeInt(w, 1)
		}
		return encodeInt(w, 0)
	case []any:
		return encodeList(w, v)
	case []string:
		list := make([]any, len(v))
		for i, s := range v {
			list[i] = s
		}
		return encodeList(w, list)
	case map[string]any:
		return encodeDict(w, v)
	case map[string]string:
		dict := make(map[string]any, len(v))
		for k, valStr := range v {
			dict[k] = valStr
		}
		return encodeDict(w, dict)
	default:
		return fmt.Errorf("unsupported bencode type: %T", val)
	}
}

// EncodeBytes returns the bencoded bytes for val.
func EncodeBytes(val any) ([]byte, error) {
	var buf bytes.Buffer
	if err := Encode(&buf, val); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeInt(w io.Writer, v int64) error {
	_, err := fmt.Fprintf(w, "i%de", v)
	return err
}

func encodeString(w io.Writer, b []byte) error {
	if _, err := fmt.Fprintf(w, "%d:", len(b)); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}

func encodeList(w io.Writer, list []any) error {
	if _, err := w.Write([]byte{'l'}); err != nil {
		return err
	}
	for _, item := range list {
		if err := Encode(w, item); err != nil {
			return err
		}
	}
	_, err := w.Write([]byte{'e'})
	return err
}

func encodeDict(w io.Writer, dict map[string]any) error {
	if _, err := w.Write([]byte{'d'}); err != nil {
		return err
	}

	keys := make([]string, 0, len(dict))
	for k := range dict {
		keys = append(keys, k)
	}
	sort.Strings(keys) // BitTorrent spec requires sorted keys

	for _, k := range keys {
		if err := encodeString(w, []byte(k)); err != nil {
			return err
		}
		if err := Encode(w, dict[k]); err != nil {
			return err
		}
	}
	_, err := w.Write([]byte{'e'})
	return err
}
