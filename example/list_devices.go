package main

import (
	"fmt"
	"log"
	"os"

	usb "github.com/kevmo314/go-usb"
)

func main() {
	// Check if running as root
	if os.Getuid() != 0 {
		fmt.Println("Warning: This program may require root privileges to access USB devices")
	}

	// Get device list
	devices, err := usb.DeviceList()
	if err != nil {
		log.Fatalf("Failed to get device list: %v", err)
	}

	fmt.Printf("Found %d USB devices:\n\n", len(devices))

	// List all devices
	for i, dev := range devices {
		desc := dev.Descriptor
		fmt.Printf("Device #%d:\n", i+1)
		fmt.Printf("  Path:        %s\n", dev.Path)
		fmt.Printf("  Bus:         %03d\n", dev.Bus)
		fmt.Printf("  Address:     %03d\n", dev.Address)
		fmt.Printf("  VID:PID:     %04x:%04x\n", desc.VendorID, desc.ProductID)
		fmt.Printf("  USB Version: %x.%02x\n", desc.USBVersion>>8, desc.USBVersion&0xff)
		fmt.Printf("  Class:       %02x\n", desc.DeviceClass)
		fmt.Printf("  SubClass:    %02x\n", desc.DeviceSubClass)
		fmt.Printf("  Protocol:    %02x\n", desc.DeviceProtocol)

		// Try to open device and get strings
		handle, err := dev.Open()
		if err == nil {
			defer handle.Close()

			if desc.ManufacturerIndex > 0 {
				if manufacturer, err := handle.StringDescriptor(desc.ManufacturerIndex); err == nil {
					fmt.Printf("  Manufacturer: %s\n", manufacturer)
				}
			}

			if desc.ProductIndex > 0 {
				if product, err := handle.StringDescriptor(desc.ProductIndex); err == nil {
					fmt.Printf("  Product:     %s\n", product)
				}
			}

			if desc.SerialNumberIndex > 0 {
				if serial, err := handle.StringDescriptor(desc.SerialNumberIndex); err == nil {
					fmt.Printf("  Serial:      %s\n", serial)
				}
			}

			// Get configuration info
			config, interfaces, endpoints, err := handle.ReadConfigDescriptor(0)
			if err == nil {
				fmt.Printf("  Configuration:\n")
				fmt.Printf("    Interfaces: %d\n", config.NumInterfaces)
				fmt.Printf("    MaxPower:   %dmA\n", config.MaxPower*2)

				if len(interfaces) > 0 {
					fmt.Printf("  Interfaces:\n")
					for _, iface := range interfaces {
						fmt.Printf("    Interface %d: Class=%02x SubClass=%02x Protocol=%02x Endpoints=%d\n",
							iface.InterfaceNumber, iface.InterfaceClass,
							iface.InterfaceSubClass, iface.InterfaceProtocol,
							iface.NumEndpoints)
					}
				}

				if len(endpoints) > 0 {
					fmt.Printf("  Endpoints:\n")
					for _, ep := range endpoints {
						dir := "OUT"
						if ep.EndpointAddr&0x80 != 0 {
							dir = "IN"
						}
						epType := ""
						switch ep.Attributes & 0x03 {
						case 0:
							epType = "Control"
						case 1:
							epType = "Isochronous"
						case 2:
							epType = "Bulk"
						case 3:
							epType = "Interrupt"
						}
						fmt.Printf("    Endpoint %02x: %s %s MaxPacket=%d\n",
							ep.EndpointAddr&0x7f, dir, epType, ep.MaxPacketSize)
					}
				}
			}
		} else if err == usb.ErrPermissionDenied {
			fmt.Printf("  (Permission denied - run as root for more details)\n")
		}

		fmt.Println()
	}
}
