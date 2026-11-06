# FlokiDB

A Redis-compatible, in-memory key-value server written in Go using TCP sockets,
RESP, and Linux `epoll`.

## Benchmarks

Tool: `redis-benchmark` (from `redis-tools`), run in WSL2 (Ubuntu) against
FlokiDB built and running in the same WSL instance, port 7379.

| Command | Result | Notes |
|---|---|---|
| `redis-benchmark -p 7379 -t set,get -n 100000 -c 50 -q` | GET: ~152,439 req/s, p50 = 0.167 ms | 100k requests, 50 clients, no pipelining. SET (run separately): ~156,495 req/s, p50 = 0.159 ms. |
| `redis-benchmark -p 7379 -t set,get -n 200000 -c 50 -P 16 -q` | Failed: `Connection reset by peer` | Pipelining. The server closes the connection when a read ends mid-command. See "Deferred: Stream-safe command pipelining" below. |

Baseline: real Redis 8.0.5 (`redis-server --port 6380 --save ""`, no persistence),
same machine and same benchmark command (`-t set,get -n 100000 -c 50 -q`):

| Server | SET (req/s) | GET (req/s) | p50 |
|---|---|---|---|
| Redis 8.0.5 | 165,289 | 160,256 | 0.151 ms |
| FlokiDB | 156,495 | 152,439 | 0.159 ms (SET), 0.167 ms (GET) |

FlokiDB reaches about 95% of Redis throughput on SET and GET in this test. Note that
Redis ran with persistence off, while FlokiDB writes an AOF.

### Concurrency stress test

`redis-benchmark -p 7379 -t set,get -n 200000 -c <clients> -q`, with
`ulimit -n 65536` for both server and client, same WSL2 instance (loopback).

| Clients | SET (req/s) | GET (req/s) | p50 SET / GET | Server survived |
|---|---|---|---|---|
| 50 | 152,555 | 150,602 | 0.167 / 0.167 ms | yes |
| 500 | 135,593 | 134,862 | 1.807 / 1.783 ms | yes |
| 1,000 | 122,474 | 110,988 | 4.087 / 4.351 ms | yes |
| 2,000 | 97,943 | 91,575 | 9.671 / 10.479 ms | yes |
| 5,000 | 75,131 | 71,149 | 32.831 / 34.463 ms | yes |
| 10,000 | 92,039 | 102,881 | 53.791 / 47.167 ms | yes |
| 15,000 | 102,669 | 110,497 | 48.767 / 46.527 ms | yes |

FlokiDB served 15,000 concurrent connections without errors or crashes,
sustaining ~100K req/s. Latency rises with client count, as expected for a
single-threaded event loop. Runs beyond 15,000 clients were not completed.
The benchmark client shares the same machine, so it competes for CPU at high
client counts.

`WARNING: Could not fetch server CONFIG` appears because `CONFIG GET` is not
implemented. It is harmless.

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
