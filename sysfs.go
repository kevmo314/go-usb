package usb

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SysfsDevice represents a USB device as seen in sysfs
type SysfsDevice struct {
	Path         string
	Name         string
	BusNum       uint8
	DevNum       uint8
	VID          uint16
	PID          uint16
	USB          uint16
	Device       uint16
	Class        uint8
	SubClass     uint8
	Protocol     uint8
	MaxPacket    uint8
	NumConfigs   uint8
	Manufacturer string
	Product      string
	Serial       string
}

// SysfsEnumerator handles USB device enumeration via sysfs
type SysfsEnumerator struct{}

// NewSysfsEnumerator creates a new sysfs enumerator
func NewSysfsEnumerator() *SysfsEnumerator {
	return &SysfsEnumerator{}
}

// EnumerateDevices returns all USB devices found in sysfs
func (e *SysfsEnumerator) EnumerateDevices() ([]*SysfsDevice, error) {
	sysfsDir := "/sys/bus/usb/devices"
	entries, err := os.ReadDir(sysfsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sysfs USB directory: %w", err)
	}

	var devices []*SysfsDevice

	for _, entry := range entries {
		name := entry.Name()

		// Skip interfaces (contain :)
		if strings.Contains(name, ":") {
			continue
		}

		// Include device entries (contain dash) and root hubs (usb1, usb2, etc.)
		if !strings.Contains(name, "-") && !strings.HasPrefix(name, "usb") {
			continue
		}

		sysfsPath := filepath.Join(sysfsDir, name)
		device, err := e.loadDeviceFromSysfs(sysfsPath, name)
		if err == nil {
			devices = append(devices, device)
		}
	}

	return devices, nil
}

// loadDeviceFromSysfs loads a single device from sysfs
func (e *SysfsEnumerator) loadDeviceFromSysfs(sysfsPath, name string) (*SysfsDevice, error) {
	device := &SysfsDevice{
		Path: sysfsPath,
		Name: name,
	}

	// Helper to read numeric values
	readUint8 := func(filename string) (uint8, error) {
		data, err := os.ReadFile(filepath.Join(sysfsPath, filename))
		if err != nil {
			return 0, err
		}
		val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 8)
		return uint8(val), err
	}

	readUint16Hex := func(filename string) (uint16, error) {
		data, err := os.ReadFile(filepath.Join(sysfsPath, filename))
		if err != nil {
			return 0, err
		}
		val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 16, 16)
		return uint16(val), err
	}

	readString := func(filename string) string {
		data, err := os.ReadFile(filepath.Join(sysfsPath, filename))
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}

	// Read required fields
	var err error
	if device.BusNum, err = readUint8("busnum"); err != nil {
		return nil, err
	}

	if device.DevNum, err = readUint8("devnum"); err != nil {
		return nil, err
	}

	if device.VID, err = readUint16Hex("idVendor"); err != nil {
		return nil, err
	}

	if device.PID, err = readUint16Hex("idProduct"); err != nil {
		return nil, err
	}

	// Read optional fields
	device.Device, _ = readUint16Hex("bcdDevice")

	// USB version is in the 'version' field, format like " 2.01"
	if versionData, err := os.ReadFile(filepath.Join(sysfsPath, "version")); err == nil {
		versionStr := strings.TrimSpace(string(versionData))
		if len(versionStr) > 0 {
			// Parse version like "2.01" to 0x0201
			var major, minor int
			if n, _ := fmt.Sscanf(versionStr, "%d.%02d", &major, &minor); n == 2 {
				device.USB = uint16(major)<<8 | uint16(minor)
			}
		}
	}
	device.Class, _ = readUint8("bDeviceClass")
	device.SubClass, _ = readUint8("bDeviceSubClass")
	device.Protocol, _ = readUint8("bDeviceProtocol")
	device.MaxPacket, _ = readUint8("bMaxPacketSize0")
	device.NumConfigs, _ = readUint8("bNumConfigurations")

	// Read string descriptors if available
	device.Manufacturer = readString("manufacturer")
	device.Product = readString("product")
	device.Serial = readString("serial")

	return device, nil
}

// ToUSBDevice converts a SysfsDevice to a USB Device
func (s *SysfsDevice) ToUSBDevice() *Device {
	device := &Device{
		Path:    fmt.Sprintf("/dev/bus/usb/%03d/%03d", s.BusNum, s.DevNum),
		Bus:     s.BusNum,
		Address: s.DevNum,
		sysfsStrings: &SysfsStrings{
			Manufacturer: s.Manufacturer,
			Product:      s.Product,
			Serial:       s.Serial,
		},
		Descriptor: DeviceDescriptor{
			Length:            18,
			DescriptorType:    1,
			USBVersion:        s.USB,
			DeviceClass:       s.Class,
			DeviceSubClass:    s.SubClass,
			DeviceProtocol:    s.Protocol,
			MaxPacketSize0:    s.MaxPacket,
			VendorID:          s.VID,
			ProductID:         s.PID,
			DeviceVersion:     s.Device,
			ManufacturerIndex: 1, // These indices are not easily available from sysfs
			ProductIndex:      2,
			SerialNumberIndex: 3,
			NumConfigurations: s.NumConfigs,
		},
	}

	return device
}
