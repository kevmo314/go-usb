package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/kevmo314/go-usb"
)

func main() {
	var (
		vid     = flag.Int("vid", 0, "Vendor ID (0 for any)")
		pid     = flag.Int("pid", 0, "Product ID (0 for any)")
		verbose = flag.Bool("v", false, "Verbose output")
	)
	flag.Parse()

	// Get list of USB devices
	devices, err := usb.DeviceList()
	if err != nil {
		log.Fatalf("Failed to get device list: %v", err)
	}

	if len(devices) == 0 {
		fmt.Println("No USB devices found")
		return
	}

	fmt.Printf("Found %d USB devices\n\n", len(devices))

	for _, dev := range devices {
		// Filter by VID/PID if specified
		if *vid != 0 && dev.Descriptor.VendorID != uint16(*vid) {
			continue
		}
		if *pid != 0 && dev.Descriptor.ProductID != uint16(*pid) {
			continue
		}

		fmt.Printf("Device: Bus %03d Device %03d VID:0x%04x PID:0x%04x\n",
			dev.Bus, dev.Address, dev.Descriptor.VendorID, dev.Descriptor.ProductID)

		// Try to open the device
		handle, err := dev.Open()
		if err != nil {
			fmt.Printf("  Cannot open device: %v\n\n", err)
			continue
		}
		defer handle.Close()

		// Get manufacturer and product strings if available
		if dev.Descriptor.ManufacturerIndex > 0 {
			if str, err := handle.StringDescriptor(dev.Descriptor.ManufacturerIndex); err == nil {
				fmt.Printf("  Manufacturer: %s\n", str)
			}
		}
		if dev.Descriptor.ProductIndex > 0 {
			if str, err := handle.StringDescriptor(dev.Descriptor.ProductIndex); err == nil {
				fmt.Printf("  Product: %s\n", str)
			}
		}

		fmt.Printf("  Configurations: %d\n", dev.Descriptor.NumConfigurations)

		// Get each configuration descriptor
		for configIdx := uint8(0); configIdx < dev.Descriptor.NumConfigurations; configIdx++ {
			config, err := handle.ConfigDescriptorByValue(configIdx)
			if err != nil {
				fmt.Printf("    Config %d: Error getting descriptor: %v\n", configIdx, err)
				continue
			}

			printConfig(config, configIdx, *verbose)
		}

		fmt.Println()
	}
}

func printConfig(config *usb.ConfigDescriptor, index uint8, verbose bool) {
	fmt.Printf("    Config %d:\n", index)
	fmt.Printf("      Value: %d\n", config.ConfigurationValue)
	fmt.Printf("      Attributes: 0x%02x", config.Attributes)

	attrs := []string{}
	if config.Attributes&0x80 != 0 {
		attrs = append(attrs, "Bus Powered")
	}
	if config.Attributes&0x40 != 0 {
		attrs = append(attrs, "Self Powered")
	}
	if config.Attributes&0x20 != 0 {
		attrs = append(attrs, "Remote Wakeup")
	}
	if len(attrs) > 0 {
		fmt.Printf(" (%s)", attrs[0])
		for i := 1; i < len(attrs); i++ {
			fmt.Printf(", %s", attrs[i])
		}
	}
	fmt.Println()

	fmt.Printf("      MaxPower: %dmA\n", config.MaxPower*2)
	fmt.Printf("      Interfaces: %d\n", len(config.Interfaces))

	if !verbose {
		return
	}

	// Print detailed interface information
	for ifaceNum, iface := range config.Interfaces {
		fmt.Printf("      Interface %d:\n", ifaceNum)
		fmt.Printf("        Alternate Settings: %d\n", len(iface.AltSettings))

		for _, alt := range iface.AltSettings {
			fmt.Printf("        Alt Setting %d:\n", alt.AlternateSetting)
			fmt.Printf("          Interface Number: %d\n", alt.InterfaceNumber)
			fmt.Printf("          Class: 0x%02x", alt.InterfaceClass)
			fmt.Printf(" (%s)\n", getClassName(alt.InterfaceClass))
			fmt.Printf("          SubClass: 0x%02x\n", alt.InterfaceSubClass)
			fmt.Printf("          Protocol: 0x%02x\n", alt.InterfaceProtocol)
			fmt.Printf("          Endpoints: %d\n", len(alt.Endpoints))

			for _, ep := range alt.Endpoints {
				printEndpoint(&ep)
			}

			if len(alt.Extra) > 0 {
				fmt.Printf("          Extra descriptors: %d bytes\n", len(alt.Extra))
				if verbose {
					fmt.Printf("            %x\n", alt.Extra)
				}
			}
		}
	}

	if len(config.Extra) > 0 {
		fmt.Printf("      Config extra descriptors: %d bytes\n", len(config.Extra))
		if verbose {
			fmt.Printf("        %x\n", config.Extra)
		}
	}
}

func printEndpoint(ep *usb.Endpoint) {
	fmt.Printf("          Endpoint 0x%02x:\n", ep.EndpointAddr)

	direction := "OUT"
	if ep.IsInput() {
		direction = "IN"
	}
	fmt.Printf("            Direction: %s\n", direction)

	transferType := ""
	switch ep.TransferType() {
	case 0: // Control
		transferType = "Control"
	case 1: // Isochronous
		transferType = "Isochronous"
	case 2: // Bulk
		transferType = "Bulk"
	case 3: // Interrupt
		transferType = "Interrupt"
	}
	fmt.Printf("            Type: %s\n", transferType)

	fmt.Printf("            MaxPacketSize: %d\n", ep.MaxPacketSize)
	fmt.Printf("            Interval: %d\n", ep.Interval)

	if ep.SSCompanion != nil {
		fmt.Printf("            SuperSpeed Companion:\n")
		fmt.Printf("              MaxBurst: %d\n", ep.SSCompanion.MaxBurst)
		fmt.Printf("              Attributes: 0x%02x\n", ep.SSCompanion.Attributes)
		fmt.Printf("              BytesPerInterval: %d\n", ep.SSCompanion.BytesPerInterval)
	}
}

func getClassName(class uint8) string {
	switch class {
	case 0x00:
		return "Device"
	case 0x01:
		return "Audio"
	case 0x02:
		return "Communications"
	case 0x03:
		return "HID"
	case 0x05:
		return "Physical"
	case 0x06:
		return "Image"
	case 0x07:
		return "Printer"
	case 0x08:
		return "Mass Storage"
	case 0x09:
		return "Hub"
	case 0x0a:
		return "CDC Data"
	case 0x0b:
		return "Smart Card"
	case 0x0d:
		return "Content Security"
	case 0x0e:
		return "Video"
	case 0x0f:
		return "Personal Healthcare"
	case 0x10:
		return "Audio/Video"
	case 0x11:
		return "Billboard"
	case 0x12:
		return "USB Type-C Bridge"
	case 0xdc:
		return "Diagnostic"
	case 0xe0:
		return "Wireless"
	case 0xef:
		return "Miscellaneous"
	case 0xfe:
		return "Application Specific"
	case 0xff:
		return "Vendor Specific"
	default:
		return "Unknown"
	}
}
