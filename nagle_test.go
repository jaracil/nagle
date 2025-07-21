package nagle

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

// MockReadWriteCloser mocks an io.ReadWriteCloser for testing purposes.
type MockReadWriteCloser struct {
	buffer    bytes.Buffer
	closed    bool
	mu        sync.Mutex
	failWrite bool     // Simulate write failures
	writeErr  error    // Error to return on write failure
	writeCount int     // Count of write attempts
}

func (m *MockReadWriteCloser) Write(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	
	m.writeCount++
	
	if m.failWrite {
		if m.writeErr != nil {
			return 0, m.writeErr
		}
		return 0, errors.New("mock write failure")
	}
	
	return m.buffer.Write(p)
}

func (m *MockReadWriteCloser) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.buffer.Read(p)
}

func (m *MockReadWriteCloser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return io.ErrClosedPipe
	}
	m.closed = true
	return nil
}

// String returns the buffer content in a thread-safe manner
func (m *MockReadWriteCloser) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buffer.String()
}

// Reset clears the buffer in a thread-safe manner
func (m *MockReadWriteCloser) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buffer.Reset()
}

// SetWriteFailure configures the mock to fail writes
func (m *MockReadWriteCloser) SetWriteFailure(shouldFail bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failWrite = shouldFail
	m.writeErr = err
}

// GetWriteCount returns the number of write attempts
func (m *MockReadWriteCloser) GetWriteCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeCount
}

func TestNagleWrapper_WriteFlushByBufferSize(t *testing.T) {
	mockRWC := &MockReadWriteCloser{}
	nagleWrapper := NewNagleWrapper(mockRWC, 10, 50*time.Millisecond)

	// Write 10 bytes (exact buffer size)
	data := []byte("0123456789")
	n, err := nagleWrapper.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected to write %d bytes, wrote %d", len(data), n)
	}

	// Check if buffer is flushed
	if mockRWC.String() != "0123456789" {
		t.Fatalf("expected buffer to contain '0123456789', but got: %s", mockRWC.String())
	}
}

func TestNagleWrapper_WriteFlushByTimeout(t *testing.T) {
	mockRWC := &MockReadWriteCloser{}
	nagleWrapper := NewNagleWrapper(mockRWC, 10, 50*time.Millisecond)
	for i := 0; i <= 2; i++ {
		// Write 5 bytes (less than buffer size)
		data := []byte("01234")
		n, err := nagleWrapper.Write(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != len(data) {
			t.Fatalf("expected to write %d bytes, wrote %d", len(data), n)
		}

		// Buffer should not be flushed yet
		if mockRWC.String() != "" {
			t.Fatalf("expected buffer to be empty, but got: %s", mockRWC.String())
		}

		// Wait for flush timeout
		time.Sleep(100 * time.Millisecond)

		// Buffer should be flushed now
		if mockRWC.String() != "01234" {
			t.Fatalf("expected buffer to contain '01234', but got: %s", mockRWC.String())
		}
		mockRWC.Reset()
	}
}

func TestNagleWrapper_CloseFlushesData(t *testing.T) {
	mockRWC := &MockReadWriteCloser{}
	nagleWrapper := NewNagleWrapper(mockRWC, 10, 50*time.Millisecond)

	// Write 5 bytes (less than buffer size)
	data := []byte("01234")
	n, err := nagleWrapper.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected to write %d bytes, wrote %d", len(data), n)
	}

	// Close should flush the buffer
	err = nagleWrapper.Close()
	if err != nil {
		t.Fatalf("unexpected error on close: %v", err)
	}

	// Check if buffer was flushed
	if mockRWC.String() != "01234" {
		t.Fatalf("expected buffer to contain '01234', but got: %s", mockRWC.String())
	}

	// Further writes should fail after close
	_, err = nagleWrapper.Write([]byte("more data"))
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected ErrClosedPipe, but got: %v", err)
	}

	// Further reads should fail after close
	buf := make([]byte, 5)
	_, err = nagleWrapper.Read(buf)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected ErrClosedPipe, but got: %v", err)
	}

	// Further closes should fail after close
	err = nagleWrapper.Close()
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected ErrClosedPipe, but got: %v", err)
	}
}

func TestNagleWrapper_Read(t *testing.T) {
	mockRWC := &MockReadWriteCloser{}
	nagleWrapper := NewNagleWrapper(mockRWC, 10, 50*time.Millisecond)

	// Prepare data to be read
	mockRWC.Write([]byte("readable data"))

	buf := make([]byte, 12)
	n, err := nagleWrapper.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 12 {
		t.Fatalf("expected to read 12 bytes, read %d", n)
	}

	expected := "readable dat"
	if string(buf[:n]) != expected {
		t.Fatalf("expected to read '%s', but got: '%s'", expected, string(buf[:n]))
	}
}

func TestNagleWrapper_Flush(t *testing.T) {
	mockRWC := &MockReadWriteCloser{}
	nagleWrapper := NewNagleWrapper(mockRWC, 100, 50*time.Millisecond)

	// Write some data (less than buffer size)
	data := []byte("test data")
	n, err := nagleWrapper.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected to write %d bytes, wrote %d", len(data), n)
	}

	// Buffer should not be flushed automatically yet
	if mockRWC.String() != "" {
		t.Fatalf("expected buffer to be empty before manual flush, but got: %s", mockRWC.String())
	}

	// Manual flush
	err = nagleWrapper.Flush()
	if err != nil {
		t.Fatalf("unexpected error on flush: %v", err)
	}

	// Buffer should be flushed now
	if mockRWC.String() != "test data" {
		t.Fatalf("expected buffer to contain 'test data', but got: %s", mockRWC.String())
	}

	// Second flush should do nothing
	err = nagleWrapper.Flush()
	if err != nil {
		t.Fatalf("unexpected error on second flush: %v", err)
	}

	// Close the wrapper
	err = nagleWrapper.Close()
	if err != nil {
		t.Fatalf("unexpected error on close: %v", err)
	}

	// Flush after close should fail
	err = nagleWrapper.Flush()
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("expected ErrClosedPipe after close, but got: %v", err)
	}
}

func TestNagleWrapper_WriteFailureOnFlush(t *testing.T) {
	mockRWC := &MockReadWriteCloser{}
	nagleWrapper := NewNagleWrapper(mockRWC, 10, 50*time.Millisecond)

	// Configure mock to fail writes
	expectedErr := errors.New("network connection broken")
	mockRWC.SetWriteFailure(true, expectedErr)

	// Write exactly buffer size to trigger immediate flush
	data := []byte("0123456789")
	n, err := nagleWrapper.Write(data)

	// Should return the error from the failed flush
	if err == nil {
		t.Fatalf("expected error from failed write, but got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, but got %v", expectedErr, err)
	}

	// Should return 0 bytes written since flush failed
	if n != 0 {
		t.Fatalf("expected 0 bytes written on flush failure, but got %d", n)
	}

	// Verify the underlying Write was attempted
	if mockRWC.GetWriteCount() != 1 {
		t.Fatalf("expected 1 write attempt, but got %d", mockRWC.GetWriteCount())
	}

	// Buffer should still contain the data for retry
	nagleWrapper.mutex.Lock()
	bufferLen := nagleWrapper.buffer.Len()
	nagleWrapper.mutex.Unlock()

	if bufferLen != len(data) {
		t.Fatalf("expected buffer to contain %d bytes for retry, but got %d", len(data), bufferLen)
	}

	nagleWrapper.Close()
}

func TestNagleWrapper_ManualFlushFailure(t *testing.T) {
	mockRWC := &MockReadWriteCloser{}
	nagleWrapper := NewNagleWrapper(mockRWC, 100, 50*time.Millisecond)

	// Write some data (less than buffer size, no auto-flush)
	data := []byte("test data")
	n, err := nagleWrapper.Write(data)
	if err != nil {
		t.Fatalf("unexpected error on write: %v", err)
	}
	if n != len(data) {
		t.Fatalf("expected to write %d bytes, wrote %d", len(data), n)
	}

	// Verify data is buffered, not flushed yet
	if mockRWC.String() != "" {
		t.Fatalf("expected buffer to be empty before flush, but got: %s", mockRWC.String())
	}

	// Configure mock to fail writes
	expectedErr := errors.New("disk full")
	mockRWC.SetWriteFailure(true, expectedErr)

	// Manual flush should fail
	err = nagleWrapper.Flush()
	if err == nil {
		t.Fatalf("expected error from manual flush, but got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, but got %v", expectedErr, err)
	}

	// Verify the underlying Write was attempted
	if mockRWC.GetWriteCount() != 1 {
		t.Fatalf("expected 1 write attempt, but got %d", mockRWC.GetWriteCount())
	}

	// Buffer should still contain the data after failed flush
	nagleWrapper.mutex.Lock()
	bufferLen := nagleWrapper.buffer.Len()
	nagleWrapper.mutex.Unlock()

	if bufferLen != len(data) {
		t.Fatalf("expected buffer to contain %d bytes after failed flush, but got %d", len(data), bufferLen)
	}

	// Fix the write issue and retry flush
	mockRWC.SetWriteFailure(false, nil)
	err = nagleWrapper.Flush()
	if err != nil {
		t.Fatalf("unexpected error on retry flush: %v", err)
	}

	// Now data should be flushed successfully
	if mockRWC.String() != "test data" {
		t.Fatalf("expected buffer to contain 'test data' after successful flush, but got: %s", mockRWC.String())
	}

	// Buffer should be empty after successful flush
	nagleWrapper.mutex.Lock()
	bufferLen = nagleWrapper.buffer.Len()
	nagleWrapper.mutex.Unlock()

	if bufferLen != 0 {
		t.Fatalf("expected buffer to be empty after successful flush, but got %d bytes", bufferLen)
	}

	nagleWrapper.Close()
}
