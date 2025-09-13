package usb

import (
	"fmt"
	"strconv"
	"strings"
)

// Version returns the version of the go-usb library
func Version() string {
	return "1.0.0"
}

// Error types
var (
	ErrDeviceNotFound   = fmt.Errorf("device not found")
	ErrPermissionDenied = fmt.Errorf("permission denied")
	ErrDeviceBusy       = fmt.Errorf("device busy")
	ErrEAGAIN           = fmt.Errorf("resource temporarily unavailable")
)

// Speed types
type Speed int

const (
	SpeedUnknown Speed = iota
	SpeedLow
	SpeedFull
	SpeedHigh
	SpeedSuper
	SpeedSuperPlus
)

// Endpoint direction
type EndpointDirection uint8

const (
	EndpointDirectionOut EndpointDirection = 0
	EndpointDirectionIn  EndpointDirection = 0x80
)

// DeviceList returns a list of USB devices
func DeviceList() ([]*Device, error) {
	// Use sysfs enumerator for fast device discovery
	enumerator := NewSysfsEnumerator()
	sysfsDevices, err := enumerator.EnumerateDevices()
	if err != nil {
		return nil, err
	}

	devices := make([]*Device, 0, len(sysfsDevices))
	for _, sysfsDevice := range sysfsDevices {
		device := sysfsDevice.ToUSBDevice()
		devices = append(devices, device)
	}

	return devices, nil
}

// OpenDevice opens a device by vendor and product ID
func OpenDevice(vendorID, productID uint16) (*DeviceHandle, error) {
	devices, err := DeviceList()
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

// OpenDeviceWithPath opens a device by its path
func OpenDeviceWithPath(path string) (*DeviceHandle, error) {
	devices, err := DeviceList()
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

// IsValidDevicePath checks if a path is a valid USB device path
func IsValidDevicePath(path string) bool {
	if !strings.HasPrefix(path, "/dev/bus/usb/") {
		return false
	}

	// Extract bus and device numbers from path
	parts := strings.Split(path, "/")
	if len(parts) != 6 {
		return false
	}

	// Check bus number (parts[4])
	busNum, err := strconv.Atoi(parts[4])
	if err != nil || busNum < 1 || busNum > 255 {
		return false
	}

	// Check device number (parts[5])
	devNum, err := strconv.Atoi(parts[5])
	if err != nil || devNum < 1 || devNum > 255 {
		return false
	}

	return true
}
