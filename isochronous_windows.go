package usb

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// IsochronousTransfer represents an isochronous USB transfer.
// On Windows, isochronous transfers are not supported through WinUSB.
type IsochronousTransfer struct {
	handle     *DeviceHandle
	endpoint   uint8
	numPackets int
	packetSize int
}

// IsoPacket represents a single isochronous packet result
type IsoPacket struct {
	Data         []byte
	Status       int32
	ActualLength int
}

// NewIsochronousTransfer creates a new isochronous transfer.
// On Windows, this returns an error as WinUSB does not support isochronous transfers.
func (h *DeviceHandle) NewIsochronousTransfer(endpoint uint8, numPackets int, packetSize int) (*IsochronousTransfer, error) {
	return nil, fmt.Errorf("isochronous transfers are not supported on Windows through WinUSB")
}

// Submit submits the isochronous transfer.
func (t *IsochronousTransfer) Submit() error {
	return fmt.Errorf("isochronous transfers are not supported on Windows")
}

// Wait waits for the isochronous transfer to complete.
func (t *IsochronousTransfer) Wait() error {
	return fmt.Errorf("isochronous transfers are not supported on Windows")
}

// Cancel cancels the isochronous transfer.
func (t *IsochronousTransfer) Cancel() error {
	return nil
}

// Packets returns the packet results.
func (t *IsochronousTransfer) Packets() []IsoPacket {
	return nil
}

// IsoPacketBuffer returns the buffer for a specific packet.
func (t *IsochronousTransfer) IsoPacketBuffer(index int) ([]byte, error) {
	return nil, fmt.Errorf("isochronous transfers are not supported on Windows")
}

// Read reads data from the isochronous transfer.
func (t *IsochronousTransfer) Read(buf []byte) (int, error) {
	return 0, fmt.Errorf("isochronous transfers are not supported on Windows")
}

// Write writes data to the isochronous transfer.
func (t *IsochronousTransfer) Write(buf []byte) (int, error) {
	return 0, fmt.Errorf("isochronous transfers are not supported on Windows")
}

// Close closes the isochronous transfer.
func (t *IsochronousTransfer) Close() error {
	return nil
}

// AsyncBulkTransfer represents an asynchronous bulk USB transfer.
// On Windows, this simulates async behavior using synchronous transfers.
type AsyncBulkTransfer struct {
	handle     *DeviceHandle
	endpoint   uint8
	bufferSize int
	buffer     []byte
	result     []byte
	resultErr  error
	submitted  bool
	completed  bool
	closed     bool
	mu         sync.Mutex
	cond       *sync.Cond
}

// NewAsyncBulkTransfer creates a new async bulk transfer.
// On Windows, this uses synchronous transfers internally but provides an async-like interface.
func (h *DeviceHandle) NewAsyncBulkTransfer(endpoint uint8, bufferSize int) (*AsyncBulkTransfer, error) {
	if h.closed {
		return nil, ErrDeviceNotFound
	}

	t := &AsyncBulkTransfer{
		handle:     h,
		endpoint:   endpoint,
		bufferSize: bufferSize,
		buffer:     make([]byte, bufferSize),
	}
	t.cond = sync.NewCond(&t.mu)

	return t, nil
}

// Submit submits the bulk transfer.
// On Windows, this starts a goroutine that performs the transfer.
func (t *AsyncBulkTransfer) Submit() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return io.EOF
	}

	if t.submitted && !t.completed {
		return fmt.Errorf("transfer already submitted")
	}

	t.submitted = true
	t.completed = false
	t.result = nil
	t.resultErr = nil

	// Perform transfer in background
	go func() {
		n, err := t.handle.BulkTransfer(t.endpoint, t.buffer, 5*time.Second)

		t.mu.Lock()
		if err != nil {
			t.resultErr = err
		} else {
			t.result = make([]byte, n)
			copy(t.result, t.buffer[:n])
		}
		t.completed = true
		t.cond.Broadcast()
		t.mu.Unlock()
	}()

	return nil
}

// Wait waits for the transfer to complete and returns the result.
func (t *AsyncBulkTransfer) Wait() ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for !t.completed && !t.closed {
		t.cond.Wait()
	}

	if t.closed {
		return nil, io.EOF
	}

	return t.result, t.resultErr
}

// Cancel cancels the transfer.
func (t *AsyncBulkTransfer) Cancel() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// On Windows, we can't truly cancel - just mark as completed with error
	if t.submitted && !t.completed {
		t.resultErr = fmt.Errorf("transfer cancelled")
		t.completed = true
		t.cond.Broadcast()
	}

	return nil
}

// ActualLength returns the number of bytes actually transferred.
func (t *AsyncBulkTransfer) ActualLength() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.result != nil {
		return len(t.result)
	}
	return 0
}

// Read reads data from the async bulk transfer.
// On Windows, this performs a synchronous bulk transfer.
func (t *AsyncBulkTransfer) Read(buf []byte) (int, error) {
	if t.closed {
		return 0, io.EOF
	}

	if t.handle.closed {
		return 0, ErrDeviceNotFound
	}

	// Perform synchronous bulk transfer
	return t.handle.BulkTransfer(t.endpoint, buf, 5*time.Second)
}

// Close closes the async bulk transfer.
func (t *AsyncBulkTransfer) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true
	t.cond.Broadcast()
	return nil
}
