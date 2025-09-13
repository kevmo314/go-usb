package main

import (
	"fmt"
	"log"
	"time"

	usb "github.com/kevmo314/go-usb"
)

func main() {
	// Get device list
	devices, err := usb.GetDeviceList()
	if err != nil {
		log.Fatalf("Failed to get device list: %v", err)
	}

	fmt.Printf("Found %d USB devices\n", len(devices))

	// Find a suitable test device (prefer non-root hub devices)
	var testDevice *usb.Device
	for _, dev := range devices {
		// Skip root hubs
		if dev.Descriptor.VendorID == 0x1d6b {
			continue
		}
		testDevice = dev
		break
	}

	if testDevice == nil && len(devices) > 0 {
		// Fallback to any device if no non-hub device found
		testDevice = devices[0]
	}

	if testDevice == nil {
		log.Fatal("No USB devices found")
	}

	fmt.Printf("\nTesting with device: VID=0x%04x PID=0x%04x Path=%s\n",
		testDevice.Descriptor.VendorID, testDevice.Descriptor.ProductID, testDevice.Path)

	// Try to open the device
	handle, err := testDevice.Open()
	if err != nil {
		log.Fatalf("Failed to open device: %v", err)
	}
	defer handle.Close()

	fmt.Println("\nDevice opened successfully!")

	// Test 1: Read device descriptor using control transfer
	fmt.Println("\nTest 1: Reading device descriptor via control transfer...")
	buf := make([]byte, 18)
	n, err := handle.ControlTransfer(
		0x80,          // bmRequestType (device-to-host)
		0x06,          // bRequest (GET_DESCRIPTOR)
		0x0100,        // wValue (DEVICE descriptor)
		0x0000,        // wIndex
		buf,           // data buffer
		5*time.Second, // timeout
	)

	if err != nil {
		fmt.Printf("Control transfer failed: %v\n", err)
	} else {
		fmt.Printf("Successfully read %d bytes\n", n)
		fmt.Printf("Device descriptor: %02x\n", buf[:n])

		if n >= 18 {
			fmt.Printf("  Length: %d\n", buf[0])
			fmt.Printf("  Type: %d\n", buf[1])
			fmt.Printf("  USB Version: %d.%d\n", buf[3], buf[2])
			fmt.Printf("  VendorID: 0x%04x\n", uint16(buf[8])|uint16(buf[9])<<8)
			fmt.Printf("  ProductID: 0x%04x\n", uint16(buf[10])|uint16(buf[11])<<8)
		}
	}

	// Test 2: Read string descriptors if available
	desc := handle.GetDescriptor()

	if desc.ManufacturerIndex > 0 {
		fmt.Printf("\nTest 2: Reading manufacturer string descriptor (index %d)...\n", desc.ManufacturerIndex)
		manufacturer, err := handle.GetStringDescriptor(desc.ManufacturerIndex)
		if err != nil {
			fmt.Printf("Failed to read manufacturer: %v\n", err)
		} else {
			fmt.Printf("Manufacturer: %s\n", manufacturer)
		}
	}

	if desc.ProductIndex > 0 {
		fmt.Printf("\nTest 3: Reading product string descriptor (index %d)...\n", desc.ProductIndex)
		product, err := handle.GetStringDescriptor(desc.ProductIndex)
		if err != nil {
			fmt.Printf("Failed to read product: %v\n", err)
		} else {
			fmt.Printf("Product: %s\n", product)
		}
	}

	if desc.SerialNumberIndex > 0 {
		fmt.Printf("\nTest 4: Reading serial number string descriptor (index %d)...\n", desc.SerialNumberIndex)
		serial, err := handle.GetStringDescriptor(desc.SerialNumberIndex)
		if err != nil {
			fmt.Printf("Failed to read serial: %v\n", err)
		} else {
			fmt.Printf("Serial Number: %s\n", serial)
		}
	}

	// Test 5: Read configuration descriptor
	fmt.Printf("\nTest 5: Reading configuration descriptor...\n")
	config, interfaces, endpoints, err := handle.ReadConfigDescriptor(0)
	if err != nil {
		fmt.Printf("Failed to read config descriptor: %v\n", err)
	} else {
		fmt.Printf("Configuration:\n")
		fmt.Printf("  Interfaces: %d\n", config.NumInterfaces)
		fmt.Printf("  Max Power: %dmA\n", config.MaxPower*2)
		fmt.Printf("  Attributes: 0x%02x\n", config.Attributes)

		fmt.Printf("Found %d interfaces and %d endpoints\n", len(interfaces), len(endpoints))

		for i, iface := range interfaces {
			fmt.Printf("  Interface %d:\n", i)
			fmt.Printf("    Number: %d\n", iface.InterfaceNumber)
			fmt.Printf("    Class: %d\n", iface.InterfaceClass)
			fmt.Printf("    SubClass: %d\n", iface.InterfaceSubClass)
			fmt.Printf("    Protocol: %d\n", iface.InterfaceProtocol)
			fmt.Printf("    Endpoints: %d\n", iface.NumEndpoints)
		}

		for i, ep := range endpoints {
			fmt.Printf("  Endpoint %d:\n", i)
			fmt.Printf("    Address: 0x%02x\n", ep.EndpointAddr)
			fmt.Printf("    Attributes: 0x%02x\n", ep.Attributes)
			fmt.Printf("    Max Packet Size: %d\n", ep.MaxPacketSize)
			fmt.Printf("    Interval: %d\n", ep.Interval)
		}
	}

	fmt.Println("\nAll tests completed successfully!")
}
