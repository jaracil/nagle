package nagle

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

// MockReadWriteCloser mocks an io.ReadWriteCloser for testing purposes.
type MockReadWriteCloser struct {
	buffer bytes.Buffer
	closed bool
}

func (m *MockReadWriteCloser) Write(p []byte) (int, error) {
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.buffer.Write(p)
}

func (m *MockReadWriteCloser) Read(p []byte) (int, error) {
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.buffer.Read(p)
}

func (m *MockReadWriteCloser) Close() error {
	if m.closed {
		return io.ErrClosedPipe
	}
	m.closed = true
	return nil
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
	if mockRWC.buffer.String() != "0123456789" {
		t.Fatalf("expected buffer to contain '0123456789', but got: %s", mockRWC.buffer.String())
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
		if mockRWC.buffer.String() != "" {
			t.Fatalf("expected buffer to be empty, but got: %s", mockRWC.buffer.String())
		}

		// Wait for flush timeout
		time.Sleep(100 * time.Millisecond)

		// Buffer should be flushed now
		if mockRWC.buffer.String() != "01234" {
			t.Fatalf("expected buffer to contain '01234', but got: %s", mockRWC.buffer.String())
		}
		mockRWC.buffer.Reset()
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
	if mockRWC.buffer.String() != "01234" {
		t.Fatalf("expected buffer to contain '01234', but got: %s", mockRWC.buffer.String())
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
