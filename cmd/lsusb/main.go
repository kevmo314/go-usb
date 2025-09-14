package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	usb "github.com/kevmo314/go-usb"
)

var (
	verbose    = flag.Bool("v", false, "Verbose output")
	tree       = flag.Bool("t", false, "Tree display")
	device     = flag.String("d", "", "Show only devices with specified VID:PID (e.g., 1234:5678)")
	busDevice  = flag.String("s", "", "Show only devices with specified [[bus]:][devnum] (e.g., 1:6, :6, 1:)")
	version    = flag.Bool("V", false, "Show version")
	devicePath = flag.String("D", "", "Show information for specific device path")
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("lsusb (go-usb) %s\n", usb.Version())
		fmt.Println("Copyright (C) 2024 go-usb project")
		return
	}

	// Get device list
	devices, err := usb.DeviceList()
	if err != nil {
		log.Fatalf("Failed to get device list: %v", err)
	}

	// Filter devices if needed
	filteredDevices := filterDevices(devices)

	// Sort devices by bus and address
	sort.Slice(filteredDevices, func(i, j int) bool {
		if filteredDevices[i].Bus != filteredDevices[j].Bus {
			return filteredDevices[i].Bus < filteredDevices[j].Bus
		}
		return filteredDevices[i].Address < filteredDevices[j].Address
	})

	if *tree {
		displayTree(filteredDevices)
	} else if *verbose {
		displayVerbose(filteredDevices)
	} else {
		displaySimple(filteredDevices)
	}
}

func filterDevices(devices []*usb.Device) []*usb.Device {
	var filtered []*usb.Device

	for _, dev := range devices {
		// Filter by bus:device
		if *busDevice != "" {
			var busNum, devNum int = -1, -1

			// Parse [[bus]:][devnum] format
			if strings.Contains(*busDevice, ":") {
				parts := strings.Split(*busDevice, ":")
				if len(parts) == 2 {
					if parts[0] != "" {
						busNum, _ = strconv.Atoi(parts[0])
					}
					if parts[1] != "" {
						devNum, _ = strconv.Atoi(parts[1])
					}
				}
			} else {
				// No colon means it's just the device number
				devNum, _ = strconv.Atoi(*busDevice)
			}

			// Apply filters
			if busNum >= 0 && dev.Bus != uint8(busNum) {
				continue
			}
			if devNum >= 0 && dev.Address != uint8(devNum) {
				continue
			}
		}

		// Filter by device path
		if *devicePath != "" && dev.Path != *devicePath {
			continue
		}

		// Filter by VID:PID
		if *device != "" {
			parts := strings.Split(*device, ":")
			if len(parts) == 2 {
				var vid, pid uint16
				fmt.Sscanf(parts[0], "%x", &vid)
				fmt.Sscanf(parts[1], "%x", &pid)
				if dev.Descriptor.VendorID != vid || dev.Descriptor.ProductID != pid {
					continue
				}
			}
		}

		filtered = append(filtered, dev)
	}

	return filtered
}

func displaySimple(devices []*usb.Device) {
	for _, dev := range devices {
		desc := dev.Descriptor

		vendorName := usb.VendorName(desc.VendorID)
		productName := usb.ProductName(desc.VendorID, desc.ProductID)

		// Try to get from sysfs first (faster)
		if productName == "" && dev.SysfsStrings != nil {
			if sysfsProduct := dev.SysfsStrings.Product; sysfsProduct != "" {
				productName = sysfsProduct
			}
		}

		fmt.Printf("Bus %03d Device %03d: ID %04x:%04x %s %s\n",
			dev.Bus, dev.Address,
			desc.VendorID, desc.ProductID,
			vendorName, productName)
	}
}

func displayVerbose(devices []*usb.Device) {
	for _, dev := range devices {
		desc := dev.Descriptor

		fmt.Printf("\nBus %03d Device %03d: ID %04x:%04x\n",
			dev.Bus, dev.Address,
			desc.VendorID, desc.ProductID)

		fmt.Printf("Device Descriptor:\n")
		fmt.Printf("  bLength             %5d\n", desc.Length)
		fmt.Printf("  bDescriptorType     %5d\n", desc.DescriptorType)
		fmt.Printf("  bcdUSB              %2d.%02d\n", desc.USBVersion>>8, desc.USBVersion&0xff)
		className := usb.ClassName(desc.DeviceClass)
		if className != "" {
			fmt.Printf("  bDeviceClass        %5d %s\n", desc.DeviceClass, className)
		} else {
			fmt.Printf("  bDeviceClass        %5d\n", desc.DeviceClass)
		}
		fmt.Printf("  bDeviceSubClass     %5d\n", desc.DeviceSubClass)

		protocolDesc := getProtocolDescription(desc.DeviceClass, desc.DeviceProtocol)
		if protocolDesc != "" {
			fmt.Printf("  bDeviceProtocol     %5d %s\n", desc.DeviceProtocol, protocolDesc)
		} else {
			fmt.Printf("  bDeviceProtocol     %5d\n", desc.DeviceProtocol)
		}
		fmt.Printf("  bMaxPacketSize0     %5d\n", desc.MaxPacketSize0)
		fmt.Printf("  idVendor           0x%04x %s\n", desc.VendorID, usb.VendorName(desc.VendorID))
		fmt.Printf("  idProduct          0x%04x %s\n", desc.ProductID, usb.ProductName(desc.VendorID, desc.ProductID))
		fmt.Printf("  bcdDevice           %2d.%02d\n", desc.DeviceVersion>>8, desc.DeviceVersion&0xff)
		fmt.Printf("  iManufacturer       %5d\n", desc.ManufacturerIndex)
		fmt.Printf("  iProduct            %5d\n", desc.ProductIndex)
		fmt.Printf("  iSerialNumber       %5d\n", desc.SerialNumberIndex)
		fmt.Printf("  bNumConfigurations  %5d\n", desc.NumConfigurations)

		// Try to open device for more info
		handle, err := dev.Open()
		if err == nil {
			defer handle.Close()

			// Get string descriptors
			if desc.ManufacturerIndex > 0 {
				if str, err := handle.StringDescriptor(desc.ManufacturerIndex); err == nil && str != "" {
					fmt.Printf("  Manufacturer: %s\n", str)
				}
			}

			if desc.ProductIndex > 0 {
				if str, err := handle.StringDescriptor(desc.ProductIndex); err == nil && str != "" {
					fmt.Printf("  Product: %s\n", str)
				}
			}

			if desc.SerialNumberIndex > 0 {
				if str, err := handle.StringDescriptor(desc.SerialNumberIndex); err == nil && str != "" {
					fmt.Printf("  Serial Number: %s\n", str)
				}
			}

			// Get configuration descriptor
			for i := uint8(0); i < desc.NumConfigurations; i++ {
				config, interfaces, endpoints, err := handle.ReadConfigDescriptor(i)
				if err != nil {
					continue
				}

				fmt.Printf("  Configuration Descriptor:\n")
				fmt.Printf("    bLength             %5d\n", config.Length)
				fmt.Printf("    bDescriptorType     %5d\n", config.DescriptorType)
				fmt.Printf("    wTotalLength       0x%04x\n", config.TotalLength)
				fmt.Printf("    bNumInterfaces      %5d\n", config.NumInterfaces)
				fmt.Printf("    bConfigurationValue %5d\n", config.ConfigurationValue)
				fmt.Printf("    iConfiguration      %5d\n", config.ConfigurationIndex)
				fmt.Printf("    bmAttributes         0x%02x\n", config.Attributes)

				if config.Attributes&0x80 != 0 {
					fmt.Printf("      (Bus Powered)\n")
				}
				if config.Attributes&0x40 != 0 {
					fmt.Printf("      Self Powered\n")
				}
				if config.Attributes&0x20 != 0 {
					fmt.Printf("      Remote Wakeup\n")
				}

				fmt.Printf("    MaxPower            %5dmA\n", config.MaxPower*2)

				// Display interfaces
				for _, iface := range interfaces {
					fmt.Printf("    Interface Descriptor:\n")
					fmt.Printf("      bLength             %5d\n", iface.Length)
					fmt.Printf("      bDescriptorType     %5d\n", iface.DescriptorType)
					fmt.Printf("      bInterfaceNumber    %5d\n", iface.InterfaceNumber)
					fmt.Printf("      bAlternateSetting   %5d\n", iface.AlternateSetting)
					fmt.Printf("      bNumEndpoints       %5d\n", iface.NumEndpoints)
					fmt.Printf("      bInterfaceClass     %5d %s\n", iface.InterfaceClass, usb.ClassName(iface.InterfaceClass))
					fmt.Printf("      bInterfaceSubClass  %5d\n", iface.InterfaceSubClass)
					fmt.Printf("      bInterfaceProtocol  %5d\n", iface.InterfaceProtocol)
					fmt.Printf("      iInterface          %5d\n", iface.InterfaceIndex)
				}

				// Display endpoints
				for _, ep := range endpoints {
					fmt.Printf("      Endpoint Descriptor:\n")
					fmt.Printf("        bLength             %5d\n", ep.Length)
					fmt.Printf("        bDescriptorType     %5d\n", ep.DescriptorType)
					fmt.Printf("        bEndpointAddress     0x%02x  EP %d %s\n",
						ep.EndpointAddr,
						ep.EndpointAddr&0x7f,
						getEndpointDirection(ep.EndpointAddr))
					fmt.Printf("        bmAttributes         0x%02x\n", ep.Attributes)
					fmt.Printf("          Transfer Type            %s\n", getTransferType(ep.Attributes))
					fmt.Printf("          Synch Type               %s\n", getSynchType(ep.Attributes))
					fmt.Printf("          Usage Type               %s\n", getUsageType(ep.Attributes))
					fmt.Printf("        wMaxPacketSize     0x%04x\n", ep.MaxPacketSize)
					fmt.Printf("        bInterval           %5d\n", ep.Interval)
				}
			}
		} else if os.Getuid() != 0 {
			fmt.Printf("  (Run as root for more details)\n")
		}
	}
}

func displayTree(devices []*usb.Device) {
	// Group devices by bus
	busMap := make(map[uint8][]*usb.Device)
	for _, dev := range devices {
		busMap[dev.Bus] = append(busMap[dev.Bus], dev)
	}

	// Sort buses
	var buses []uint8
	for bus := range busMap {
		buses = append(buses, bus)
	}
	sort.Slice(buses, func(i, j int) bool {
		return buses[i] < buses[j]
	})

	// Display tree in lsusb format
	for _, bus := range buses {
		busDevices := busMap[bus]

		// Sort devices by address
		sort.Slice(busDevices, func(i, j int) bool {
			return busDevices[i].Address < busDevices[j].Address
		})

		// Find root hub
		var rootHub *usb.Device
		var otherDevices []*usb.Device

		for _, dev := range busDevices {
			if dev.Address == 1 && dev.Descriptor.DeviceClass == 9 {
				rootHub = dev
			} else {
				otherDevices = append(otherDevices, dev)
			}
		}

		if rootHub != nil {
			speed := getSpeedString(rootHub)
			maxPorts := getMaxPorts(rootHub)

			fmt.Printf("/:  Bus %03d.Port 001: Dev 001, Class=root_hub, Driver=xhci_hcd/%dp, %s\n",
				bus, maxPorts, speed)

			// Display connected devices
			for _, dev := range otherDevices {
				displayDeviceTree(dev, "    ")
			}
		}
	}
}

func getEndpointDirection(addr uint8) string {
	if addr&0x80 != 0 {
		return "IN"
	}
	return "OUT"
}

func getTransferType(attr uint8) string {
	switch attr & 0x03 {
	case 0:
		return "Control"
	case 1:
		return "Isochronous"
	case 2:
		return "Bulk"
	case 3:
		return "Interrupt"
	}
	return "Unknown"
}

func getSynchType(attr uint8) string {
	switch (attr >> 2) & 0x03 {
	case 0:
		return "None"
	case 1:
		return "Asynchronous"
	case 2:
		return "Adaptive"
	case 3:
		return "Synchronous"
	}
	return "Unknown"
}

func getUsageType(attr uint8) string {
	switch (attr >> 4) & 0x03 {
	case 0:
		return "Data"
	case 1:
		return "Feedback"
	case 2:
		return "Implicit feedback"
	case 3:
		return "Reserved"
	}
	return "Unknown"
}

func getProtocolDescription(class, protocol uint8) string {
	switch class {
	case 9: // Hub
		switch protocol {
		case 1:
			return "Single TT"
		case 2:
			return "TT per port"
		}
	case 0xef: // Miscellaneous Device
		if protocol == 1 {
			return "Interface Association"
		}
	}
	return ""
}

func getSpeedString(dev *usb.Device) string {
	// Try to read speed from sysfs
	sysfsPath := fmt.Sprintf("/sys/bus/usb/devices/%s", getSysfsDeviceName(dev))
	if speedData, err := os.ReadFile(filepath.Join(sysfsPath, "speed")); err == nil {
		speed := strings.TrimSpace(string(speedData))
		switch speed {
		case "1.5":
			return "1.5M"
		case "12":
			return "12M"
		case "480":
			return "480M"
		case "5000":
			return "5000M"
		case "10000":
			return "10000M"
		case "20000":
			return "20000M"
		default:
			return speed + "M"
		}
	}

	// Fallback based on USB version
	version := dev.Descriptor.USBVersion
	if version >= 0x0300 {
		return "5000M"
	} else if version >= 0x0200 {
		return "480M"
	} else if version >= 0x0110 {
		return "12M"
	}
	return "1.5M"
}

func getMaxPorts(dev *usb.Device) int {
	// Try to read maxchild from sysfs
	sysfsPath := fmt.Sprintf("/sys/bus/usb/devices/%s", getSysfsDeviceName(dev))
	if maxChildData, err := os.ReadFile(filepath.Join(sysfsPath, "maxchild")); err == nil {
		if maxChild, err := strconv.Atoi(strings.TrimSpace(string(maxChildData))); err == nil {
			return maxChild
		}
	}
	return 4 // Default fallback
}

func getSysfsDeviceName(dev *usb.Device) string {
	if dev.Address == 1 {
		return fmt.Sprintf("usb%d", dev.Bus)
	}
	// For non-root devices, we'd need to parse the topology
	// This is simplified - real implementation would need to track ports
	return fmt.Sprintf("%d-%d", dev.Bus, dev.Address-1)
}

func displayDeviceTree(dev *usb.Device, indent string) {
	className := getDeviceClassName(dev.Descriptor.DeviceClass)
	speed := getSpeedString(dev)

	// For now, use a simplified port number (would need proper topology parsing)
	portNum := int(dev.Address) - 1
	if portNum < 1 {
		portNum = 1
	}

	fmt.Printf("%s|__ Port %03d: Dev %03d, If 0, Class=%s, Driver=[unknown], %s\n",
		indent, portNum, dev.Address, className, speed)
}

func getDeviceClassName(class uint8) string {
	switch class {
	case 0:
		return "Use class info in Interface Descriptors"
	case 1:
		return "Audio"
	case 2:
		return "Communications"
	case 3:
		return "Human Interface Device"
	case 5:
		return "Physical"
	case 6:
		return "Image"
	case 7:
		return "Printer"
	case 8:
		return "Mass Storage"
	case 9:
		return "Hub"
	case 10:
		return "CDC Data"
	case 11:
		return "Smart Card"
	case 13:
		return "Content Security"
	case 14:
		return "Video"
	case 15:
		return "Personal Healthcare"
	case 16:
		return "Audio/Video Devices"
	case 220:
		return "Diagnostic"
	case 224:
		return "Wireless"
	case 239:
		return "Miscellaneous Device"
	case 254:
		return "Application Specific"
	case 255:
		return "Vendor Specific"
	default:
		return fmt.Sprintf("Unknown(%d)", class)
	}
}
