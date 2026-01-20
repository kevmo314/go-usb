package usb

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <IOKit/usb/IOUSBLib.h>
#include <CoreFoundation/CoreFoundation.h>

// Forward declaration of function defined in iokit_darwin.go
extern void ReleaseService(io_service_t service);
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"sync"
)

// DeviceHandle represents an open USB device on macOS
type DeviceHandle struct {
	device        *Device
	devInterface  *IOUSBDeviceInterface
	service       C.io_service_t
	interfaces    map[uint8]*IOUSBInterfaceInterface
	claimedIfaces map[uint8]bool
	mu            sync.RWMutex
	closed        bool
	asyncSource   C.CFRunLoopSourceRef
}

// Close closes the device handle
func (h *DeviceHandle) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}

	// Remove async source from run loop if present
	if h.asyncSource != 0 {
		C.CFRunLoopRemoveSource(C.CFRunLoopGetCurrent(), h.asyncSource, C.kCFRunLoopDefaultMode)
		C.CFRelease(C.CFTypeRef(h.asyncSource))
		h.asyncSource = 0
	}

	// Release all claimed interfaces
	for iface := range h.claimedIfaces {
		h.releaseInterfaceInternal(iface)
	}

	// Close device
	if h.devInterface != nil {
		h.devInterface.Close()
		h.devInterface.Release()
		h.devInterface = nil
	}

	// Release service
	if h.service != 0 {
		C.ReleaseService(h.service)
		h.service = 0
	}

	h.closed = true
	return nil
}

// SetConfiguration sets the device configuration
func (h *DeviceHandle) SetConfiguration(config int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("device is closed")
	}

	return h.devInterface.SetConfiguration(uint8(config))
}

// GetConfiguration gets the current device configuration
func (h *DeviceHandle) GetConfiguration() (int, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, fmt.Errorf("device is closed")
	}

	config, err := h.devInterface.GetConfiguration()
	return int(config), err
}

// ClaimInterface claims an interface for exclusive use
func (h *DeviceHandle) ClaimInterface(iface uint8) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("device is closed")
	}

	if h.claimedIfaces[iface] {
		return nil // Already claimed
	}

	// Find and open the interface
	// Note: This is a simplified implementation
	// A full implementation would iterate through interfaces properly

	h.claimedIfaces[iface] = true
	return nil
}

// ReleaseInterface releases a previously claimed interface
func (h *DeviceHandle) ReleaseInterface(iface uint8) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("device is closed")
	}

	return h.releaseInterfaceInternal(iface)
}

func (h *DeviceHandle) releaseInterfaceInternal(iface uint8) error {
	if !h.claimedIfaces[iface] {
		return nil // Not claimed
	}

	// Close interface if it's open
	if intf, ok := h.interfaces[iface]; ok {
		intf.Close()
		intf.Release()
		delete(h.interfaces, iface)
	}

	delete(h.claimedIfaces, iface)
	return nil
}

// SetAltSetting sets the alternate setting for an interface
func (h *DeviceHandle) SetAltSetting(iface, altSetting uint8) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("device is closed")
	}

	if !h.claimedIfaces[iface] {
		return fmt.Errorf("interface %d not claimed", iface)
	}

	intf, ok := h.interfaces[iface]
	if !ok {
		return fmt.Errorf("interface %d not open", iface)
	}

	return intf.SetAlternateSetting(altSetting)
}

// ClearHalt clears a halt/stall condition on an endpoint
func (h *DeviceHandle) ClearHalt(endpoint uint8) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("device is closed")
	}

	// Determine interface from endpoint
	// This is simplified - a full implementation would track endpoint-to-interface mapping
	for _, intf := range h.interfaces {
		// Try to clear on this interface
		// The pipeRef would need to be determined from endpoint address
		err := intf.ClearPipeStall(endpoint & 0x0F)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("endpoint %02x not found", endpoint)
}

// ResetDevice resets the USB device
func (h *DeviceHandle) ResetDevice() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return fmt.Errorf("device is closed")
	}

	return h.devInterface.ResetDevice()
}

// KernelDriverActive checks if a kernel driver is active for an interface
func (h *DeviceHandle) KernelDriverActive(iface uint8) (bool, error) {
	// macOS doesn't expose this in the same way as Linux
	// Interfaces claimed by the system will fail to open
	return false, nil
}

// DetachKernelDriver detaches the kernel driver from an interface
func (h *DeviceHandle) DetachKernelDriver(iface uint8) error {
	// Not directly supported on macOS
	// The system manages drivers differently
	return nil
}

// AttachKernelDriver re-attaches the kernel driver to an interface
func (h *DeviceHandle) AttachKernelDriver(iface uint8) error {
	// Not directly supported on macOS
	// The system manages drivers differently
	return nil
}

// StringDescriptor retrieves a string descriptor from the device
func (h *DeviceHandle) StringDescriptor(index uint8) (string, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return "", fmt.Errorf("device is closed")
	}

	// First get language ID (index 0)
	langID := uint16(0x0409) // Default to US English
	if index == 0 {
		// Get supported languages
		buf := make([]byte, 256)
		_, err := h.devInterface.ControlTransfer(
			0x80,   // Device-to-host, standard, device
			0x06,   // GET_DESCRIPTOR
			0x0300, // String descriptor, index 0
			0,
			buf,
			5000,
		)
		if err != nil {
			return "", err
		}

		if len(buf) >= 4 {
			langID = binary.LittleEndian.Uint16(buf[2:4])
		}
	}

	// Check cached strings first
	if h.device.CachedStrings != nil {
		switch index {
		case h.device.Descriptor.ManufacturerIndex:
			if h.device.CachedStrings.Manufacturer != "" {
				return h.device.CachedStrings.Manufacturer, nil
			}
		case h.device.Descriptor.ProductIndex:
			if h.device.CachedStrings.Product != "" {
				return h.device.CachedStrings.Product, nil
			}
		case h.device.Descriptor.SerialNumberIndex:
			if h.device.CachedStrings.Serial != "" {
				return h.device.CachedStrings.Serial, nil
			}
		}
	}

	// Get the actual string descriptor
	return h.devInterface.GetStringDescriptor(index, langID)
}

// GetDeviceDescriptor retrieves the device descriptor
func (h *DeviceHandle) GetDeviceDescriptor() (*DeviceDescriptor, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, fmt.Errorf("device is closed")
	}

	return h.devInterface.GetDeviceDescriptor()
}

// GetActiveConfigDescriptor gets the descriptor for the active configuration
func (h *DeviceHandle) GetActiveConfigDescriptor() (*ConfigDescriptor, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, fmt.Errorf("device is closed")
	}

	// Get current configuration
	config, err := h.GetConfiguration()
	if err != nil {
		return nil, err
	}

	// Configuration values start at 1, but index starts at 0
	if config > 0 {
		return h.GetConfigDescriptor(uint8(config - 1))
	}

	return h.GetConfigDescriptor(0)
}

// GetConfigDescriptor gets a specific configuration descriptor
func (h *DeviceHandle) GetConfigDescriptor(index uint8) (*ConfigDescriptor, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, fmt.Errorf("device is closed")
	}

	// First get the configuration descriptor header
	buf := make([]byte, 9)
	_, err := h.devInterface.ControlTransfer(
		0x80,                             // Device-to-host, standard, device
		USB_REQ_GET_DESCRIPTOR,           // GET_DESCRIPTOR
		(USB_DT_CONFIG<<8)|uint16(index), // Config descriptor
		0,
		buf,
		5000,
	)
	if err != nil {
		return nil, err
	}

	// Parse total length
	totalLength := binary.LittleEndian.Uint16(buf[2:4])

	// Get full descriptor
	fullBuf := make([]byte, totalLength)
	_, err = h.devInterface.ControlTransfer(
		0x80,
		USB_REQ_GET_DESCRIPTOR,
		(USB_DT_CONFIG<<8)|uint16(index),
		0,
		fullBuf,
		5000,
	)
	if err != nil {
		return nil, err
	}

	// Parse the configuration descriptor
	return parseConfigDescriptor(fullBuf)
}

// ResetEndpoint resets an endpoint
func (h *DeviceHandle) ResetEndpoint(endpoint uint8) error {
	// On macOS, this is handled through ClearHalt
	return h.ClearHalt(endpoint)
}

// parseConfigDescriptor parses a raw configuration descriptor
func parseConfigDescriptor(data []byte) (*ConfigDescriptor, error) {
	if len(data) < 9 {
		return nil, fmt.Errorf("config descriptor too short")
	}

	config := &ConfigDescriptor{
		Length:             data[0],
		DescriptorType:     data[1],
		TotalLength:        binary.LittleEndian.Uint16(data[2:4]),
		NumInterfaces:      data[4],
		ConfigurationValue: data[5],
		ConfigurationIndex: data[6],
		Attributes:         data[7],
		MaxPower:           data[8],
		Interfaces:         make([]Interface, 0),
	}

	// Map to track interfaces by number
	interfaceMap := make(map[uint8]*Interface)

	// Parse interfaces
	offset := 9
	for offset < len(data) {
		if offset+1 >= len(data) {
			break
		}

		length := int(data[offset])
		descType := data[offset+1]

		if length == 0 || offset+length > len(data) {
			break
		}

		if descType == USB_DT_INTERFACE && length >= 9 {
			altSetting := InterfaceAltSetting{
				Length:            data[offset],
				DescriptorType:    data[offset+1],
				InterfaceNumber:   data[offset+2],
				AlternateSetting:  data[offset+3],
				NumEndpoints:      data[offset+4],
				InterfaceClass:    data[offset+5],
				InterfaceSubClass: data[offset+6],
				InterfaceProtocol: data[offset+7],
				InterfaceIndex:    data[offset+8],
				Endpoints:         make([]Endpoint, 0),
			}

			// Get or create interface
			intfNum := altSetting.InterfaceNumber
			if _, exists := interfaceMap[intfNum]; !exists {
				interfaceMap[intfNum] = &Interface{
					AltSettings: make([]InterfaceAltSetting, 0),
				}
			}
			interfaceMap[intfNum].AltSettings = append(interfaceMap[intfNum].AltSettings, altSetting)
		}

		offset += length
	}

	// Convert map to sorted slice
	for i := uint8(0); i < config.NumInterfaces; i++ {
		if intf, exists := interfaceMap[i]; exists {
			config.Interfaces = append(config.Interfaces, *intf)
		}
	}

	return config, nil
}

// Additional descriptor parsing functions

// GetBOSDescriptor retrieves the Binary Object Store descriptor
func (h *DeviceHandle) GetBOSDescriptor() (*BOSDescriptor, []DeviceCapabilityDescriptor, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, nil, fmt.Errorf("device is closed")
	}

	// First get BOS descriptor header
	buf := make([]byte, 5)
	_, err := h.devInterface.ControlTransfer(
		0x80,
		USB_REQ_GET_DESCRIPTOR,
		(USB_DT_BOS << 8),
		0,
		buf,
		5000,
	)
	if err != nil {
		return nil, nil, err
	}

	bos := &BOSDescriptor{
		Length:         buf[0],
		DescriptorType: buf[1],
		TotalLength:    binary.LittleEndian.Uint16(buf[2:4]),
		NumDeviceCaps:  buf[4],
	}

	// Get full BOS descriptor with capabilities
	fullBuf := make([]byte, bos.TotalLength)
	_, err = h.devInterface.ControlTransfer(
		0x80,
		USB_REQ_GET_DESCRIPTOR,
		(USB_DT_BOS << 8),
		0,
		fullBuf,
		5000,
	)
	if err != nil {
		return nil, nil, err
	}

	// Parse device capabilities
	var caps []DeviceCapabilityDescriptor
	offset := 5
	for i := 0; i < int(bos.NumDeviceCaps) && offset < len(fullBuf); i++ {
		if offset+2 >= len(fullBuf) {
			break
		}

		length := int(fullBuf[offset])
		if offset+length > len(fullBuf) {
			break
		}

		cap := DeviceCapabilityDescriptor{
			Length:            fullBuf[offset],
			DescriptorType:    fullBuf[offset+1],
			DevCapabilityType: fullBuf[offset+2],
		}
		caps = append(caps, cap)

		offset += length
	}

	return bos, caps, nil
}

// GetDeviceQualifierDescriptor retrieves the device qualifier descriptor
func (h *DeviceHandle) GetDeviceQualifierDescriptor() (*DeviceQualifierDescriptor, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, fmt.Errorf("device is closed")
	}

	buf := make([]byte, 10)
	_, err := h.devInterface.ControlTransfer(
		0x80,
		USB_REQ_GET_DESCRIPTOR,
		(USB_DT_DEVICE_QUALIFIER << 8),
		0,
		buf,
		5000,
	)
	if err != nil {
		return nil, err
	}

	return &DeviceQualifierDescriptor{
		Length:            buf[0],
		DescriptorType:    buf[1],
		USBVersion:        binary.LittleEndian.Uint16(buf[2:4]),
		DeviceClass:       buf[4],
		DeviceSubClass:    buf[5],
		DeviceProtocol:    buf[6],
		MaxPacketSize0:    buf[7],
		NumConfigurations: buf[8],
		Reserved:          buf[9],
	}, nil
}

// GetCapabilities returns device capabilities (not directly available on macOS)
func (h *DeviceHandle) GetCapabilities() (uint32, error) {
	// macOS doesn't expose capabilities in the same way as Linux
	// Return a default set of capabilities
	return 0, nil
}

// GetSpeed returns the device speed (not directly available on macOS)
func (h *DeviceHandle) GetSpeed() (Speed, error) {
	// macOS doesn't expose speed in the same simple way
	// Would need to query device properties
	return SpeedUnknown, nil
}
