package usb

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
)

type USBIDDatabase struct {
	vendors map[uint16]Vendor
	classes map[uint8]string
	mu      sync.RWMutex
	loaded  bool
}

type Vendor struct {
	Name     string
	Products map[uint16]string
}

var globalUSBIDs = &USBIDDatabase{
	vendors: make(map[uint16]Vendor),
	classes: make(map[uint8]string),
}

func init() {
	// Initialize with some basic entries
	globalUSBIDs.initBasicEntries()
}

func (db *USBIDDatabase) initBasicEntries() {
	db.vendors[0x1d6b] = Vendor{
		Name: "Linux Foundation",
		Products: map[uint16]string{
			0x0001: "1.1 root hub",
			0x0002: "2.0 root hub",
			0x0003: "3.0 root hub",
		},
	}

	db.vendors[0x174c] = Vendor{
		Name: "ASMedia Technology Inc.",
		Products: map[uint16]string{
			0x2074: "ASM1074 High-Speed hub",
			0x3074: "ASM1074 SuperSpeed hub",
		},
	}

	db.vendors[0x0db0] = Vendor{
		Name: "Micro Star International",
		Products: map[uint16]string{
			0x422d: "USB Audio",
		},
	}

	db.vendors[0x0e8d] = Vendor{
		Name: "MediaTek Inc.",
		Products: map[uint16]string{
			0x0616: "Wireless_Device",
		},
	}

	db.vendors[0x1462] = Vendor{
		Name: "Micro Star International",
		Products: map[uint16]string{
			0x7d75: "MYSTIC LIGHT ",
		},
	}

	db.vendors[0x05e3] = Vendor{
		Name: "Genesys Logic, Inc.",
		Products: map[uint16]string{
			0x0608: "Hub",
		},
	}

	db.vendors[0x046d] = Vendor{
		Name: "Logitech, Inc.",
		Products: map[uint16]string{
			0x08e5: "C920 PRO HD Webcam",
		},
	}

	db.vendors[0x2ca3] = Vendor{
		Name: "DJI Technology Co., Ltd.",
		Products: map[uint16]string{
			0x0023: "OsmoAction4",
		},
	}

	// Class codes
	db.classes[0x00] = "Use class information in the Interface Descriptors"
	db.classes[0x01] = "Audio"
	db.classes[0x02] = "Communications and CDC Control"
	db.classes[0x03] = "Human Interface Device"
	db.classes[0x05] = "Physical"
	db.classes[0x06] = "Image"
	db.classes[0x07] = "Printer"
	db.classes[0x08] = "Mass Storage"
	db.classes[0x09] = "Hub"
	db.classes[0x0a] = "CDC Data"
	db.classes[0x0b] = "Smart Card"
	db.classes[0x0d] = "Content Security"
	db.classes[0x0e] = "Video"
	db.classes[0x0f] = "Personal Healthcare"
	db.classes[0x10] = "Audio/Video Devices"
	db.classes[0xdc] = "Diagnostic"
	db.classes[0xe0] = "Wireless"
	db.classes[0xef] = "Miscellaneous Device"
	db.classes[0xfe] = "Application Specific"
	db.classes[0xff] = "Vendor Specific"
}

func (db *USBIDDatabase) LoadFromFile(path string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentVendor uint16
	var inVendor bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for class section
		if strings.HasPrefix(line, "C ") {
			inVendor = false
			continue
		}

		if !inVendor {
			// Parse vendor line
			if len(line) >= 4 && isHex(line[:4]) {
				vid, err := strconv.ParseUint(line[:4], 16, 16)
				if err != nil {
					continue
				}
				currentVendor = uint16(vid)
				vendorName := strings.TrimSpace(line[4:])

				vendor := db.vendors[currentVendor]
				vendor.Name = vendorName
				if vendor.Products == nil {
					vendor.Products = make(map[uint16]string)
				}
				db.vendors[currentVendor] = vendor
				inVendor = true
			}
		} else {
			// Parse product line (should start with tab)
			if strings.HasPrefix(line, "\t") && len(line) >= 5 {
				line = line[1:] // Remove tab
				if len(line) >= 4 && isHex(line[:4]) {
					pid, err := strconv.ParseUint(line[:4], 16, 16)
					if err != nil {
						continue
					}
					productName := strings.TrimSpace(line[4:])

					vendor := db.vendors[currentVendor]
					if vendor.Products == nil {
						vendor.Products = make(map[uint16]string)
					}
					vendor.Products[uint16(pid)] = productName
					db.vendors[currentVendor] = vendor
				}
			} else {
				inVendor = false
			}
		}
	}

	db.loaded = true
	return scanner.Err()
}

func (db *USBIDDatabase) VendorName(vid uint16) string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if vendor, ok := db.vendors[vid]; ok {
		return vendor.Name
	}
	return ""
}

func (db *USBIDDatabase) ProductName(vid, pid uint16) string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if vendor, ok := db.vendors[vid]; ok {
		if product, ok := vendor.Products[pid]; ok {
			return product
		}
	}
	return ""
}

func (db *USBIDDatabase) ClassName(class uint8) string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if name, ok := db.classes[class]; ok {
		return name
	}
	return ""
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// Global functions for convenience
func VendorName(vid uint16) string {
	// Try to load USB IDs database if not loaded
	if !globalUSBIDs.loaded {
		paths := []string{
			"/usr/share/hwdata/usb.ids",
			"/usr/share/usb.ids",
			"/var/lib/usbutils/usb.ids",
		}
		for _, path := range paths {
			if err := globalUSBIDs.LoadFromFile(path); err == nil {
				break
			}
		}
	}

	return globalUSBIDs.VendorName(vid)
}

func ProductName(vid, pid uint16) string {
	if !globalUSBIDs.loaded {
		paths := []string{
			"/usr/share/hwdata/usb.ids",
			"/usr/share/usb.ids",
			"/var/lib/usbutils/usb.ids",
		}
		for _, path := range paths {
			if err := globalUSBIDs.LoadFromFile(path); err == nil {
				break
			}
		}
	}

	return globalUSBIDs.ProductName(vid, pid)
}

func ClassName(class uint8) string {
	return globalUSBIDs.ClassName(class)
}

// Add method to Device to get string descriptors from sysfs
func (d *Device) ManufacturerFromSysfs() string {
	if d.sysfsStrings != nil {
		return d.sysfsStrings.Manufacturer
	}
	return ""
}

func (d *Device) ProductFromSysfs() string {
	if d.sysfsStrings != nil {
		return d.sysfsStrings.Product
	}
	return ""
}

func (d *Device) SerialFromSysfs() string {
	if d.sysfsStrings != nil {
		return d.sysfsStrings.Serial
	}
	return ""
}
