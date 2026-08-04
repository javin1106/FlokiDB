package server

import (
	"fmt"
	"io"
	"javinkv/config"
	"javinkv/core"
	"log"
	"net"
	"strconv"
	"strings"
)

func toArrayString(values []interface{}) ([]string, error) {
	tokens := make([]string, len(values))
	for i, value := range values {
		token, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("expected command token to be a string, got %T", value)
		}
		tokens[i] = token
	}
	return tokens, nil
}

func readCommands(c io.ReadWriter) (core.RedisCmds, error) {
	buf := make([]byte, 512)
	n, err := c.Read(buf[:])
	if err != nil {
		return nil, err
	}

	values, err := core.Decode(buf[:n])
	if err != nil {
		return nil, err
	}

	cmds := make(core.RedisCmds, 0, len(values))
	for _, value := range values {
		array, ok := value.([]interface{})
		if !ok {
			return nil, fmt.Errorf("expected command to be a RESP array, got %T", value)
		}

		tokens, err := toArrayString(array)
		if err != nil {
			return nil, err
		}
		if len(tokens) == 0 {
			return nil, fmt.Errorf("command array cannot be empty")
		}

		cmds = append(cmds, &core.RedisCmd{
			Cmd:  strings.ToUpper(tokens[0]),
			Args: tokens[1:],
		})
	}

	return cmds, nil
}

func respond(cmds core.RedisCmds, c io.ReadWriter) {
	core.EvalAndRespond(cmds, c)
}

func RunSyncTCPServer() {
	log.Println("starting a synchronous TCP server on", config.Host, config.Port)

	var con_clients int = 0

	// listening to the configured host:port
	lsnr, err := net.Listen("tcp", config.Host+":"+strconv.Itoa(config.Port))
	if err != nil {
		log.Println("err", err)
		return
	}

	for {
		// blocking call: waiting for the new client to connect
		c, err := lsnr.Accept()
		if err != nil {
			log.Println("err", err)
			continue
		}

		// increment the number of concurrent clients
		con_clients += 1

		for {
			// over the socket, continuously read the command and print it out
			cmds, err := readCommands(c)
			if err != nil {
				c.Close()
				con_clients -= 1
				if err != io.EOF {
					log.Println("err", err)
				}
				break
			}
			respond(cmds, c)
		}
	}
}
