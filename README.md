# Nagle's Algorithm Wrapper Module for Go

This Go module implements a wrapper for the `io.ReadWriteCloser` interface, applying a buffering mechanism similar to **Nagle's Algorithm**. Nagle's Algorithm is used to reduce the number of small packets sent over TCP by combining multiple small writes into a single larger write, optimizing network usage.

## Overview

The `NagleWrapper` struct wraps an existing `io.ReadWriteCloser` object and buffers write operations. It flushes the buffer either when:
1. The buffer reaches a specified size (controlled by the `bufferSize` parameter).
2. A timeout occurs (controlled by the `flushTimeout` parameter), ensuring that even small amounts of data are eventually sent.

This module is useful when working with TCP connections or any other stream-based protocols where sending small packets individually could be inefficient.

## Features

- **Buffered Writing**: Data is buffered and only sent when the buffer is full or the timeout is reached.
- **Configurable Buffer Size and Timeout**: You can specify the buffer size and flush timeout when initializing the wrapper.
- **Concurrent Safety**: The implementation uses a mutex to protect the buffer during concurrent writes.
- **Automatic Flushing**: A background goroutine flushes the buffer when the timeout expires.

## Usage

### 1. Initialize the Module

To use the `NagleWrapper`, you must wrap an existing `io.ReadWriteCloser` such as a TCP connection. You need to provide:
- A `bufferSize` to determine how much data can be buffered before sending.
- A `flushTimeout` to define how long to wait before flushing the buffer automatically.

Example:

```go
conn, err := net.Dial("tcp", "localhost:8080")
if err != nil {
    log.Fatal(err)
}

wrappedConn := nagle.NewNagleWrapper(conn, 1024, 100*time.Millisecond)
```
