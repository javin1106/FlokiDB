package core

import (
	"bytes"
	"errors"
	"io"
	"strconv"
	"time"
)

var RESP_NIL []byte = []byte("$-1\r\n")
var RESP_OK []byte = []byte("+OK\r\n")
var RESP_ZERO []byte = []byte(":0\r\n")
var RESP_ONE []byte = []byte(":1\r\n")
var RESP_MINUS_1 []byte = []byte(":-1\r\n")
var RESP_MINUS_2 []byte = []byte(":-2\r\n")

func evalPING(args []string) []byte {
	var b []byte
	if len(args) >= 2 {
		return Encode(errors.New("ERR wrong number of arguments for 'ping' command"), false)
	}

	if len(args) == 0 {
		// PING with no arguments returns the standard simple-string reply.
		b = Encode("PONG", true)
	} else {
		// PING with one argument echoes that value back as a bulk string.
		b = Encode(args[0], false)
	}

	return b
}

func evalSET(args []string) []byte {
	if len(args) <= 1 {
		return Encode(errors.New("(error) ERR wrong number of arguments for 'set' command"), false)
	}

	var key, value string
	var exDurationMs int64 = -1 // never expire

	key, value = args[0], args[1]
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "EX", "ex":
			i++
			if i == len(args) {
				return Encode(errors.New("(error) ERR syntax error"), false)
			}
			exDurationSec, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				return Encode(errors.New("(error) ERR value is not an integer or out of range"), false)
			}
			if exDurationSec <= 0 {
				return Encode(errors.New("(error) ERR invalid expire time in 'set' command"), false)
			}
			exDurationMs = exDurationSec * 1000 // in Redis, time is by default in ms

		default:
			return Encode(errors.New("(error) ERR syntax error"), false)
		}
	}
	Put(key, NewObj(value, exDurationMs))
	return RESP_OK
}

func evalGET(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("(error) ERR wrong number of arguments for 'get' command"), false)
	}

	var key string = args[0]

	// Get the key from the hash table
	obj := Get(key)

	// if key does not exist, return RESP encoded nil
	if obj == nil {
		return RESP_NIL
	}

	// if key already expired then return nil
	if obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		Del(key)
		return RESP_NIL
	}

	// return the RESP encoded value
	return Encode(obj.Value, false)
}

func evalTTL(args []string) []byte {
	if len(args) != 1 {
		return Encode(errors.New("(error) ERR wrong number of arguments for 'ttl' command"), false)
	}

	var key string = args[0]

	obj := Get(key)

	// if key does not exist, return RESP encoded -2 denoting key does not exist
	if obj == nil {
		return RESP_MINUS_2
	}

	durationMs := obj.ExpiresAt - time.Now().UnixMilli()

	// if key expired i.e. key does not exist hence return -2
	if durationMs < 0 {
		Del(key)
		return RESP_MINUS_2
	}

	// if object exist, but no expiration is set on it then send -1
	if obj.ExpiresAt == -1 {
		return RESP_MINUS_1
	}

	// compute the time remaining for the key to expire and
	// return the RESP encoded form of it
	return Encode(int64(durationMs/1000), false)
}

func evalDEL(args []string) []byte {
	if len(args) == 0 {
		return Encode(errors.New("(error)ERR wrong number of arguments for 'del' command"), false)
	}
	var countDeleted int = 0
	for _, key := range args {
		obj := Get(key)
		if obj != nil && obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
			Del(key)
			continue
		}
		if ok := Del(key); ok {
			countDeleted++
		}
	}
	return Encode(countDeleted, false)
}

func evalEXPIRE(args []string) []byte {
	if len(args) != 2 {
		return Encode(errors.New("(error)ERR wrong number of arguments for 'expire' command"), false)
	}
	var key string = args[0]
	exDurationSec, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return Encode(errors.New("(error) ERR value is not an integer or out of range"), false)
	}
	if exDurationSec <= 0 {
		return Encode(errors.New("(error) ERR invalid expire time in 'expire' command"), false)
	}
	obj := Get(key)

	// unsuccessful -> doesn't exits, timeout not set
	if obj == nil {
		return RESP_NIL
	}
	if obj.ExpiresAt != -1 && obj.ExpiresAt <= time.Now().UnixMilli() {
		Del(key)
		return RESP_NIL
	}

	obj.ExpiresAt = time.Now().UnixMilli() + exDurationSec*1000
	return RESP_ONE
}

func EvalAndRespond(cmds RedisCmds, c io.ReadWriter) {
	var response []byte
	buf := bytes.NewBuffer(response)

	for _, cmd := range cmds {
		switch cmd.Cmd {
		case "PING":
			buf.Write(evalPING(cmd.Args))
		case "SET":
			buf.Write(evalSET(cmd.Args))
		case "GET":
			buf.Write(evalGET(cmd.Args))
		case "TTL":
			buf.Write(evalTTL(cmd.Args))
		case "DEL":
			buf.Write(evalDEL(cmd.Args))
		case "EXPIRE":
			buf.Write(evalEXPIRE(cmd.Args))
		default:
			buf.Write(evalPING(cmd.Args))
		}
	}

	c.Write(buf.Bytes())
}
