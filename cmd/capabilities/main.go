package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/kevmo314/go-usb"
)

func main() {
	var (
		vid = flag.Int("vid", 0, "Vendor ID (0 for any)")
		pid = flag.Int("pid", 0, "Product ID (0 for any)")
	)
	flag.Parse()

	// Get list of USB devices
	devices, err := usb.GetDeviceList()
	if err != nil {
		log.Fatalf("Failed to get device list: %v", err)
	}

	if len(devices) == 0 {
		fmt.Println("No USB devices found")
		return
	}

	fmt.Printf("Found %d USB devices\n\n", len(devices))

	foundAny := false
	for _, dev := range devices {
		// Filter by VID/PID if specified
		if *vid != 0 && dev.Descriptor.VendorID != uint16(*vid) {
			continue
		}
		if *pid != 0 && dev.Descriptor.ProductID != uint16(*pid) {
			continue
		}

		// Try to open the device
		handle, err := dev.Open()
		if err != nil {
			continue
		}
		defer handle.Close()

		// Try to get device speed
		speed, err := handle.GetSpeed()
		if err == nil {
			speedStr := getSpeedString(speed)
			if speedStr == "Unknown" {
				continue // Skip devices with unknown speed
			}
		}

		fmt.Printf("Device: Bus %03d Device %03d VID:0x%04x PID:0x%04x\n",
			dev.Bus, dev.Address, dev.Descriptor.VendorID, dev.Descriptor.ProductID)

		// Get manufacturer and product strings
		if dev.Descriptor.ManufacturerIndex > 0 {
			if str, err := handle.GetStringDescriptor(dev.Descriptor.ManufacturerIndex); err == nil {
				fmt.Printf("  Manufacturer: %s\n", str)
			}
		}
		if dev.Descriptor.ProductIndex > 0 {
			if str, err := handle.GetStringDescriptor(dev.Descriptor.ProductIndex); err == nil {
				fmt.Printf("  Product: %s\n", str)
			}
		}

		// Display USB version
		usbVersion := dev.Descriptor.USBVersion
		fmt.Printf("  USB Version: %x.%02x\n", (usbVersion>>8)&0xff, usbVersion&0xff)

		// Display device speed
		if speed, err := handle.GetSpeed(); err == nil {
			fmt.Printf("  Speed: %s\n", getSpeedString(speed))
		}

		// Try to get BOS descriptor (USB 3.0+)
		bos, _, err := handle.ReadBOSDescriptor()
		if err == nil && bos != nil {
			foundAny = true
			fmt.Printf("  BOS Descriptor:\n")
			fmt.Printf("    Total Length: %d bytes\n", bos.TotalLength)
			fmt.Printf("    Device Capabilities: %d\n", bos.NumDeviceCaps)

			// Try to get USB 2.0 Extension descriptor
			usb2ext, err := handle.GetUSB20ExtensionDescriptor()
			if err == nil && usb2ext != nil {
				fmt.Printf("    USB 2.0 Extension:\n")
				fmt.Printf("      Attributes: 0x%08x\n", usb2ext.Attributes)
				if usb2ext.Attributes&0x02 != 0 {
					fmt.Printf("        - LPM Capable (Link Power Management)\n")
				}
				if usb2ext.Attributes&0x04 != 0 {
					fmt.Printf("        - BESL and Alternate HIRD Supported\n")
				}
				if usb2ext.Attributes&0x08 != 0 {
					fmt.Printf("        - Baseline BESL Valid\n")
				}
				if usb2ext.Attributes&0x10 != 0 {
					fmt.Printf("        - Deep BESL Valid\n")
				}
			}

			// Try to get SuperSpeed USB capability descriptor
			ssusb, err := handle.GetSSUSBDeviceCapabilityDescriptor()
			if err == nil && ssusb != nil {
				fmt.Printf("    SuperSpeed USB Capability:\n")
				fmt.Printf("      Attributes: 0x%02x\n", ssusb.Attributes)
				if ssusb.Attributes&0x02 != 0 {
					fmt.Printf("        - LTM Capable (Latency Tolerance Messaging)\n")
				}

				fmt.Printf("      Speeds Supported: 0x%04x\n", ssusb.SpeedsSupported)
				if ssusb.SpeedsSupported&0x01 != 0 {
					fmt.Printf("        - Low Speed (1.5 Mbps)\n")
				}
				if ssusb.SpeedsSupported&0x02 != 0 {
					fmt.Printf("        - Full Speed (12 Mbps)\n")
				}
				if ssusb.SpeedsSupported&0x04 != 0 {
					fmt.Printf("        - High Speed (480 Mbps)\n")
				}
				if ssusb.SpeedsSupported&0x08 != 0 {
					fmt.Printf("        - SuperSpeed (5 Gbps)\n")
				}

				fmt.Printf("      Functionality Supported: 0x%02x\n", ssusb.FunctionalitySupported)
				fmt.Printf("      U1 Device Exit Latency: %d µs\n", ssusb.U1DevExitLat)
				fmt.Printf("      U2 Device Exit Latency: %d µs\n", ssusb.U2DevExitLat)
			}
		}

		// Check for SuperSpeed endpoints in configurations
		fmt.Printf("  Configurations with SuperSpeed endpoints:\n")
		for configIdx := uint8(0); configIdx < dev.Descriptor.NumConfigurations; configIdx++ {
			config, err := handle.GetConfigDescriptorByValue(configIdx)
			if err != nil {
				continue
			}

			hasSS := false
			for _, iface := range config.Interfaces {
				for _, alt := range iface.AltSettings {
					for _, ep := range alt.Endpoints {
						if ep.SSCompanion != nil {
							if !hasSS {
								fmt.Printf("    Config %d:\n", configIdx)
								hasSS = true
							}
							epType := "Unknown"
							switch ep.GetTransferType() {
							case 0:
								epType = "Control"
							case 1:
								epType = "Isochronous"
							case 2:
								epType = "Bulk"
							case 3:
								epType = "Interrupt"
							}
							fmt.Printf("      Interface %d Alt %d Endpoint 0x%02x (%s):\n",
								alt.InterfaceNumber, alt.AlternateSetting, ep.EndpointAddr, epType)
							fmt.Printf("        MaxBurst: %d\n", ep.SSCompanion.MaxBurst)
							fmt.Printf("        Attributes: 0x%02x\n", ep.SSCompanion.Attributes)

							// Decode attributes for bulk endpoints
							if ep.GetTransferType() == 2 { // Bulk
								maxStreams := (ep.SSCompanion.Attributes & 0x1f) + 1
								if maxStreams > 1 {
									fmt.Printf("          Max Streams: %d\n", maxStreams)
								}
							}

							// Decode attributes for isochronous endpoints
							if ep.GetTransferType() == 1 { // Isochronous
								mult := (ep.SSCompanion.Attributes & 0x03) + 1
								fmt.Printf("          Mult: %d\n", mult)
							}

							// BytesPerInterval is only used for periodic endpoints (interrupt/isochronous)
							// For bulk endpoints, it's always 0
							if ep.GetTransferType() == 2 { // Bulk
								fmt.Printf("        BytesPerInterval: %d (always 0 for bulk)\n", ep.SSCompanion.BytesPerInterval)
							} else {
								fmt.Printf("        BytesPerInterval: %d\n", ep.SSCompanion.BytesPerInterval)
							}

							// You can also get this directly using the helper method:
							ssComp, err := handle.GetSSEndpointCompanionDescriptor(
								configIdx, alt.InterfaceNumber, alt.AlternateSetting, ep.EndpointAddr)
							if err == nil && ssComp != nil {
								fmt.Printf("        (Verified via GetSSEndpointCompanionDescriptor)\n")
							}
						}
					}
				}
			}
			if !hasSS {
				fmt.Printf("    Config %d: No SuperSpeed endpoints\n", configIdx)
			}
		}

		fmt.Println()
	}

	if !foundAny {
		fmt.Println("\nNo USB 3.0+ devices with BOS descriptors found.")
		fmt.Println("Try connecting a USB 3.0 device to a USB 3.0 port.")
	}
}

func getSpeedString(speed uint8) string {
	switch speed {
	case 1:
		return "Low Speed (1.5 Mbps)"
	case 2:
		return "Full Speed (12 Mbps)"
	case 3:
		return "High Speed (480 Mbps)"
	case 4:
		return "Wireless"
	case 5:
		return "SuperSpeed (5 Gbps)"
	case 6:
		return "SuperSpeed+ (10 Gbps)"
	default:
		return "Unknown"
	}
}
