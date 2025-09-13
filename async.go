package usb

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// AsyncTransfer represents an asynchronous USB transfer
type AsyncTransfer struct {
	handle       *DeviceHandle
	endpoint     uint8
	transferType TransferType
	buffer       []byte
	timeout      time.Duration
	actualLength int
	isoPackets   []IsoPacket

	// URB fields
	urb       *URB
	urbBuffer []byte // Holds URB struct (+ iso packets if needed)
	submitted bool

	// Auto-reaping support
	reapErr  error
	reaped   bool
	reapCond *sync.Cond
}

// IsoPacket represents an isochronous packet
type IsoPacket struct {
	Length       int
	ActualLength int
	Status       int
}

// NewBulkTransfer creates a new bulk transfer
func (h *DeviceHandle) NewBulkTransfer(endpoint uint8, bufferSize int) (*AsyncTransfer, error) {
	return h.newAsyncTransfer(endpoint, TransferTypeBulk, bufferSize, 0)
}

// NewInterruptTransfer creates a new interrupt transfer
func (h *DeviceHandle) NewInterruptTransfer(endpoint uint8, bufferSize int) (*AsyncTransfer, error) {
	return h.newAsyncTransfer(endpoint, TransferTypeInterrupt, bufferSize, 0)
}

// NewControlTransfer creates a new control transfer
func (h *DeviceHandle) NewControlTransfer(bufferSize int) (*AsyncTransfer, error) {
	return h.newAsyncTransfer(0, TransferTypeControl, bufferSize, 0)
}

// newAsyncTransfer creates a new asynchronous transfer
func (h *DeviceHandle) newAsyncTransfer(endpoint uint8, transferType TransferType, bufferSize int, isoPackets int) (*AsyncTransfer, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, ErrDeviceNotFound
	}

	transfer := &AsyncTransfer{
		handle:       h,
		endpoint:     endpoint,
		transferType: transferType,
		buffer:       make([]byte, bufferSize),
		timeout:      5 * time.Second,
		reapCond:     sync.NewCond(&sync.Mutex{}),
	}

	// Calculate URB size
	urbSize := unsafe.Sizeof(URB{})
	if transferType == TransferTypeIsochronous && isoPackets > 0 {
		// Add space for iso packet descriptors
		urbSize += uintptr(isoPackets) * unsafe.Sizeof(IsoPacketDescriptor{})
		transfer.isoPackets = make([]IsoPacket, isoPackets)
	}

	// Allocate URB buffer
	transfer.urbBuffer = make([]byte, urbSize)
	transfer.urb = (*URB)(unsafe.Pointer(&transfer.urbBuffer[0]))

	// Set up URB fields
	switch transferType {
	case TransferTypeBulk:
		transfer.urb.Type = USBDEVFS_URB_TYPE_BULK
	case TransferTypeInterrupt:
		transfer.urb.Type = USBDEVFS_URB_TYPE_INTERRUPT
	case TransferTypeControl:
		transfer.urb.Type = USBDEVFS_URB_TYPE_CONTROL
	case TransferTypeIsochronous:
		transfer.urb.Type = USBDEVFS_URB_TYPE_ISO
		transfer.urb.NumberOfPackets = int32(isoPackets)
		transfer.urb.Flags = USBDEVFS_URB_ISO_ASAP
	}

	transfer.urb.Endpoint = endpoint
	transfer.urb.Buffer = unsafe.Pointer(&transfer.buffer[0])
	transfer.urb.BufferLength = int32(bufferSize)

	return transfer, nil
}

// SetTimeout sets the transfer timeout
func (t *AsyncTransfer) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}

// GetStatus returns the transfer status
func (t *AsyncTransfer) GetStatus() TransferStatus {
	t.waitForReaping()

	if t.reapErr != nil {
		if t.urb.Status == -int32(syscall.ETIMEDOUT) {
			return TransferTimedOut
		}
		return TransferError
	}
	return TransferCompleted
}

// GetActualLength returns actual bytes transferred
func (t *AsyncTransfer) GetActualLength() int {
	t.waitForReaping()
	return t.actualLength
}

// GetBuffer returns the transfer buffer
func (t *AsyncTransfer) GetBuffer() []byte {
	t.waitForReaping()
	return t.buffer[:t.actualLength]
}

// GetIsoPackets returns isochronous packets
func (t *AsyncTransfer) GetIsoPackets() []IsoPacket {
	t.waitForReaping()
	return t.isoPackets
}

// Submit submits the transfer for execution
func (t *AsyncTransfer) Submit() error {
	if t.submitted {
		return fmt.Errorf("transfer already submitted")
	}

	t.handle.mu.RLock()
	defer t.handle.mu.RUnlock()

	if t.handle.closed {
		return ErrDeviceNotFound
	}

	// Reset URB fields
	t.urb.Status = 0
	t.urb.ActualLength = 0
	t.urb.ErrorCount = 0

	// Set up iso packets if needed
	if t.transferType == TransferTypeIsochronous && len(t.isoPackets) > 0 {
		isoPackets := (*[1 << 16]IsoPacketDescriptor)(unsafe.Pointer(
			uintptr(unsafe.Pointer(t.urb)) + unsafe.Sizeof(URB{})))
		for i := range t.isoPackets {
			isoPackets[i].Length = uint32(t.isoPackets[i].Length)
			isoPackets[i].ActualLength = 0
			isoPackets[i].Status = 0
		}
	}

	// Submit URB to kernel
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(t.handle.fd),
		USBDEVFS_SUBMITURB,
		uintptr(unsafe.Pointer(t.urb)),
	)

	if errno != 0 {
		return fmt.Errorf("failed to submit URB: %v", errno)
	}

	t.submitted = true
	t.reaped = false

	// Register with centralized reaper
	t.handle.registerURBCompletion(uintptr(unsafe.Pointer(t.urb)), func(err error) {
		// Process URB completion
		t.reapCond.L.Lock()
		defer t.reapCond.L.Unlock()

		t.reapErr = err

		if err == nil {
			t.actualLength = int(t.urb.ActualLength)

			// Update iso packets if needed
			if t.transferType == TransferTypeIsochronous && len(t.isoPackets) > 0 {
				isoPackets := (*[1 << 16]IsoPacketDescriptor)(unsafe.Pointer(
					uintptr(unsafe.Pointer(t.urb)) + unsafe.Sizeof(URB{})))
				for i := range t.isoPackets {
					t.isoPackets[i].ActualLength = int(isoPackets[i].ActualLength)
					t.isoPackets[i].Status = int(isoPackets[i].Status)
				}
			}
		}

		// Clear submitted flag to allow resubmission
		t.submitted = false
		t.reaped = true
		t.reapCond.Broadcast()
	})

	return nil
}

// Cancel cancels the transfer
func (t *AsyncTransfer) Cancel() error {
	if !t.submitted {
		return fmt.Errorf("transfer not submitted")
	}

	t.handle.mu.RLock()
	defer t.handle.mu.RUnlock()

	if t.handle.closed {
		return ErrDeviceNotFound
	}

	// Discard the URB
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(t.handle.fd),
		USBDEVFS_DISCARDURB,
		uintptr(unsafe.Pointer(t.urb)),
	)

	if errno != 0 && errno != syscall.EINVAL {
		return fmt.Errorf("failed to cancel URB: %v", errno)
	}

	return nil
}

// waitForReaping waits for the transfer to be reaped
func (t *AsyncTransfer) waitForReaping() {
	t.reapCond.L.Lock()
	defer t.reapCond.L.Unlock()

	for !t.reaped {
		t.reapCond.Wait()
	}
}

// Wait waits for transfer completion with timeout
func (t *AsyncTransfer) Wait() error {
	t.waitForReaping()
	return t.reapErr
}

// WaitWithTimeout waits for transfer completion with timeout
func (t *AsyncTransfer) WaitWithTimeout(timeout time.Duration) error {
	done := make(chan error, 1)

	go func() {
		t.waitForReaping()
		done <- t.reapErr
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		// Try to cancel the transfer
		t.Cancel()
		return ErrTimeout
	}
}

// Fill fills the buffer with data (for OUT transfers)
func (t *AsyncTransfer) Fill(data []byte) error {
	if len(data) > len(t.buffer) {
		return fmt.Errorf("data too large for buffer")
	}
	copy(t.buffer, data)
	// Update the buffer length for the actual data size
	t.urb.BufferLength = int32(len(data))
	return nil
}

// SetIsoPacketLengths sets the length for all isochronous packets
func (t *AsyncTransfer) SetIsoPacketLengths(length int) {
	for i := range t.isoPackets {
		t.isoPackets[i].Length = length
	}
}
