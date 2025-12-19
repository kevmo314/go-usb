package usb

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <IOKit/usb/IOUSBLib.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdio.h>

// Use the correct constant based on availability
#ifndef kIOMainPortDefault
  #ifdef kIOMasterPortDefault
    #define kIOMainPortDefault kIOMasterPortDefault
  #else
    #define kIOMainPortDefault 0
  #endif
#endif

// Silence deprecation warning
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"

// Debug function to list all USB-related services
void DebugListUSBServices() {
    printf("Listing USB services in IORegistry:\n");
    
    // Try different service classes
    const char* classes[] = {
        "IOUSBHostDevice",
        "IOUSBDevice", 
        "IOUSBHostInterface",
        "IOUSBInterface",
        "AppleUSBHostController",
        "AppleUSBXHCI",
        "AppleUSBEHCI",
        "AppleUSBOHCI",
        "AppleUSBUHCI",
        NULL
    };
    
    for (int i = 0; classes[i] != NULL; i++) {
        CFMutableDictionaryRef matchingDict = IOServiceMatching(classes[i]);
        if (!matchingDict) continue;
        
        io_iterator_t iterator = 0;
        kern_return_t kr = IOServiceGetMatchingServices(kIOMainPortDefault, matchingDict, &iterator);
        
        if (kr == KERN_SUCCESS && iterator != 0) {
            io_object_t service;
            int count = 0;
            while ((service = IOIteratorNext(iterator)) != 0) {
                count++;
                IOObjectRelease(service);
            }
            if (count > 0) {
                printf("  Found %d instances of %s\n", count, classes[i]);
            }
            IOObjectRelease(iterator);
        }
    }
}

// Alternative iterator using IORegistry walking
io_iterator_t CreateUSBIteratorAlternative() {
    io_iterator_t iterator = 0;
    kern_return_t kr;
    
    // Method 1: Try IOUSBHostDevice (modern macOS)
    CFMutableDictionaryRef matchingDict = IOServiceMatching("IOUSBHostDevice");
    if (matchingDict) {
        kr = IOServiceGetMatchingServices(kIOMainPortDefault, matchingDict, &iterator);
        if (kr == KERN_SUCCESS && iterator != 0) {
            io_object_t test = IOIteratorNext(iterator);
            if (test != 0) {
                IOObjectRelease(test);
                IOIteratorReset(iterator);
                return iterator;
            }
            IOObjectRelease(iterator);
            iterator = 0;
        }
    }
    
    // Method 2: Try IOUSBDevice (legacy)
    matchingDict = IOServiceMatching("IOUSBDevice");
    if (matchingDict) {
        kr = IOServiceGetMatchingServices(kIOMainPortDefault, matchingDict, &iterator);
        if (kr == KERN_SUCCESS && iterator != 0) {
            io_object_t test = IOIteratorNext(iterator);
            if (test != 0) {
                IOObjectRelease(test);
                IOIteratorReset(iterator);
                return iterator;
            }
            IOObjectRelease(iterator);
            iterator = 0;
        }
    }
    
    // Method 3: Walk the IORegistry tree from USB plane root
    io_registry_entry_t usbPlane = IORegistryGetRootEntry(kIOMainPortDefault);
    if (usbPlane != 0) {
        kr = IORegistryEntryCreateIterator(usbPlane,
                                          kIOUSBPlane,
                                          kIORegistryIterateRecursively,
                                          &iterator);
        IOObjectRelease(usbPlane);
        if (kr == KERN_SUCCESS && iterator != 0) {
            return iterator;
        }
    }
    
    return 0;
}

// Check if a service is a USB device
int IsUSBDevice(io_service_t service) {
    // Check if it conforms to USB device class
    if (IOObjectConformsTo(service, "IOUSBHostDevice")) return 1;
    if (IOObjectConformsTo(service, "IOUSBDevice")) return 1;
    if (IOObjectConformsTo(service, kIOUSBDeviceClassName)) return 1;
    return 0;
}

// Helper to get property as integer (duplicate for this file)
int GetIntPropertyDebug(io_service_t service, const char* key) {
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

#pragma clang diagnostic pop

*/
import "C"

import "fmt"

// DebugUSBServices prints debug info about USB services
func DebugUSBServices() {
	C.DebugListUSBServices()
}

// TryAlternativeEnumeration attempts alternative enumeration
func TryAlternativeEnumeration() ([]*Device, error) {
	fmt.Println("Trying alternative USB enumeration method...")
	
	iterator := C.CreateUSBIteratorAlternative()
	if iterator == 0 {
		return nil, fmt.Errorf("alternative enumeration failed to create iterator")
	}
	defer C.IOObjectRelease(iterator)
	
	var devices []*Device
	
	for {
		service := C.IOIteratorNext(iterator)
		if service == 0 {
			break
		}
		defer C.IOObjectRelease(service)
		
		// Check if this is actually a USB device
		if C.IsUSBDevice(service) == 0 {
			continue
		}
		
		// Get basic properties
		vendorID := C.GetIntPropertyDebug(service, C.CString("idVendor"))
		productID := C.GetIntPropertyDebug(service, C.CString("idProduct"))
		locationID := C.GetIntPropertyDebug(service, C.CString("locationID"))
		
		if vendorID >= 0 && productID >= 0 {
			fmt.Printf("Found device: VID=%04x PID=%04x Location=%08x\n", 
				vendorID, productID, locationID)
			
			// Create a basic device entry
			dev := &Device{
				Path:    fmt.Sprintf("iokit:%08x", locationID),
				Bus:     uint8((locationID >> 24) & 0xFF),
				Address: uint8(len(devices) + 1),
				Descriptor: DeviceDescriptor{
					VendorID:  uint16(vendorID),
					ProductID: uint16(productID),
				},
			}
			devices = append(devices, dev)
		}
	}
	
	return devices, nil
}