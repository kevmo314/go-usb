package usb

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrDeviceNotFound     = errors.New("device not found")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrDeviceBusy         = errors.New("device busy")
	ErrInvalidParameter   = errors.New("invalid parameter")
	ErrIO                 = errors.New("I/O error")
	ErrNotSupported       = errors.New("operation not supported")
	ErrTimeout            = errors.New("operation timed out")
	ErrPipe               = errors.New("pipe error")
	ErrInterrupted        = errors.New("interrupted")
	ErrNoMemory           = errors.New("out of memory")
	ErrOther              = errors.New("unknown error")
)

type TransferType uint8

const (
	TransferTypeControl     TransferType = 0
	TransferTypeIsochronous TransferType = 1
	TransferTypeBulk        TransferType = 2
	TransferTypeInterrupt   TransferType = 3
)

// USB descriptor types
const (
	USB_DT_DEVICE               = 0x01
	USB_DT_CONFIG               = 0x02
	USB_DT_STRING               = 0x03
	USB_DT_INTERFACE            = 0x04
	USB_DT_ENDPOINT             = 0x05
	USB_DT_DEVICE_QUALIFIER     = 0x06
	USB_DT_OTHER_SPEED_CONFIG   = 0x07
	USB_DT_INTERFACE_POWER      = 0x08
	USB_DT_OTG                  = 0x09
	USB_DT_DEBUG                = 0x0A
	USB_DT_INTERFACE_ASSOC      = 0x0B
	USB_DT_SECURITY             = 0x0C
	USB_DT_KEY                  = 0x0D
	USB_DT_ENCRYPTION_TYPE      = 0x0E
	USB_DT_BOS                  = 0x0F
	USB_DT_DEVICE_CAPABILITY    = 0x10
	USB_DT_WIRELESS_ENDPOINT_COMP = 0x11
	USB_DT_WIRE_ADAPTER         = 0x21
	USB_DT_RPIPE                = 0x22
	USB_DT_CS_RADIO_CONTROL     = 0x23
	USB_DT_SS_ENDPOINT_COMP     = 0x30
)

// USB standard requests
const (
	USB_REQ_GET_STATUS          = 0x00
	USB_REQ_CLEAR_FEATURE       = 0x01
	USB_REQ_SET_FEATURE         = 0x03
	USB_REQ_SET_ADDRESS         = 0x05
	USB_REQ_GET_DESCRIPTOR      = 0x06
	USB_REQ_SET_DESCRIPTOR      = 0x07
	USB_REQ_GET_CONFIGURATION   = 0x08
	USB_REQ_SET_CONFIGURATION   = 0x09
	USB_REQ_GET_INTERFACE       = 0x0A
	USB_REQ_SET_INTERFACE       = 0x0B
	USB_REQ_SYNCH_FRAME         = 0x0C
)

// USB feature selectors
const (
	USB_ENDPOINT_HALT           = 0x00
	USB_DEVICE_REMOTE_WAKEUP    = 0x01
	USB_DEVICE_TEST_MODE        = 0x02
	USB_DEVICE_B_HNP_ENABLE     = 0x03
	USB_DEVICE_A_HNP_SUPPORT    = 0x04
	USB_DEVICE_A_ALT_HNP_SUPPORT = 0x05
)

// USB test modes
const (
	USB_TEST_J              = 0x01
	USB_TEST_K              = 0x02
	USB_TEST_SE0_NAK        = 0x03
	USB_TEST_PACKET         = 0x04
	USB_TEST_FORCE_ENABLE   = 0x05
)

type EndpointDirection uint8

const (
	EndpointDirectionOut EndpointDirection = 0
	EndpointDirectionIn  EndpointDirection = 0x80
)

type Context struct {
	mu         sync.RWMutex
	devices    []*Device
	debug      bool
	managers   []*AsyncTransferManager
	eventLoop  chan struct{}
	running    bool
}

func NewContext() (*Context, error) {
	return &Context{
		devices:   make([]*Device, 0),
		debug:     false,
		managers:  make([]*AsyncTransferManager, 0),
		eventLoop: make(chan struct{}),
	}, nil
}

func (c *Context) SetDebug(debug bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.debug = debug
}

func (c *Context) GetDeviceList() ([]*Device, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Use sysfs enumerator for fast device discovery
	enumerator := NewSysfsEnumerator()
	sysfsDevices, err := enumerator.EnumerateDevices()
	if err != nil {
		return nil, err
	}
	
	devices := make([]*Device, 0, len(sysfsDevices))
	for _, sysfsDevice := range sysfsDevices {
		device := sysfsDevice.ToUSBDevice(c)
		devices = append(devices, device)
	}
	
	c.devices = devices
	return devices, nil
}

func (c *Context) OpenDevice(vendorID, productID uint16) (*DeviceHandle, error) {
	devices, err := c.GetDeviceList()
	if err != nil {
		return nil, err
	}
	
	for _, dev := range devices {
		if dev.Descriptor.VendorID == vendorID && dev.Descriptor.ProductID == productID {
			return dev.Open()
		}
	}
	
	return nil, ErrDeviceNotFound
}

func (c *Context) OpenDeviceWithPath(path string) (*DeviceHandle, error) {
	devices, err := c.GetDeviceList()
	if err != nil {
		return nil, err
	}
	
	for _, dev := range devices {
		if dev.Path == path {
			return dev.Open()
		}
	}
	
	return nil, ErrDeviceNotFound
}

// HandleEvents processes pending events for all transfer managers
func (c *Context) HandleEvents() error {
	return c.HandleEventsTimeout(0)
}

// HandleEventsTimeout processes events with a timeout
func (c *Context) HandleEventsTimeout(timeout time.Duration) error {
	c.mu.RLock()
	managers := make([]*AsyncTransferManager, len(c.managers))
	copy(managers, c.managers)
	c.mu.RUnlock()
	
	// If no managers are registered, still wait the timeout period
	// This matches libusb behavior
	if len(managers) == 0 {
		if timeout == 0 {
			// Non-blocking check - return immediately
			return nil
		}
		// Wait for the specified timeout
		time.Sleep(timeout)
		return nil
	}
	
	// Process events for all managers
	for _, manager := range managers {
		if err := manager.HandleEvents(timeout); err != nil {
			return err
		}
	}
	
	return nil
}

// RegisterAsyncTransferManager registers a new async transfer manager
func (c *Context) RegisterAsyncTransferManager(manager *AsyncTransferManager) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.managers = append(c.managers, manager)
}

// UnregisterAsyncTransferManager removes an async transfer manager
func (c *Context) UnregisterAsyncTransferManager(manager *AsyncTransferManager) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for i, m := range c.managers {
		if m == manager {
			c.managers = append(c.managers[:i], c.managers[i+1:]...)
			break
		}
	}
}

func (c *Context) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Stop all transfer managers
	for _, manager := range c.managers {
		manager.Close()
	}
	c.managers = nil
	
	// Close all device handles
	for _, dev := range c.devices {
		if dev.handle != nil {
			dev.handle.Close()
		}
	}
	
	c.devices = nil
	return nil
}

func GetVersion() string {
	return "1.0.0"
}

func GetCapabilities() map[string]bool {
	return map[string]bool{
		"has_capability":     true,
		"has_hotplug":        false,
		"has_hid_access":     true,
		"supports_detach_kernel_driver": true,
	}
}

func IsValidDevicePath(path string) bool {
	if !strings.HasPrefix(path, "/dev/bus/usb/") {
		return false
	}
	
	parts := strings.Split(path, "/")
	if len(parts) != 6 {
		return false
	}
	
	busNum, err := strconv.Atoi(parts[4])
	if err != nil || busNum < 0 || busNum > 255 {
		return false
	}
	
	devNum, err := strconv.Atoi(parts[5])
	if err != nil || devNum < 0 || devNum > 255 {
		return false
	}
	
	return true
}