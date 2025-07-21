# Nagle's Algorithm Wrapper Module for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/jaracil/nagle.svg)](https://pkg.go.dev/github.com/jaracil/nagle)
[![Go Report Card](https://goreportcard.com/badge/github.com/jaracil/nagle)](https://goreportcard.com/report/github.com/jaracil/nagle)

This Go module implements a wrapper for the `io.ReadWriteCloser` interface, applying a buffering mechanism similar to **Nagle's Algorithm**. Nagle's Algorithm is used to reduce the number of small packets sent over TCP by combining multiple small writes into a single larger write, optimizing network usage.

## Installation

```bash
go get github.com/jaracil/nagle
```

## Overview

The `NagleWrapper` struct wraps an existing `io.ReadWriteCloser` object and buffers write operations. It flushes the buffer either when:
1. The buffer reaches a specified size (controlled by the `bufferSize` parameter).
2. A timeout occurs (controlled by the `flushTimeout` parameter), ensuring that even small amounts of data are eventually sent.
3. Manual flush is triggered using the `Flush()` method.

This module is useful when working with TCP connections or any other stream-based protocols where sending small packets individually could be inefficient.

## Features

- **Buffered Writing**: Data is buffered and only sent when the buffer is full or the timeout is reached.
- **Configurable Buffer Size and Timeout**: You can specify the buffer size and flush timeout when initializing the wrapper.
- **Manual Flush Control**: Use the `Flush()` method to force immediate flushing of buffered data.
- **Automatic Flushing**: A background goroutine flushes the buffer when the timeout expires.

## API Reference

### Creating a Wrapper

```go
func NewNagleWrapper(rwc io.ReadWriteCloser, bufferSize int, flushTimeout time.Duration) *NagleWrapper
```

Creates a new Nagle wrapper around an existing `io.ReadWriteCloser`.

**Parameters:**
- `rwc`: The underlying `io.ReadWriteCloser` to wrap
- `bufferSize`: Maximum buffer size before automatic flush (in bytes)
- `flushTimeout`: Maximum time to wait before automatic flush

### Methods

#### Write
```go
func (nw *NagleWrapper) Write(data []byte) (int, error)
```
Writes data to the buffer. If the buffer reaches `bufferSize`, it automatically flushes to the underlying writer.

#### Read
```go
func (nw *NagleWrapper) Read(p []byte) (int, error)
```
Reads data directly from the underlying reader (unbuffered).

#### Flush
```go
func (nw *NagleWrapper) Flush() error
```
Manually flushes any buffered data to the underlying writer.

#### Close
```go
func (nw *NagleWrapper) Close() error
```
Flushes remaining data and closes the underlying writer.

## Usage Examples

### Basic TCP Connection

```go
package main

import (
    "log"
    "net"
    "time"
    
    "github.com/jaracil/nagle"
)

func main() {
    // Connect to server
    conn, err := net.Dial("tcp", "localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    
    // Wrap connection with Nagle algorithm
    // Buffer up to 1KB or flush after 100ms
    wrapped := nagle.NewNagleWrapper(conn, 1024, 100*time.Millisecond)
    defer wrapped.Close()
    
    // Small writes will be buffered
    for i := 0; i < 10; i++ {
        _, err := wrapped.Write([]byte("Hello "))
        if err != nil {
            log.Fatal(err)
        }
    }
    
    // Force immediate flush
    err = wrapped.Flush()
    if err != nil {
        log.Fatal(err)
    }
    
    // Read response
    buf := make([]byte, 1024)
    n, err := wrapped.Read(buf)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Received: %s", buf[:n])
}
```

### File Writing with Buffering

```go
package main

import (
    "os"
    "time"
    
    "github.com/jaracil/nagle"
)

func main() {
    file, err := os.Create("output.txt")
    if err != nil {
        panic(err)
    }
    
    // Buffer up to 4KB or flush every 500ms
    wrapped := nagle.NewNagleWrapper(file, 4096, 500*time.Millisecond)
    defer wrapped.Close()
    
    // Multiple small writes
    for i := 0; i < 1000; i++ {
        wrapped.Write([]byte("Line of text\n"))
    }
    
    // Close will flush remaining data
}
```

## Running Tests

```bash
# Run all tests
go test

# Run tests with verbose output
go test -v

# Run tests with race detection
go test -race

# Run tests with coverage
go test -cover
```

## Performance Considerations

- **Buffer Size**: Larger buffers reduce system calls but increase memory usage and latency
- **Flush Timeout**: Shorter timeouts reduce latency but may increase the number of writes
- **Use Cases**: Most beneficial when dealing with many small writes that can be combined

## License

See LICENSE file for details.
