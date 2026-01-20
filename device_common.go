package usb

import (
	"time"
)

// controlSetupPacket represents a USB control setup packet
type controlSetupPacket struct {
	bmRequestType uint8
	bRequest      uint8
	wValue        uint16
	wIndex        uint16
	wLength       uint16
}

// DeviceHandleInterface defines the common interface for device operations
// that must be implemented by platform-specific code
type DeviceHandleInterface interface {
	Close() error
	SetConfiguration(config int) error
	GetConfiguration() (int, error)
	ClaimInterface(iface uint8) error
	ReleaseInterface(iface uint8) error
	SetAltSetting(iface, altSetting uint8) error
	ClearHalt(endpoint uint8) error
	ResetDevice() error
	KernelDriverActive(iface uint8) (bool, error)
	DetachKernelDriver(iface uint8) error
	AttachKernelDriver(iface uint8) error
	ControlTransfer(requestType, request uint8, value, index uint16, data []byte, timeout time.Duration) (int, error)
	BulkTransfer(endpoint uint8, data []byte, timeout time.Duration) (int, error)
	InterruptTransfer(endpoint uint8, data []byte, timeout time.Duration) (int, error)
	StringDescriptor(index uint8) (string, error)
	GetDeviceDescriptor() (*DeviceDescriptor, error)
	GetActiveConfigDescriptor() (*ConfigDescriptor, error)
	GetConfigDescriptor(index uint8) (*ConfigDescriptor, error)
}
