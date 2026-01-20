package usb

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DeviceListOption is a functional option for configuring DeviceList behavior.
type DeviceListOption func(*deviceListOptions)

// deviceListOptions holds the configuration for DeviceList.
type deviceListOptions struct {
	includeInaccessible bool
}

// WithInaccessibleDevices returns an option that includes devices that cannot
// be opened (e.g., devices without WinUSB drivers). These devices will have
// limited information available.
func WithInaccessibleDevices() DeviceListOption {
	return func(o *deviceListOptions) {
		o.includeInaccessible = true
	}
}

// DeviceList returns a list of USB devices on the system.
// This uses SetupAPI enumeration on Windows.
//
// By default, only devices with WinUSB-compatible drivers that can be fully
// accessed are returned. Use WithInaccessibleDevices() to include devices
// that cannot be opened (they will have limited information).
func DeviceList(opts ...DeviceListOption) ([]*Device, error) {
	// Apply options
	options := &deviceListOptions{}
	for _, opt := range opts {
		opt(options)
	}

	winDevices, err := EnumerateUSBDevices()
	if err != nil {
		return nil, err
	}

	var devices []*Device
	for _, wd := range winDevices {
		device, err := createDeviceFromPath(wd.DevicePath)
		if err != nil {
			if options.includeInaccessible {
				// Create a minimal device with just the path and parsed VID/PID
				vid, pid := parseVidPidFromPath(wd.DevicePath)
				device = &Device{
					Path:       wd.DevicePath,
					devicePath: wd.DevicePath,
					Descriptor: DeviceDescriptor{
						VendorID:  vid,
						ProductID: pid,
					},
				}
			} else {
				// Skip devices we can't open
				continue
			}
		}
		devices = append(devices, device)
	}

	return devices, nil
}

// createDeviceFromPath creates a Device from a Windows device path
func createDeviceFromPath(devicePath string) (*Device, error) {
	// Open the device temporarily to read descriptors
	pathPtr, err := windows.UTF16PtrFromString(devicePath)
	if err != nil {
		return nil, err
	}

	fileHandle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OVERLAPPED,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open device: %w", err)
	}
	defer windows.CloseHandle(fileHandle)

	// Initialize WinUSB
	var winusbHandle winusbInterfaceHandle
	r0, _, e1 := syscall.SyscallN(
		procWinUsb_Initialize.Addr(),
		uintptr(fileHandle),
		uintptr(unsafe.Pointer(&winusbHandle)),
	)
	if r0 == 0 {
		return nil, fmt.Errorf("WinUsb_Initialize failed: %w", e1)
	}
	defer syscall.SyscallN(procWinUsb_Free.Addr(), uintptr(winusbHandle))

	// Read device descriptor
	descBuf := make([]byte, 18)
	var transferred uint32

	r0, _, e1 = syscall.SyscallN(
		procWinUsb_GetDescriptor.Addr(),
		uintptr(winusbHandle),
		uintptr(USB_DT_DEVICE),
		uintptr(0),
		uintptr(0),
		uintptr(unsafe.Pointer(&descBuf[0])),
		uintptr(len(descBuf)),
		uintptr(unsafe.Pointer(&transferred)),
	)
	if r0 == 0 {
		return nil, fmt.Errorf("failed to get device descriptor: %w", e1)
	}

	// Parse VID/PID from device path as fallback
	vid, pid := parseVidPidFromPath(devicePath)

	device := &Device{
		Path:       devicePath,
		devicePath: devicePath,
		Descriptor: DeviceDescriptor{
			Length:            descBuf[0],
			DescriptorType:    descBuf[1],
			USBVersion:        binary.LittleEndian.Uint16(descBuf[2:4]),
			DeviceClass:       descBuf[4],
			DeviceSubClass:    descBuf[5],
			DeviceProtocol:    descBuf[6],
			MaxPacketSize0:    descBuf[7],
			VendorID:          binary.LittleEndian.Uint16(descBuf[8:10]),
			ProductID:         binary.LittleEndian.Uint16(descBuf[10:12]),
			DeviceVersion:     binary.LittleEndian.Uint16(descBuf[12:14]),
			ManufacturerIndex: descBuf[14],
			ProductIndex:      descBuf[15],
			SerialNumberIndex: descBuf[16],
			NumConfigurations: descBuf[17],
		},
	}

	// Use parsed VID/PID if descriptor read failed
	if device.Descriptor.VendorID == 0 && vid != 0 {
		device.Descriptor.VendorID = vid
		device.Descriptor.ProductID = pid
	}

	// Try to read string descriptors for SysfsStrings
	device.SysfsStrings = &SysfsStrings{}
	if device.Descriptor.ManufacturerIndex > 0 {
		if str, err := readStringDescriptor(winusbHandle, device.Descriptor.ManufacturerIndex); err == nil {
			device.SysfsStrings.Manufacturer = str
		}
	}
	if device.Descriptor.ProductIndex > 0 {
		if str, err := readStringDescriptor(winusbHandle, device.Descriptor.ProductIndex); err == nil {
			device.SysfsStrings.Product = str
		}
	}
	if device.Descriptor.SerialNumberIndex > 0 {
		if str, err := readStringDescriptor(winusbHandle, device.Descriptor.SerialNumberIndex); err == nil {
			device.SysfsStrings.Serial = str
		}
	}

	return device, nil
}

// readStringDescriptor reads a string descriptor
func readStringDescriptor(winusbHandle winusbInterfaceHandle, index uint8) (string, error) {
	buf := make([]byte, 256)
	var transferred uint32

	r0, _, e1 := syscall.SyscallN(
		procWinUsb_GetDescriptor.Addr(),
		uintptr(winusbHandle),
		uintptr(USB_DT_STRING),
		uintptr(index),
		uintptr(0x0409), // English (US)
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&transferred)),
	)
	if r0 == 0 {
		return "", e1
	}

	if transferred < 2 {
		return "", fmt.Errorf("invalid string descriptor")
	}

	length := int(buf[0])
	if length > int(transferred) {
		length = int(transferred)
	}

	result := make([]uint16, 0, (length-2)/2)
	for i := 2; i < length; i += 2 {
		if i+1 < length {
			result = append(result, binary.LittleEndian.Uint16(buf[i:i+2]))
		}
	}

	return string(utf16ToRunes(result)), nil
}

// parseVidPidFromPath extracts VID and PID from Windows device path
func parseVidPidFromPath(path string) (vid, pid uint16) {
	// Windows device paths look like:
	// \\?\usb#vid_1234&pid_5678#...
	pathLower := strings.ToLower(path)

	vidIdx := strings.Index(pathLower, "vid_")
	if vidIdx >= 0 && vidIdx+8 <= len(pathLower) {
		if v, err := parseHex4(pathLower[vidIdx+4 : vidIdx+8]); err == nil {
			vid = v
		}
	}

	pidIdx := strings.Index(pathLower, "pid_")
	if pidIdx >= 0 && pidIdx+8 <= len(pathLower) {
		if p, err := parseHex4(pathLower[pidIdx+4 : pidIdx+8]); err == nil {
			pid = p
		}
	}

	return
}

func parseHex4(s string) (uint16, error) {
	var result uint16
	for _, c := range s {
		result <<= 4
		switch {
		case c >= '0' && c <= '9':
			result |= uint16(c - '0')
		case c >= 'a' && c <= 'f':
			result |= uint16(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			result |= uint16(c - 'A' + 10)
		default:
			return 0, fmt.Errorf("invalid hex character: %c", c)
		}
	}
	return result, nil
}

// OpenDevice opens a USB device by vendor ID and product ID.
// Returns the first matching device found.
func OpenDevice(vid, pid uint16) (*DeviceHandle, error) {
	devices, err := DeviceList()
	if err != nil {
		return nil, err
	}

	for _, dev := range devices {
		if dev.Descriptor.VendorID == vid && dev.Descriptor.ProductID == pid {
			return dev.Open()
		}
	}
	return nil, ErrDeviceNotFound
}

// devicePathRegex matches Windows USB device paths
var devicePathRegex = regexp.MustCompile(`(?i)\\\\[?]\\usb#vid_[0-9a-f]{4}&pid_[0-9a-f]{4}`)

// IsValidDevicePath checks if the given path is a valid USB device path.
func IsValidDevicePath(path string) bool {
	return devicePathRegex.MatchString(path)
}

// GetConfiguration gets the current device configuration
func (h *DeviceHandle) GetConfiguration() (int, error) {
	return h.Configuration()
}

// GetConfigDescriptor gets a configuration descriptor by index
func (h *DeviceHandle) GetConfigDescriptor(index uint8) (*ConfigDescriptor, error) {
	return h.ConfigDescriptorByValue(index + 1)
}

// GetActiveConfigDescriptor gets the descriptor for the active configuration
func (h *DeviceHandle) GetActiveConfigDescriptor() (*ConfigDescriptor, error) {
	config, err := h.GetConfiguration()
	if err != nil {
		return nil, err
	}

	if config > 0 {
		return h.ConfigDescriptorByValue(uint8(config))
	}

	return h.ConfigDescriptorByValue(1)
}

// GetDeviceDescriptor returns the device descriptor
func (h *DeviceHandle) GetDeviceDescriptor() (*DeviceDescriptor, error) {
	desc := h.Descriptor()
	return &desc, nil
}

// SetAltSetting sets the alternate setting for an interface
func (h *DeviceHandle) SetAltSetting(iface, altSetting uint8) error {
	return h.SetInterfaceAltSetting(iface, altSetting)
}

// KernelDriverActive checks if a kernel driver is active
func (h *DeviceHandle) KernelDriverActive(iface uint8) (bool, error) {
	// On Windows with WinUSB, the WinUSB driver is always active
	return false, nil
}

// GetBOSDescriptor gets the BOS descriptor
func (h *DeviceHandle) GetBOSDescriptor() (*BOSDescriptor, []DeviceCapabilityDescriptor, error) {
	return h.ReadBOSDescriptor()
}

// ReadBOSDescriptor reads the Binary Object Store descriptor
func (h *DeviceHandle) ReadBOSDescriptor() (*BOSDescriptor, []DeviceCapabilityDescriptor, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, nil, ErrDeviceNotFound
	}

	// First get header
	buf := make([]byte, 5)
	var transferred uint32

	r0, _, e1 := syscall.SyscallN(
		procWinUsb_GetDescriptor.Addr(),
		uintptr(h.winusbHandle),
		uintptr(USB_DT_BOS),
		uintptr(0),
		uintptr(0),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&transferred)),
	)
	if r0 == 0 {
		return nil, nil, fmt.Errorf("failed to get BOS descriptor: %w", e1)
	}

	if transferred < 5 || buf[1] != USB_DT_BOS {
		return nil, nil, fmt.Errorf("invalid BOS descriptor")
	}

	bos := &BOSDescriptor{
		Length:         buf[0],
		DescriptorType: buf[1],
		TotalLength:    binary.LittleEndian.Uint16(buf[2:4]),
		NumDeviceCaps:  buf[4],
	}

	// Get full descriptor
	fullBuf := make([]byte, bos.TotalLength)
	r0, _, e1 = syscall.SyscallN(
		procWinUsb_GetDescriptor.Addr(),
		uintptr(h.winusbHandle),
		uintptr(USB_DT_BOS),
		uintptr(0),
		uintptr(0),
		uintptr(unsafe.Pointer(&fullBuf[0])),
		uintptr(len(fullBuf)),
		uintptr(unsafe.Pointer(&transferred)),
	)
	if r0 == 0 {
		return nil, nil, fmt.Errorf("failed to get full BOS descriptor: %w", e1)
	}

	// Parse device capabilities
	caps := make([]DeviceCapabilityDescriptor, 0, bos.NumDeviceCaps)
	pos := 5

	for i := 0; i < int(bos.NumDeviceCaps) && pos < len(fullBuf); i++ {
		if pos+3 > len(fullBuf) {
			break
		}

		length := int(fullBuf[pos])
		if length < 3 || pos+length > len(fullBuf) {
			break
		}

		cap := DeviceCapabilityDescriptor{
			Length:            fullBuf[pos],
			DescriptorType:    fullBuf[pos+1],
			DevCapabilityType: fullBuf[pos+2],
		}

		caps = append(caps, cap)
		pos += length
	}

	return bos, caps, nil
}

// GetDeviceQualifierDescriptor gets the device qualifier descriptor
func (h *DeviceHandle) GetDeviceQualifierDescriptor() (*DeviceQualifierDescriptor, error) {
	return h.ReadDeviceQualifierDescriptor()
}

// ReadDeviceQualifierDescriptor reads device qualifier descriptor
func (h *DeviceHandle) ReadDeviceQualifierDescriptor() (*DeviceQualifierDescriptor, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, ErrDeviceNotFound
	}

	buf := make([]byte, 10)
	var transferred uint32

	r0, _, e1 := syscall.SyscallN(
		procWinUsb_GetDescriptor.Addr(),
		uintptr(h.winusbHandle),
		uintptr(USB_DT_DEVICE_QUALIFIER),
		uintptr(0),
		uintptr(0),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&transferred)),
	)
	if r0 == 0 {
		return nil, fmt.Errorf("failed to get device qualifier descriptor: %w", e1)
	}

	if transferred < 10 {
		return nil, fmt.Errorf("invalid device qualifier descriptor")
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

// GetCapabilities returns device capabilities (not directly available on Windows)
func (h *DeviceHandle) GetCapabilities() (uint32, error) {
	// Windows doesn't have direct equivalent to Linux usbfs capabilities
	return 0, ErrNotSupported
}

// GetSpeed returns the device speed
func (h *DeviceHandle) GetSpeed() (Speed, error) {
	speed, err := h.Speed()
	if err != nil {
		return SpeedUnknown, err
	}

	// Convert WinUSB speed to library Speed type
	switch speed {
	case LowSpeed:
		return SpeedLow, nil
	case FullSpeed:
		return SpeedFull, nil
	case HighSpeed:
		return SpeedHigh, nil
	case SuperSpeed:
		return SpeedSuper, nil
	default:
		return SpeedUnknown, nil
	}
}

// GetStatus gets device/interface/endpoint status
func (h *DeviceHandle) GetStatus(recipient, index uint16) (uint16, error) {
	buf := make([]byte, 2)
	requestType := uint8(0x80 | (recipient & 0x1F))

	_, err := h.ControlTransfer(requestType, USB_REQ_GET_STATUS, 0, index, buf, 5000*1000000)
	if err != nil {
		return 0, err
	}

	return binary.LittleEndian.Uint16(buf), nil
}
