package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io"
)

// Gob messages are length-prefixed. Replacing one uniquely identified string
// preserves every other field byte, including fields unknown to this utility.
func replaceGobString(raw []byte, old, next string) ([]byte, error) {
	needle := append(gobUint(uint64(len(old))), []byte(old)...)
	replacement := append(gobUint(uint64(len(next))), []byte(next)...)
	if old == "" || bytes.Count(raw, needle) != 1 {
		return nil, errors.New("ambiguous_gob_identity")
	}
	out := []byte{}
	changed := false
	for offset := 0; offset < len(raw); {
		length, n, err := readGobUint(raw[offset:])
		if err != nil || length > uint64(len(raw)-offset-n) {
			return nil, errors.New("invalid_gob_message")
		}
		payload := raw[offset+n : offset+n+int(length)]
		if index := bytes.Index(payload, needle); index >= 0 {
			if changed {
				return nil, errors.New("ambiguous_gob_identity")
			}
			changed = true
			out = append(out, gobUint(uint64(len(payload)-len(needle)+len(replacement)))...)
			out = append(out, payload[:index]...)
			out = append(out, replacement...)
			out = append(out, payload[index+len(needle):]...)
		} else {
			out = append(out, raw[offset:offset+n+int(length)]...)
		}
		offset += n + int(length)
	}
	if !changed {
		return nil, errors.New("missing_gob_identity")
	}
	return out, nil
}

func gobUint(value uint64) []byte {
	if value < 128 {
		return []byte{byte(value)}
	}
	var digits [8]byte
	start := len(digits)
	for value != 0 {
		start--
		digits[start] = byte(value)
		value >>= 8
	}
	return append([]byte{byte(256 - (len(digits) - start))}, digits[start:]...)
}

func readGobUint(raw []byte) (uint64, int, error) {
	if len(raw) == 0 {
		return 0, 0, io.ErrUnexpectedEOF
	}
	if raw[0] < 128 {
		return uint64(raw[0]), 1, nil
	}
	size := 256 - int(raw[0])
	if size > 8 || size+1 > len(raw) {
		return 0, 0, io.ErrUnexpectedEOF
	}
	var value uint64
	for _, b := range raw[1 : 1+size] {
		value = value<<8 | uint64(b)
	}
	return value, 1 + size, nil
}

func decodeGob(raw []byte, into any) error {
	if len(raw) == 0 || len(raw) > 16<<20 {
		return errors.New("invalid_gob_record")
	}
	decoder := gob.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(into); err != nil {
		return errors.New("invalid_gob_record")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("multiple_gob_records")
	}
	return nil
}
