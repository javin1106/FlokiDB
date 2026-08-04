package core

import (
	"errors"
	"fmt"
)

// return length, "delta" -> how many bytes we moved forward in the buffer, including the trailing \r\n
func readLength(data []byte) (int, int) {
	pos, length := 0, 0 // both start at 0
	for pos = range data {
		b := data[pos]
		if !(b >= '0' && b <= '9') { // is NOT a valid digit, stop and return
			return length, pos + 2
		}
		length = length*10 + int(b-'0')
	}
	return 0, 0 // fallback -> no \r\n encountered
}

func readSimpleString(data []byte) (string, int, error) {
	pos := 1 // 0-th index is the symbol for string '+'
	for ; data[pos] != '\r'; pos++ {
	}

	return string(data[1:pos]), pos + 2, nil
}

func readError(data []byte) (string, int, error) {
	return readSimpleString(data)
}

func readInt64(data []byte) (int64, int, error) {
	pos := 1
	var value int64 = 0

	for ; data[pos] != '\r'; pos++ {
		value = value*10 + int64(data[pos]-'0')
	}
	return value, pos + 2, nil
}

func readBulkString(data []byte) (string, int, error) {
	pos := 1

	len, delta := readLength(data[pos:])
	pos += delta

	return string(data[pos:(pos + len)]), pos + len + 2, nil
}

func readArray(data []byte) (interface{}, int, error) {
	pos := 1
	count, delta := readLength(data[pos:])
	pos += delta
	var elems []interface{} = make([]interface{}, count)

	for i := range elems {
		elem, delta, err := DecodeOne(data[pos:])
		if err != nil {
			return nil, 0, err
		}

		elems[i] = elem
		pos += delta
	}
	return elems, pos, nil
}

func DecodeOne(data []byte) (interface{}, int, error) {
	if len(data) == 0 {
		return nil, 0, errors.New("no data")
	}

	switch data[0] { // only the first ch to identify data type
	case '+':
		return readSimpleString(data)
	case '-':
		return readError(data)
	case ':':
		return readInt64(data)
	case '$':
		return readBulkString(data)
	case '*':
		return readArray(data)
	}
	return nil, 0, nil // fallback
}

func Decode(data []byte) (interface{}, error) {
	if len(data) == 0 {
		return nil, errors.New("no data")
	}
	value, _, err := DecodeOne(data)
	return value, err
}

func DecodeArrayString(data []byte) ([]string, error) {
	value, err := Decode(data)
	if err != nil {
		return nil, err
	}

	ts := value.([]interface{})
	tokens := make([]string, len(ts))
	for i := range tokens {
		tokens[i] = ts[i].(string)
	}

	return tokens, nil
}

func Encode(value interface{}, isSimple bool) []byte {
	switch v := value.(type) {
	case string:
		if isSimple {
			// Simple strings are replies like +PONG\r\n.
			return fmt.Appendf(nil, "+%s\r\n", v)
		}
		// Bulk strings include the payload length before the actual value.
		return fmt.Appendf(nil, "$%d\r\n%s\r\n", len(v), v)
	case int, int8, int16, int32, int64:
		return fmt.Appendf(nil, ":%d\r\n", v)
	}
	return []byte{}
}
