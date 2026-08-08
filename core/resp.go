package core

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

var ErrIncompleteRESP = errors.New("incomplete RESP frame")

func readLine(data []byte, start int) ([]byte, int, error) {
	if start >= len(data) {
		return nil, 0, ErrIncompleteRESP
	}

	end := bytes.Index(data[start:], []byte("\r\n"))
	if end == -1 {
		return nil, 0, ErrIncompleteRESP
	}
	end += start

	return data[start:end], end + 2, nil
}

// readLength returns the parsed length and the number of bytes consumed,
// including the trailing CRLF.
func readLength(data []byte) (int, int, error) {
	line, delta, err := readLine(data, 0)
	if err != nil {
		return 0, 0, err
	}

	length, err := strconv.Atoi(string(line))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid RESP length %q: %w", line, err)
	}

	return length, delta, nil
}

func readSimpleString(data []byte) (string, int, error) {
	line, delta, err := readLine(data, 1)
	if err != nil {
		return "", 0, err
	}

	return string(line), delta, nil
}

func readError(data []byte) (string, int, error) {
	return readSimpleString(data)
}

func readInt64(data []byte) (int64, int, error) {
	line, delta, err := readLine(data, 1)
	if err != nil {
		return 0, 0, err
	}

	value, err := strconv.ParseInt(string(line), 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid RESP integer %q: %w", line, err)
	}

	return value, delta, nil
}

func readBulkString(data []byte) (interface{}, int, error) {
	length, delta, err := readLength(data[1:])
	if err != nil {
		return nil, 0, err
	}
	pos := 1 + delta

	if length == -1 {
		return nil, pos, nil
	}
	if length < -1 {
		return nil, 0, fmt.Errorf("invalid bulk string length %d", length)
	}
	if len(data) < pos+length+2 {
		return nil, 0, ErrIncompleteRESP
	}
	if data[pos+length] != '\r' || data[pos+length+1] != '\n' {
		return nil, 0, errors.New("bulk string is not terminated by CRLF")
	}

	return string(data[pos : pos+length]), pos + length + 2, nil
}

func readArray(data []byte) (interface{}, int, error) {
	count, delta, err := readLength(data[1:])
	if err != nil {
		return nil, 0, err
	}
	pos := 1 + delta

	if count == -1 {
		return nil, pos, nil
	}
	if count < -1 {
		return nil, 0, fmt.Errorf("invalid array length %d", count)
	}

	elems := make([]interface{}, count)

	for i := range elems {
		elem, elemDelta, err := DecodeOne(data[pos:])
		if err != nil {
			return nil, 0, err
		}

		elems[i] = elem
		pos += elemDelta
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
	default:
		return nil, 0, fmt.Errorf("unknown RESP type byte %q", data[0])
	}
}

func Decode(data []byte) ([]interface{}, error) {
	if len(data) == 0 {
		return nil, errors.New("no data")
	}
	values := make([]interface{}, 0)
	index := 0
	for index < len(data) {
		value, delta, err := DecodeOne(data[index:])
		if err != nil {
			return values, err
		}
		if delta <= 0 {
			return values, errors.New("RESP decoder consumed no input")
		}
		index = index + delta
		values = append(values, value)
	}
	return values, nil
}

func encodeString(v string) []byte {
	return fmt.Appendf(nil, "$%d\r\n%s\r\n", len(v), v)
}

func Encode(value interface{}, isSimple bool) []byte {
	switch v := value.(type) {
	case string:
		if isSimple {
			// Simple strings are replies like +PONG\r\n.
			return fmt.Appendf(nil, "+%s\r\n", v)
		}
		// Bulk strings include the payload length before the actual value.
		return encodeString(v)
	case int, int8, int16, int32, int64:
		return fmt.Appendf(nil, ":%d\r\n", v)
	case []string:
		var b []byte
		buf := bytes.NewBuffer(b)
		for _, token := range v {
			buf.Write(encodeString(token))
		}

		return fmt.Appendf(nil, "*%d\r\n%s", len(v), buf.Bytes())
	case error:
		return fmt.Appendf(nil, "-%s\r\n", v.Error())
	case nil:
		return RESP_NIL
	}
	return []byte{}
}
