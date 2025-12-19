package usb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Common USB errors
var (
	ErrInvalidParameter = errors.New("invalid parameter")
	ErrIO               = errors.New("I/O error")
	ErrNoDevice         = errors.New("no device")
	ErrNotFound         = errors.New("not found")
	ErrBusy             = errors.New("busy")
	ErrTimeout          = errors.New("timeout")
	ErrOverflow         = errors.New("overflow")
	ErrPipe             = errors.New("pipe error")
	ErrInterrupted      = errors.New("interrupted")
	ErrNoMem            = errors.New("no memory")
	ErrNotSupported     = errors.New("not supported")
	ErrOther            = errors.New("other error")
)

type Transfer struct {
	handle       *DeviceHandle
	endpoint     uint8
	transferType TransferType
	buffer       []byte
	length       int
	timeout      time.Duration
	callback     TransferCallback
	userdata     interface{}
	status       TransferStatus
	actualLength int
	mu           sync.Mutex
}

type TransferCallback func(transfer *Transfer)

// Transfer types
type TransferType int

const (
	TransferTypeControl TransferType = iota
	TransferTypeIsochronous
	TransferTypeBulk
	TransferTypeInterrupt
)

type TransferStatus int

const (
	TransferCompleted TransferStatus = iota
	TransferError
	TransferTimedOut
	TransferCancelled
	TransferStall
	TransferNoDevice
	TransferOverflow
	TransferInProgress
)

func (h *DeviceHandle) ControlTransfer(requestType, request uint8, value, index uint16, data []byte, timeout time.Duration) (int, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, ErrDeviceNotFound
	}

	var dataPtr unsafe.Pointer
	dataLen := uint16(len(data))

	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}

	ctrl := usbCtrlRequest{
		RequestType: requestType,
		Request:     request,
		Value:       value,
		Index:       index,
		Length:      dataLen,
		Timeout:     uint32(timeout.Milliseconds()),
		Data:        dataPtr,
	}

	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return 0, errno
	}

	return int(ret), nil
}

func (h *DeviceHandle) BulkTransfer(endpoint uint8, data []byte, timeout time.Duration) (int, error) {
	return h.BulkTransferWithOptions(endpoint, data, timeout, false)
}

// BulkTransferWithOptions performs a bulk transfer with advanced options
func (h *DeviceHandle) BulkTransferWithOptions(endpoint uint8, data []byte, timeout time.Duration, allowZeroLength bool) (int, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, ErrDeviceNotFound
	}

	// Handle zero-length packets
	if len(data) == 0 && !allowZeroLength {
		return 0, ErrInvalidParameter
	}

	var dataPtr uintptr
	if len(data) > 0 {
		dataPtr = uintptr(unsafe.Pointer(&data[0]))
	}

	bulk := usbBulkTransfer{
		Endpoint: uint32(endpoint),
		Length:   uint32(len(data)),
		Timeout:  uint32(timeout.Milliseconds()),
		Data:     dataPtr,
	}

	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_BULK, uintptr(unsafe.Pointer(&bulk)))
	if errno != 0 {
		if errno == syscall.ETIMEDOUT {
			return 0, ErrTimeout
		}
		return 0, errno
	}

	return int(ret), nil
}

func (h *DeviceHandle) InterruptTransfer(endpoint uint8, data []byte, timeout time.Duration) (int, error) {
	return h.InterruptTransferWithRetry(endpoint, data, timeout, 1)
}

// InterruptTransferWithRetry performs interrupt transfer with automatic retry
func (h *DeviceHandle) InterruptTransferWithRetry(endpoint uint8, data []byte, timeout time.Duration, maxRetries int) (int, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		n, err := h.BulkTransfer(endpoint, data, timeout) // Interrupt uses same mechanism as bulk
		if err == nil {
			return n, nil
		}

		lastErr = err

		// Don't retry for certain errors
		if err == ErrDeviceNotFound || err == ErrInvalidParameter {
			break
		}

		// For timeout or I/O errors, try to recover
		if err == ErrTimeout || err == ErrIO {
			// Try to clear endpoint halt condition
			if clearErr := h.ClearHalt(endpoint); clearErr != nil {
				// If we can't clear halt, don't continue
				break
			}
		}
	}

	return 0, lastErr
}

// ResetDevice performs a USB device reset
func (h *DeviceHandle) ResetDevice() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	// Close and reopen the device file descriptor
	oldFd := h.fd

	fd, err := syscall.Open(h.device.Path, syscall.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to reopen device: %w", err)
	}

	h.fd = fd
	syscall.Close(oldFd)

	// Clear claimed interfaces state since reset releases all
	h.claimedIfaces = make(map[uint8]bool)

	return nil
}

// ResetEndpoint resets a specific endpoint
func (h *DeviceHandle) ResetEndpoint(endpoint uint8) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	ep := uint32(endpoint)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_RESETEP, uintptr(unsafe.Pointer(&ep)))
	if errno != 0 {
		return errno
	}

	return nil
}

// SetShortPacketMode configures short packet handling (Linux 4.6+)
func (h *DeviceHandle) SetShortPacketMode(enabled bool) error {
	// This would require newer usbfs capabilities
	// For now, return not supported
	return ErrNotSupported
}

// High-bandwidth isochronous transfer support
type HighBandwidthIsoTransfer struct {
	Endpoint        uint8
	PacketsPerFrame uint8 // 1-3 for high bandwidth
	PacketSize      uint16
	NumFrames       uint16
	Buffer          []byte
}

// SubmitHighBandwidthIso submits a high-bandwidth isochronous transfer (USB 2.0+)
func (h *DeviceHandle) SubmitHighBandwidthIso(transfer *HighBandwidthIsoTransfer, callback func([]byte, error)) error {
	// This would require complex URB handling for high-bandwidth transfers
	// For now, return not supported - would need full URB implementation
	return ErrNotSupported
}

func (h *DeviceHandle) IsochronousTransfer(endpoint uint8, data []byte, numPackets int, packetSize int, timeout time.Duration) ([]IsoPacketResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, ErrDeviceNotFound
	}

	if numPackets <= 0 || packetSize <= 0 {
		return nil, ErrInvalidParameter
	}

	// For now, return not supported - full implementation would require
	// proper URB handling with iso packet descriptors
	return nil, ErrNotSupported
}

type IsoPacketResult struct {
	Length       int
	ActualLength int
	Status       int
}

func (h *DeviceHandle) SubmitTransfer(transfer *Transfer) error {
	// TODO: Implement async transfer submission
	return ErrNotSupported
}

func (h *DeviceHandle) CancelTransfer(transfer *Transfer) error {
	// TODO: Implement async transfer cancellation
	return ErrNotSupported
}

func (h *DeviceHandle) ReapTransfer(timeout time.Duration) (*Transfer, error) {
	// TODO: Implement async transfer completion
	return nil, ErrNotSupported
}

func NewTransfer(handle *DeviceHandle, endpoint uint8, transferType TransferType, bufferSize int) *Transfer {
	return &Transfer{
		handle:       handle,
		endpoint:     endpoint,
		transferType: transferType,
		buffer:       make([]byte, bufferSize),
		timeout:      5 * time.Second,
		status:       TransferCompleted,
	}
}

func (t *Transfer) SetBuffer(data []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buffer = data
	t.length = len(data)
}

func (t *Transfer) SetCallback(callback TransferCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.callback = callback
}

func (t *Transfer) SetTimeout(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.timeout = timeout
}

func (t *Transfer) SetUserData(userdata interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.userdata = userdata
}

func (t *Transfer) Status() TransferStatus {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

func (t *Transfer) ActualLength() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.actualLength
}

func (t *Transfer) Buffer() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.actualLength > 0 && t.actualLength <= len(t.buffer) {
		return t.buffer[:t.actualLength]
	}
	return t.buffer
}

type usbBulkTransfer struct {
	Endpoint uint32
	Length   uint32
	Timeout  uint32
	Data     uintptr
}

func (h *DeviceHandle) ReadConfigDescriptor(configIndex uint8) (*ConfigDescriptor, []InterfaceDescriptor, []EndpointDescriptor, error) {
	buf := make([]byte, 512)

	ctrl := usbCtrlRequest{
		RequestType: 0x80,
		Request:     0x06,
		Value:       (0x02 << 8) | uint16(configIndex),
		Index:       0,
		Length:      uint16(len(buf)),
		Data:        unsafe.Pointer(&buf[0]),
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return nil, nil, nil, errno
	}

	if len(buf) < 9 {
		return nil, nil, nil, fmt.Errorf("invalid config descriptor")
	}

	config := &ConfigDescriptor{
		Length:             buf[0],
		DescriptorType:     buf[1],
		TotalLength:        binary.LittleEndian.Uint16(buf[2:4]),
		NumInterfaces:      buf[4],
		ConfigurationValue: buf[5],
		ConfigurationIndex: buf[6],
		Attributes:         buf[7],
		MaxPower:           buf[8],
	}

	interfaces := []InterfaceDescriptor{}
	endpoints := []EndpointDescriptor{}

	pos := int(config.Length)
	for pos < int(config.TotalLength) && pos < len(buf) {
		if pos+2 > len(buf) {
			break
		}

		length := int(buf[pos])
		descType := buf[pos+1]

		if pos+length > len(buf) {
			break
		}

		switch descType {
		case 0x04:
			if length >= 9 {
				iface := InterfaceDescriptor{
					Length:            buf[pos],
					DescriptorType:    buf[pos+1],
					InterfaceNumber:   buf[pos+2],
					AlternateSetting:  buf[pos+3],
					NumEndpoints:      buf[pos+4],
					InterfaceClass:    buf[pos+5],
					InterfaceSubClass: buf[pos+6],
					InterfaceProtocol: buf[pos+7],
					InterfaceIndex:    buf[pos+8],
				}
				interfaces = append(interfaces, iface)
			}
		case 0x05:
			if length >= 7 {
				ep := EndpointDescriptor{
					Length:         buf[pos],
					DescriptorType: buf[pos+1],
					EndpointAddr:   buf[pos+2],
					Attributes:     buf[pos+3],
					MaxPacketSize:  binary.LittleEndian.Uint16(buf[pos+4 : pos+6]),
					Interval:       buf[pos+6],
				}
				endpoints = append(endpoints, ep)
			}
		}

		pos += length
	}

	return config, interfaces, endpoints, nil
}
