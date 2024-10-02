package nagle

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// NagleWrapper wraps a ReadWriteCloser interface with Nagle's algorithm buffering logic.
type NagleWrapper struct {
	rwc          io.ReadWriteCloser
	buffer       *bytes.Buffer
	bufferSize   int
	flushTimeout time.Duration
	mutex        sync.Mutex
	timer        *time.Timer
	closed       bool
}

// NewNagleWrapper creates a new wrapper with Nagle's algorithm.
func NewNagleWrapper(rwc io.ReadWriteCloser, bufferSize int, flushTimeout time.Duration) *NagleWrapper {
	wrapper := &NagleWrapper{
		rwc:          rwc,
		buffer:       &bytes.Buffer{},
		bufferSize:   bufferSize,
		flushTimeout: flushTimeout,
		timer:        time.NewTimer(flushTimeout),
		closed:       false,
	}

	go wrapper.handleFlush()

	return wrapper
}

// Write writes data to the buffer and sends it if the buffer is full or the maximum time (timeout) has passed.
func (nw *NagleWrapper) Write(data []byte) (int, error) {
	nw.mutex.Lock()
	defer nw.mutex.Unlock()

	if nw.closed {
		return 0, io.ErrClosedPipe
	}

	nw.buffer.Write(data)

	if nw.buffer.Len() >= nw.bufferSize {
		return nw.flushLocked()
	}

	if nw.timer.Stop() {
		select {
		case <-nw.timer.C:
		default:
		}
	}

	nw.timer.Reset(nw.flushTimeout)

	return len(data), nil
}

// Read reads data from the underlying stream.
func (nw *NagleWrapper) Read(p []byte) (int, error) {
	return nw.rwc.Read(p)
}

// Close closes the wrapper, flushing any remaining data.
func (nw *NagleWrapper) Close() error {
	nw.mutex.Lock()
	defer nw.mutex.Unlock()

	if nw.closed {
		return io.ErrClosedPipe
	}

	_, err := nw.flushLocked()
	if err != nil {
		return err
	}

	nw.closed = true
	return nw.rwc.Close()
}

func (nw *NagleWrapper) handleFlush() {
	for {
		<-nw.timer.C

		nw.mutex.Lock()

		if nw.closed {
			nw.mutex.Unlock()
			return
		}

		if nw.buffer.Len() > 0 {
			nw.flushLocked()
		}
		nw.mutex.Unlock()
	}
}

func (nw *NagleWrapper) flushLocked() (int, error) {
	if nw.buffer.Len() == 0 {
		return 0, nil
	}

	n, err := nw.buffer.WriteTo(nw.rwc)
	if err != nil {
		return int(n), err
	}

	return int(n), nil
}
