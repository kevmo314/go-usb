package usb

import (
	"fmt"
	"time"
)

// ErrTimeout represents a timeout error
var ErrTimeout = fmt.Errorf("transfer timed out")

// ControlTransfer performs a control transfer on the device
func (h *DeviceHandle) ControlTransfer(requestType, request uint8, value, index uint16, data []byte, timeout time.Duration) (int, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, fmt.Errorf("device is closed")
	}

	timeoutMs := uint32(timeout.Milliseconds())
	if timeoutMs == 0 {
		timeoutMs = 5000 // Default 5 second timeout
	}

	return h.devInterface.ControlTransfer(requestType, request, value, index, data, timeoutMs)
}

// BulkTransfer performs a bulk transfer on an endpoint
func (h *DeviceHandle) BulkTransfer(endpoint uint8, data []byte, timeout time.Duration) (int, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, fmt.Errorf("device is closed")
	}

	// Determine interface from endpoint
	// This is simplified - need to track which interface owns which endpoint
	var intf *IOUSBInterfaceInterface
	for _, i := range h.interfaces {
		intf = i
		break
	}

	if intf == nil {
		// No interface claimed, try to auto-claim based on endpoint
		// In a real implementation, we'd need to properly map endpoints to interfaces
		return 0, fmt.Errorf("no interface claimed for endpoint %02x", endpoint)
	}

	timeoutMs := uint32(timeout.Milliseconds())
	if timeoutMs == 0 {
		timeoutMs = 5000 // Default 5 second timeout
	}

	// Determine direction from endpoint address
	if endpoint&0x80 != 0 {
		// IN endpoint
		return intf.BulkTransferIn(endpoint&0x0F, data, timeoutMs)
	} else {
		// OUT endpoint
		return intf.BulkTransferOut(endpoint&0x0F, data, timeoutMs)
	}
}

// InterruptTransfer performs an interrupt transfer on an endpoint
func (h *DeviceHandle) InterruptTransfer(endpoint uint8, data []byte, timeout time.Duration) (int, error) {
	// On macOS, interrupt transfers use the same mechanism as bulk transfers
	// The difference is in the endpoint type, which is handled by IOKit
	return h.BulkTransfer(endpoint, data, timeout)
}

// Transfer represents a USB transfer
type Transfer struct {
	handle       *DeviceHandle
	endpoint     uint8
	transferType TransferType
	buffer       []byte
	status       TransferStatus
	actualLength int
	callback     func(*Transfer)
	userData     interface{}
}

// NewTransfer creates a new transfer
func NewTransfer(handle *DeviceHandle, endpoint uint8, transferType TransferType, bufferSize int) *Transfer {
	return &Transfer{
		handle:       handle,
		endpoint:     endpoint,
		transferType: transferType,
		buffer:       make([]byte, bufferSize),
		status:       TransferError,
	}
}

// SetCallback sets the transfer callback
func (t *Transfer) SetCallback(callback func(*Transfer)) {
	t.callback = callback
}

// SetUserData sets user data for the transfer
func (t *Transfer) SetUserData(data interface{}) {
	t.userData = data
}

// GetUserData gets the user data
func (t *Transfer) GetUserData() interface{} {
	return t.userData
}

// Submit submits the transfer
func (t *Transfer) Submit() error {
	// Simplified synchronous implementation
	// A full implementation would use async IOKit APIs

	var n int
	var err error

	switch t.transferType {
	case TransferTypeControl:
		// Control transfers would need additional setup packet data
		return fmt.Errorf("async control transfers not yet implemented")

	case TransferTypeBulk:
		n, err = t.handle.BulkTransfer(t.endpoint, t.buffer, 5*time.Second)

	case TransferTypeInterrupt:
		n, err = t.handle.InterruptTransfer(t.endpoint, t.buffer, 5*time.Second)

	case TransferTypeIsochronous:
		return fmt.Errorf("isochronous transfers not yet implemented")

	default:
		return fmt.Errorf("unknown transfer type")
	}

	t.actualLength = n
	if err != nil {
		if err == ErrTimeout {
			t.status = TransferTimedOut
		} else {
			t.status = TransferError
		}
	} else {
		t.status = TransferCompleted
	}

	// Call callback if set
	if t.callback != nil {
		t.callback(t)
	}

	return err
}

// Cancel cancels the transfer
func (t *Transfer) Cancel() error {
	// Cancellation would require async API support
	t.status = TransferCancelled
	return nil
}

// Status returns the transfer status
func (t *Transfer) Status() TransferStatus {
	return t.status
}

// ActualLength returns the actual number of bytes transferred
func (t *Transfer) ActualLength() int {
	return t.actualLength
}

// Buffer returns the transfer buffer
func (t *Transfer) Buffer() []byte {
	return t.buffer
}

// Free frees the transfer resources
func (t *Transfer) Free() {
	// Nothing to free in this implementation
}

// SubmitTransfer submits a transfer for asynchronous execution
func (h *DeviceHandle) SubmitTransfer(transfer *Transfer) error {
	// Simplified implementation - just run synchronously for now
	return transfer.Submit()
}

// ReapTransfer waits for a completed transfer
func (h *DeviceHandle) ReapTransfer(timeout time.Duration) (*Transfer, error) {
	// This would need proper async implementation
	return nil, fmt.Errorf("async transfers not fully implemented")
}

// URB structure for macOS (compatibility)
type URB struct {
	Type            uint8
	Endpoint        uint8
	Status          int32
	Flags           uint32
	Buffer          uintptr
	BufferLength    int32
	ActualLength    int32
	StartFrame      int32
	NumberOfPackets int32
	ErrorCount      int32
	SignR           uint32
	UserContext     uintptr
}

// AllocateStreams allocates bulk streams (USB 3.0+)
func (h *DeviceHandle) AllocateStreams(numStreams uint32, endpoints []uint8) error {
	// Stream support would require IOKit USB 3.0 APIs
	return fmt.Errorf("bulk streams not supported on macOS")
}

// FreeStreams frees bulk streams
func (h *DeviceHandle) FreeStreams(endpoints []uint8) error {
	// Stream support would require IOKit USB 3.0 APIs
	return fmt.Errorf("bulk streams not supported on macOS")
}

// Control transfer helpers

// GetStatus performs a GET_STATUS control request
func (h *DeviceHandle) GetStatus(recipient, index uint16) (uint16, error) {
	buf := make([]byte, 2)
	_, err := h.ControlTransfer(
		0x80|(uint8(recipient)&0x1F), // IN, standard, recipient
		USB_REQ_GET_STATUS,
		0,
		index,
		buf,
		5*time.Second,
	)
	if err != nil {
		return 0, err
	}

	return uint16(buf[0]) | (uint16(buf[1]) << 8), nil
}

// ClearFeature performs a CLEAR_FEATURE control request
func (h *DeviceHandle) ClearFeature(recipient, feature, index uint16) error {
	_, err := h.ControlTransfer(
		uint8(recipient)&0x1F, // OUT, standard, recipient
		USB_REQ_CLEAR_FEATURE,
		feature,
		index,
		nil,
		5*time.Second,
	)
	return err
}

// SetFeature performs a SET_FEATURE control request
func (h *DeviceHandle) SetFeature(recipient, feature, index uint16) error {
	_, err := h.ControlTransfer(
		uint8(recipient)&0x1F, // OUT, standard, recipient
		USB_REQ_SET_FEATURE,
		feature,
		index,
		nil,
		5*time.Second,
	)
	return err
}
