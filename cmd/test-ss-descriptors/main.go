package main

import (
	"fmt"
	"log"
	"os"

	usb "github.com/kevmo314/go-usb"
)

func main() {
	if os.Getuid() != 0 {
		log.Fatal("This program requires root privileges to access USB devices")
	}

	fmt.Println("SuperSpeed Descriptor Test")
	fmt.Println("==========================")

	devices, err := usb.GetDeviceList()
	if err != nil {
		log.Fatalf("Failed to get device list: %v", err)
	}

	fmt.Printf("\nFound %d USB devices\n", len(devices))

	for _, dev := range devices {
		// Skip root hubs
		if dev.Descriptor.DeviceClass == 9 {
			continue
		}

		fmt.Printf("\nDevice: %04x:%04x\n", dev.Descriptor.VendorID, dev.Descriptor.ProductID)

		handle, err := dev.Open()
		if err != nil {
			fmt.Printf("  Could not open: %v\n", err)
			continue
		}
		defer handle.Close()

		// Test device speed
		if speed, err := handle.GetSpeed(); err == nil {
			speedNames := map[uint8]string{
				1: "Low Speed",
				2: "Full Speed",
				3: "High Speed",
				4: "Wireless",
				5: "Super Speed",
				6: "Super Speed+",
			}
			fmt.Printf("  Speed: %s\n", speedNames[speed])

			// Only test SS descriptors for SuperSpeed devices
			if speed >= 5 {
				testSSDescriptors(handle)
			}
		}

		// Test BOS descriptor (available on USB 2.1+ and USB 3.0+)
		if bos, caps, err := handle.ReadBOSDescriptor(); err == nil {
			fmt.Printf("  BOS Descriptor: %d capabilities\n", bos.NumDeviceCaps)
			for i, cap := range caps {
				capNames := map[uint8]string{
					0x01: "Wireless USB",
					0x02: "USB 2.0 Extension",
					0x03: "SuperSpeed USB",
					0x04: "Container ID",
					0x05: "Platform",
					0x06: "Power Delivery",
					0x07: "Battery Info",
					0x08: "PD Consumer Port",
					0x09: "PD Provider Port",
					0x0A: "SuperSpeedPlus USB",
					0x0B: "Precision Time Measurement",
					0x0C: "Wireless USB Ext",
				}
				name := capNames[cap.DevCapabilityType]
				if name == "" {
					name = fmt.Sprintf("Unknown (0x%02x)", cap.DevCapabilityType)
				}
				fmt.Printf("    [%d] %s\n", i, name)
			}

			// Test specific capability getters
			if usb20ext, err := handle.GetUSB20ExtensionDescriptor(); err == nil {
				fmt.Printf("  USB 2.0 Extension found: Attributes=0x%08x\n", usb20ext.Attributes)
				if usb20ext.Attributes&0x02 != 0 {
					fmt.Println("    - Link Power Management (LPM) supported")
				}
			}

			if ssUsb, err := handle.GetSSUSBDeviceCapabilityDescriptor(); err == nil {
				fmt.Printf("  SuperSpeed USB Capability found:\n")
				fmt.Printf("    - Speeds: 0x%04x\n", ssUsb.SpeedsSupported)
				fmt.Printf("    - U1 Exit Latency: %d µs\n", ssUsb.U1DevExitLat)
				fmt.Printf("    - U2 Exit Latency: %d µs\n", ssUsb.U2DevExitLat)
			}
		}

		// Test Device Qualifier (USB 2.0+ devices)
		if qual, err := handle.ReadDeviceQualifierDescriptor(); err == nil {
			fmt.Printf("  Device Qualifier: USB %d.%d\n",
				qual.USBVersion>>8, (qual.USBVersion&0xFF)>>4)
		}
	}

	fmt.Println("\nTest completed!")
}

func testSSDescriptors(handle *usb.DeviceHandle) {
	fmt.Println("  Testing SuperSpeed descriptors...")

	// Get first configuration
	config, err := handle.GetConfigDescriptorByValue(0)
	if err != nil {
		fmt.Printf("    Error getting config: %v\n", err)
		return
	}

	// Look for SS endpoint companions
	foundCompanion := false
	for _, iface := range config.Interfaces {
		for _, altSetting := range iface.AltSettings {
			for _, endpoint := range altSetting.Endpoints {
				// Try to get SS companion descriptor
				companion, err := handle.GetSSEndpointCompanionDescriptor(
					0, altSetting.InterfaceNumber,
					altSetting.AlternateSetting,
					endpoint.EndpointAddr,
				)
				if err == nil && companion != nil {
					if !foundCompanion {
						fmt.Println("    SS Endpoint Companions found:")
						foundCompanion = true
					}
					fmt.Printf("      EP %02x: MaxBurst=%d, BytesPerInterval=%d\n",
						endpoint.EndpointAddr,
						companion.MaxBurst,
						companion.BytesPerInterval)
				}
			}
		}
	}

	if !foundCompanion {
		fmt.Println("    No SS Endpoint Companions found")
	}
}
