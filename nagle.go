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
	wg           sync.WaitGroup
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

	wrapper.wg.Add(1)
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

	n, err := nw.buffer.Write(data)
	if err != nil {
		return n, err
	}

	if nw.buffer.Len() >= nw.bufferSize {
		flushedBytes, flushErr := nw.flushLocked()
		if flushErr != nil {
			return flushedBytes, flushErr
		}
		return flushedBytes, nil
	}

	if !nw.timer.Stop() {
		select {
		case <-nw.timer.C:
		default:
		}
	}

	nw.timer.Reset(nw.flushTimeout)

	return n, nil
}

// Read reads data from the underlying stream.
func (nw *NagleWrapper) Read(p []byte) (int, error) {
	return nw.rwc.Read(p)
}

// Flush manually flushes any buffered data to the underlying writer.
func (nw *NagleWrapper) Flush() error {
	nw.mutex.Lock()
	defer nw.mutex.Unlock()

	if nw.closed {
		return io.ErrClosedPipe
	}

	_, err := nw.flushLocked()
	return err
}

// Close closes the wrapper, flushing any remaining data.
func (nw *NagleWrapper) Close() error {
	nw.mutex.Lock()
	
	if nw.closed {
		nw.mutex.Unlock()
		return io.ErrClosedPipe
	}

	// Flush any remaining data
	_, flushErr := nw.flushLocked()

	// Mark as closed to signal the flush goroutine
	nw.closed = true
	
	// Stop and drain the timer to wake up the flush goroutine
	if !nw.timer.Stop() {
		select {
		case <-nw.timer.C:
		default:
		}
	}
	nw.timer.Reset(0)
	
	nw.mutex.Unlock()
	
	// Wait for the flush goroutine to exit
	nw.wg.Wait()
	
	// Close the underlying writer
	closeErr := nw.rwc.Close()
	
	// Return the first error that occurred
	if flushErr != nil {
		return flushErr
	}
	return closeErr
}

func (nw *NagleWrapper) handleFlush() {
	defer nw.wg.Done()
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

	data := nw.buffer.Bytes()
	totalWritten := 0

	for totalWritten < len(data) {
		n, err := nw.rwc.Write(data[totalWritten:])
		totalWritten += n
		if err != nil {
			// Keep unwritten data in buffer for next flush attempt
			remaining := data[totalWritten:]
			nw.buffer.Reset()
			nw.buffer.Write(remaining)
			return totalWritten, err
		}
	}

	nw.buffer.Reset()
	return totalWritten, nil
}
