package usb

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <IOKit/usb/IOUSBLib.h>
#include <IOKit/IOCFPlugIn.h>
#include <CoreFoundation/CoreFoundation.h>

// Use the correct constant based on availability
#ifndef kIOMainPortDefault
  #ifdef kIOMasterPortDefault
    #define kIOMainPortDefault kIOMasterPortDefault
  #else
    #define kIOMainPortDefault 0
  #endif
#endif

// Silence deprecation warning for kIOMasterPortDefault
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

// Helper to get property as integer
int GetIntProperty(io_service_t service, const char* key) {
    CFStringRef keyRef = CFStringCreateWithCString(kCFAllocatorDefault, key, kCFStringEncodingUTF8);
    CFNumberRef valueRef = (CFNumberRef)IORegistryEntryCreateCFProperty(service, keyRef, kCFAllocatorDefault, 0);
    CFRelease(keyRef);
    
    if (valueRef == NULL) {
        return -1;
    }
    
    int value = 0;
    CFNumberGetValue(valueRef, kCFNumberIntType, &value);
    CFRelease(valueRef);
    return value;
}

// Helper to get property as string
char* GetStringProperty(io_service_t service, const char* key) {
    CFStringRef keyRef = CFStringCreateWithCString(kCFAllocatorDefault, key, kCFStringEncodingUTF8);
    CFStringRef valueRef = (CFStringRef)IORegistryEntryCreateCFProperty(service, keyRef, kCFAllocatorDefault, 0);
    CFRelease(keyRef);
    
    if (valueRef == NULL) {
        return NULL;
    }
    
    static char buffer[256];
    Boolean result = CFStringGetCString(valueRef, buffer, sizeof(buffer), kCFStringEncodingUTF8);
    CFRelease(valueRef);
    
    if (!result) {
        return NULL;
    }
    
    return buffer;
}

// Create iterator for USB devices
io_iterator_t CreateUSBIterator() {
    io_iterator_t iterator = 0;
    kern_return_t kr;
    
    // Try to create a broad matching dictionary for all USB devices
    // Use IOServiceMatching with a more general approach
    CFMutableDictionaryRef matchingDict = NULL;
    
    // First, try to get ALL services and filter manually
    // This is less efficient but more likely to find devices
    matchingDict = IOServiceMatching("IOUSBHostDevice");
    if (matchingDict == NULL) {
        matchingDict = IOServiceMatching("IOUSBDevice");
    }
    if (matchingDict == NULL) {
        matchingDict = IOServiceMatching(kIOUSBDeviceClassName);
    }
    
    if (matchingDict != NULL) {
        kr = IOServiceGetMatchingServices(kIOMainPortDefault, matchingDict, &iterator);
        if (kr == KERN_SUCCESS && iterator != 0) {
            // Check if we actually have any devices
            io_object_t testObj = IOIteratorNext(iterator);
            if (testObj != 0) {
                // Reset the iterator back to start
                IOObjectRelease(testObj);
                IOIteratorReset(iterator);
                return iterator;
            }
            IOObjectRelease(iterator);
        }
    }
    
    // If that failed, try a different approach - look for USB controllers
    matchingDict = IOServiceMatching("AppleUSBXHCI");
    if (matchingDict != NULL) {
        io_iterator_t controllerIter = 0;
        kr = IOServiceGetMatchingServices(kIOMainPortDefault, matchingDict, &controllerIter);
        if (kr == KERN_SUCCESS && controllerIter != 0) {
            // We found controllers, now look for their children
            io_object_t controller;
            while ((controller = IOIteratorNext(controllerIter)) != 0) {
                io_iterator_t childIter = 0;
                kr = IORegistryEntryGetChildIterator(controller, kIOServicePlane, &childIter);
                if (kr == KERN_SUCCESS && childIter != 0) {
                    IOObjectRelease(controller);
                    IOObjectRelease(controllerIter);
                    return childIter;
                }
                IOObjectRelease(controller);
            }
            IOObjectRelease(controllerIter);
        }
    }
    
    return 0;
}

// Get next USB device from iterator
io_service_t GetNextUSBDevice(io_iterator_t iterator) {
    return IOIteratorNext(iterator);
}

// Release iterator
void ReleaseIterator(io_iterator_t iterator) {
    IOObjectRelease(iterator);
}

// Release service
void ReleaseService(io_service_t service) {
    IOObjectRelease(service);
}

#pragma clang diagnostic pop

*/
import "C"

import (
	"fmt"
	"strconv"
	"strings"
)

// IOKitDevice represents a USB device discovered via IOKit
type IOKitDevice struct {
	Service    C.io_service_t
	LocationID uint32
	VendorID   uint16
	ProductID  uint16
	Bus        uint8
	Address    uint8
}

// IOKitEnumerator handles USB device enumeration via IOKit
type IOKitEnumerator struct{}

// NewIOKitEnumerator creates a new IOKit enumerator
func NewIOKitEnumerator() *IOKitEnumerator {
	return &IOKitEnumerator{}
}

// EnumerateDevices returns all USB devices found via IOKit
func (e *IOKitEnumerator) EnumerateDevices() ([]*Device, error) {
	iterator := C.CreateUSBIterator()
	if iterator == 0 {
		// No USB devices found or unable to create iterator
		// This is common on Apple Silicon Macs with no USB devices connected
		return []*Device{}, nil
	}
	defer C.ReleaseIterator(iterator)

	var devices []*Device
	
	// Check if iterator is valid but empty
	firstDevice := C.GetNextUSBDevice(iterator)
	if firstDevice == 0 {
		// No USB devices found - common on systems with no USB devices
		return devices, nil
	}
	
	// Process all devices
	// Use a helper function to handle defer properly in loop
	processDevice := func(device C.io_service_t) {
		defer C.ReleaseService(device)
		
		// Get device properties
		vendorID := C.GetIntProperty(device, C.CString("idVendor"))
		productID := C.GetIntProperty(device, C.CString("idProduct"))
		locationID := C.GetIntProperty(device, C.CString("locationID"))
		
		if vendorID < 0 || productID < 0 {
			return
		}
		
		// Get device interface to retrieve descriptor
		devInterface, err := GetUSBDeviceInterface(device)
		if err != nil {
			return
		}
		defer devInterface.Release()
		
		// Get device descriptor
		descriptor, err := devInterface.GetDeviceDescriptor()
		if err != nil {
			return
		}
		
		// Extract bus and address from location ID
		// Location ID format: 0xBBDDPPPP where BB = bus, DD = depth, PPPP = port
		bus := uint8((locationID >> 24) & 0xFF)
		address := uint8(len(devices) + 1) // Simple incrementing address
		
		// Get string properties if available
		manufacturer := C.GoString(C.GetStringProperty(device, C.CString("USB Vendor Name")))
		product := C.GoString(C.GetStringProperty(device, C.CString("USB Product Name")))
		serial := C.GoString(C.GetStringProperty(device, C.CString("USB Serial Number")))
		
		usbDev := &Device{
			Path:    fmt.Sprintf("iokit:%08x", locationID),
			Bus:     bus,
			Address: address,
			Descriptor: *descriptor,
			IOKitDevice: &IOKitDevice{
				Service:    0, // Don't store service as we're releasing it
				LocationID: uint32(locationID),
				VendorID:   uint16(vendorID),
				ProductID:  uint16(productID),
				Bus:        bus,
				Address:    address,
			},
			CachedStrings: &CachedStrings{
				Manufacturer: manufacturer,
				Product:      product,
				Serial:       serial,
			},
		}
		
		devices = append(devices, usbDev)
	}
	
	// Process first device
	processDevice(firstDevice)
	
	// Process remaining devices
	for usbDevice := C.GetNextUSBDevice(iterator); usbDevice != 0; usbDevice = C.GetNextUSBDevice(iterator) {
		processDevice(usbDevice)
	}
	
	return devices, nil
}

// Device represents a USB device on macOS
type Device struct {
	Path          string
	Bus           uint8
	Address       uint8
	Descriptor    DeviceDescriptor
	IOKitDevice   *IOKitDevice
	CachedStrings *CachedStrings
}

// CachedStrings holds cached string descriptors
type CachedStrings struct {
	Manufacturer string
	Product      string
	Serial       string
}

// DeviceList returns a list of USB devices on macOS
func DeviceList() ([]*Device, error) {
	enumerator := NewIOKitEnumerator()
	return enumerator.EnumerateDevices()
}

// Open opens the USB device for communication
func (d *Device) Open() (*DeviceHandle, error) {
	// Re-acquire the device service
	iterator := C.CreateUSBIterator()
	if iterator == 0 {
		return nil, fmt.Errorf("failed to create USB device iterator")
	}
	defer C.ReleaseIterator(iterator)
	
	var usbDevice C.io_service_t
	for {
		device := C.GetNextUSBDevice(iterator)
		if device == 0 {
			break
		}
		
		locationID := C.GetIntProperty(device, C.CString("locationID"))
		if uint32(locationID) == d.IOKitDevice.LocationID {
			usbDevice = device
			break
		}
		C.ReleaseService(device)
	}
	
	if usbDevice == 0 {
		return nil, ErrDeviceNotFound
	}
	
	// Get device interface
	devInterface, err := GetUSBDeviceInterface(usbDevice)
	if err != nil {
		C.ReleaseService(usbDevice)
		return nil, err
	}
	
	// Open the device
	err = devInterface.Open()
	if err != nil {
		devInterface.Release()
		C.ReleaseService(usbDevice)
		return nil, err
	}
	
	return &DeviceHandle{
		device:       d,
		devInterface: devInterface,
		service:      usbDevice,
		interfaces:   make(map[uint8]*IOUSBInterfaceInterface),
		claimedIfaces: make(map[uint8]bool),
	}, nil
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

// IsValidDevicePath checks if a path is a valid USB device path on macOS
func IsValidDevicePath(path string) bool {
	// macOS paths start with "iokit:"
	if !strings.HasPrefix(path, "iokit:") {
		return false
	}
	
	// Extract location ID from path
	locationStr := strings.TrimPrefix(path, "iokit:")
	_, err := strconv.ParseUint(locationStr, 16, 32)
	return err == nil
}

// SysfsDevice is not used on macOS but included for compatibility
type SysfsDevice struct{}

// SysfsStrings is not used on macOS but included for compatibility
type SysfsStrings struct{}

// ToUSBDevice is not implemented on macOS
func (s *SysfsDevice) ToUSBDevice() *Device {
	return nil
}

// SysfsEnumerator is not used on macOS but included for compatibility
type SysfsEnumerator struct{}

// NewSysfsEnumerator returns nil on macOS
func NewSysfsEnumerator() *SysfsEnumerator {
	return nil
}