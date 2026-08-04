package core

// RedisCmd is the parsed command name plus any arguments sent by the client.
type RedisCmd struct {
	Cmd  string
	Args []string
}

type RedisCmds []*RedisCmd
