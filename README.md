# FlokiDB

A Redis-compatible, in-memory key-value server written in Go using TCP sockets,
RESP, and Linux `epoll`.

## Deferred: Stream-safe command pipelining

The current pipelining implementation can decode and execute multiple complete
RESP commands when they arrive together in one socket read. TCP does not
preserve application-message boundaries, however, so a command may be split
across reads, and a non-blocking write may send only part of a response batch.

The next pipelining milestone is to maintain state for every connected client:

```go
type ConnState struct {
	Input  []byte // received bytes not yet decoded into a complete command
	Output []byte // response bytes not yet written to the socket
}
```

For the asynchronous server, store this state by client file descriptor:

```go
clients := map[int]*ConnState{}
```

### Input path

1. On `EPOLLIN`, read available bytes and append them to `ConnState.Input`.
2. Decode complete RESP commands while tracking the number of bytes consumed.
3. Execute complete commands in order and append their responses to
   `ConnState.Output`.
4. If decoding returns `core.ErrIncompleteRESP`, retain the incomplete trailing
   bytes instead of closing the connection.
5. Remove only successfully consumed bytes from the input buffer.
6. Enforce a maximum input-buffer size so one client cannot consume unlimited
   memory.

A useful decoder contract is:

```go
func DecodeCommands(data []byte) (cmds RedisCmds, consumed int, err error)
```

### Output path

1. Attempt to write `ConnState.Output` after evaluating commands.
2. Remove only the bytes successfully written.
3. Retain the remainder when `write` returns a partial count, `EAGAIN`, or
   `EWOULDBLOCK`.
4. Enable `EPOLLOUT` while output remains queued.
5. Continue flushing when epoll reports `EPOLLOUT`.
6. Disable `EPOLLOUT` after the output buffer becomes empty.
7. Enforce a maximum output-buffer size for backpressure.

The intended connection flow is:

```text
EPOLLIN -> input buffer -> RESP decoder -> command evaluation
                                          |
                                          v
EPOLLOUT <- socket write <- output buffer <- ordered responses
```

This work should be completed before describing FlokiDB's pipelining as fully
TCP-stream-safe.
