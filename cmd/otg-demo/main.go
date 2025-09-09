package main

import (
	"fmt"
	"log"
	"os"
	"time"
	
	usb "github.com/kevmo314/go-usb"
)

func main() {
	if os.Getuid() != 0 {
		log.Fatal("This program requires root privileges to access USB devices")
	}
	
	fmt.Println("🔄 USB OTG Demo")
	fmt.Println("================")
	
	ctx, err := usb.NewContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()
	
	// Demo 1: Find and analyze OTG-capable devices
	fmt.Println("\n🔍 Scanning for OTG-capable devices...")
	otgDevices := findOTGDevices(ctx)
	
	if len(otgDevices) == 0 {
		fmt.Println("❌ No OTG devices found")
		fmt.Println("💡 Try connecting a USB OTG adapter or OTG-capable device")
	} else {
		fmt.Printf("✅ Found %d OTG-capable device(s)\n", len(otgDevices))
		
		for i, dev := range otgDevices {
			fmt.Printf("\n📱 OTG Device %d: %04x:%04x\n", i+1, 
				dev.Descriptor.VendorID, dev.Descriptor.ProductID)
			
			analyzeOTGDevice(dev)
		}
	}
	
	// Demo 2: Look for USB-C devices with Alt Mode potential
	fmt.Println("\n\n🔌 Scanning for USB-C/Alt Mode devices...")
	altModeDevices := findAltModeDevices(ctx)
	
	if len(altModeDevices) == 0 {
		fmt.Println("❌ No USB-C Alt Mode devices found")
	} else {
		fmt.Printf("✅ Found %d potential Alt Mode device(s)\n", len(altModeDevices))
		
		for i, dev := range altModeDevices {
			fmt.Printf("\n🖥️  Alt Mode Device %d: %04x:%04x\n", i+1,
				dev.Descriptor.VendorID, dev.Descriptor.ProductID)
			
			analyzeAltModeDevice(dev)
		}
	}
	
	fmt.Println("\n✨ OTG/Alt Mode analysis complete!")
}

func findOTGDevices(ctx *usb.Context) []*usb.Device {
	devices, err := ctx.GetDeviceList()
	if err != nil {
		log.Printf("Failed to get device list: %v", err)
		return nil
	}
	
	var otgDevices []*usb.Device
	
	for _, dev := range devices {
		if isOTGCapable(dev) {
			otgDevices = append(otgDevices, dev)
		}
	}
	
	return otgDevices
}

func isOTGCapable(dev *usb.Device) bool {
	// Check for OTG indicators
	desc := dev.Descriptor
	
	// Common OTG device patterns:
	// 1. Devices with multiple configurations (host/peripheral)
	// 2. USB 2.0+ with specific device classes
	// 3. Hub devices that might support OTG
	
	if desc.NumConfigurations > 1 {
		return true // Multiple configs often indicate OTG
	}
	
	// Check for mobile/embedded device patterns
	if desc.DeviceClass == 0 && desc.DeviceSubClass == 0 {
		return true // Composite devices often support OTG
	}
	
	// Hub devices might support OTG
	if desc.DeviceClass == 9 {
		return true
	}
	
	return false
}

func analyzeOTGDevice(dev *usb.Device) {
	handle, err := dev.Open()
	if err != nil {
		fmt.Printf("   ⚠️  Could not open: %v\n", err)
		return
	}
	defer handle.Close()
	
	fmt.Printf("   📊 Device Class: %d, Configs: %d\n", 
		dev.Descriptor.DeviceClass, dev.Descriptor.NumConfigurations)
	
	// Try to read OTG descriptor
	if otgDesc := readOTGDescriptor(handle); otgDesc != nil {
		fmt.Printf("   🎯 OTG Descriptor found!\n")
		fmt.Printf("      HNP Capable: %v\n", (otgDesc.Attributes & 0x02) != 0)
		fmt.Printf("      SRP Capable: %v\n", (otgDesc.Attributes & 0x01) != 0)
		fmt.Printf("      ADP Capable: %v\n", (otgDesc.Attributes & 0x04) != 0)
		
		// Demo OTG feature requests
		demoOTGFeatures(handle)
	}
	
	// Check device status for OTG indicators
	if status, err := handle.GetStatus(0x80, 0); err == nil {
		selfPowered := (status & 0x01) != 0
		remoteWakeup := (status & 0x02) != 0
		
		fmt.Printf("   🔋 Self-powered: %v, Remote wakeup: %v\n", selfPowered, remoteWakeup)
		
		if remoteWakeup {
			fmt.Printf("   ✨ Device supports SRP (Session Request Protocol)\n")
		}
	}
	
	// Analyze configurations for dual-role capability
	analyzeOTGConfigurations(handle)
}

func readOTGDescriptor(handle *usb.DeviceHandle) *usb.OTGDescriptor {
	buf := make([]byte, 5) // OTG descriptor can be 3-5 bytes
	
	n, err := handle.GetRawDescriptor(usb.USB_DT_OTG, 0, 0, buf)
	if err != nil || n < 3 {
		return nil
	}
	
	return &usb.OTGDescriptor{
		Length:         buf[0],
		DescriptorType: buf[1],
		Attributes:     buf[2],
	}
}

func demoOTGFeatures(handle *usb.DeviceHandle) {
	fmt.Printf("   🧪 Testing OTG features...\n")
	
	// Try to enable B-HNP capability
	if err := handle.SetFeature(0x00, usb.USB_DEVICE_B_HNP_ENABLE, 0); err == nil {
		fmt.Printf("      ✅ B-HNP feature enabled\n")
	} else {
		fmt.Printf("      ❌ B-HNP not supported: %v\n", err)
	}
	
	// Try to enable A-HNP support
	if err := handle.SetFeature(0x00, usb.USB_DEVICE_A_HNP_SUPPORT, 0); err == nil {
		fmt.Printf("      ✅ A-HNP support enabled\n")
	} else {
		fmt.Printf("      ❌ A-HNP not supported: %v\n", err)
	}
	
	// Check for alternate HNP support (USB 2.0 supplement)
	if err := handle.SetFeature(0x00, usb.USB_DEVICE_A_ALT_HNP_SUPPORT, 0); err == nil {
		fmt.Printf("      ✅ Alternate A-HNP support enabled\n")
	} else {
		fmt.Printf("      ❌ Alt A-HNP not supported: %v\n", err)
	}
}

func analyzeOTGConfigurations(handle *usb.DeviceHandle) {
	device := handle.GetDevice()
	numConfigs := device.Descriptor.NumConfigurations
	
	if numConfigs > 1 {
		fmt.Printf("   ⚙️  Multiple configurations detected (%d) - possible dual-role device\n", numConfigs)
		
		for i := uint8(0); i < numConfigs; i++ {
			config, interfaces, _, err := handle.ReadConfigDescriptor(i)
			if err != nil {
				continue
			}
			
			fmt.Printf("      Config %d: %d interfaces, %dmA power\n", 
				config.ConfigurationValue, config.NumInterfaces, config.MaxPower*2)
			
			// Analyze interface classes for role hints
			for _, iface := range interfaces {
				role := classifyOTGRole(iface.InterfaceClass, iface.InterfaceSubClass)
				if role != "" {
					fmt.Printf("         Interface %d: %s role (%d/%d)\n",
						iface.InterfaceNumber, role, iface.InterfaceClass, iface.InterfaceSubClass)
				}
			}
		}
	}
}

func classifyOTGRole(class, subclass uint8) string {
	switch class {
	case 3: // HID
		return "Host" // HID typically indicates host capability
	case 7: // Printer
		return "Host"
	case 8: // Mass Storage
		return "Host/Peripheral" // Can be either
	case 9: // Hub
		return "Host"
	case 10: // CDC Data
		return "Peripheral"
	case 14: // Video
		return "Host/Peripheral"
	default:
		return ""
	}
}

func findAltModeDevices(ctx *usb.Context) []*usb.Device {
	devices, err := ctx.GetDeviceList()
	if err != nil {
		return nil
	}
	
	var altModeDevices []*usb.Device
	
	for _, dev := range devices {
		if isAltModeCapable(dev) {
			altModeDevices = append(altModeDevices, dev)
		}
	}
	
	return altModeDevices
}

func isAltModeCapable(dev *usb.Device) bool {
	desc := dev.Descriptor
	
	// Look for USB-C indicators:
	// 1. USB 3.x support (USB-C commonly uses USB 3.x)
	// 2. Hub devices (USB-C hubs are common)
	// 3. High-power devices (USB-C supports higher power)
	// 4. Specific vendor/device patterns
	
	if desc.USBVersion >= 0x0300 { // USB 3.0+
		return true
	}
	
	if desc.DeviceClass == 9 && desc.USBVersion >= 0x0210 { // USB 2.1+ hub
		return true
	}
	
	// Check for known USB-C device vendors
	knownUSBCVendors := []uint16{
		0x17EF, // Lenovo
		0x0BDA, // Realtek (USB-C controllers)
		0x2109, // VIA Labs (USB-C hubs)
		0x05E3, // Genesys Logic (USB-C hubs)
		0x1A40, // Terminus Technology (USB-C hubs)
	}
	
	for _, vendor := range knownUSBCVendors {
		if desc.VendorID == vendor {
			return true
		}
	}
	
	return false
}

func analyzeAltModeDevice(dev *usb.Device) {
	handle, err := dev.Open()
	if err != nil {
		fmt.Printf("   ⚠️  Could not open: %v\n", err)
		return
	}
	defer handle.Close()
	
	fmt.Printf("   📊 USB %d.%d, Class: %d\n", 
		dev.Descriptor.USBVersion>>8, (dev.Descriptor.USBVersion&0xF0)>>4,
		dev.Descriptor.DeviceClass)
	
	// Try to read BOS descriptor (USB 3.0+ feature)
	if _, caps, err := handle.ReadBOSDescriptor(); err == nil {
		fmt.Printf("   📄 BOS descriptor found: %d capabilities\n", len(caps))
		
		for i, cap := range caps {
			capName := getCapabilityName(cap.DevCapabilityType)
			fmt.Printf("      Cap %d: %s (0x%02x)\n", i, capName, cap.DevCapabilityType)
			
			if cap.DevCapabilityType == 0x03 { // SuperSpeed USB
				fmt.Printf("         🚀 SuperSpeed USB capability detected\n")
			}
		}
	}
	
	// Check if device supports advanced features
	analyzeAltModeCapabilities(handle)
	
	// Simulate DisplayPort Alt Mode detection
	simulateDisplayPortDetection(handle)
}

func getCapabilityName(capType uint8) string {
	names := map[uint8]string{
		0x01: "Wireless USB",
		0x02: "USB 2.0 Extension",
		0x03: "SuperSpeed USB",
		0x04: "Container ID",
		0x05: "Platform",
		0x06: "Power Delivery",
		0x0A: "SuperSpeedPlus USB",
	}
	
	if name, ok := names[capType]; ok {
		return name
	}
	return "Unknown"
}

func analyzeAltModeCapabilities(handle *usb.DeviceHandle) {
	// Check usbfs capabilities
	if caps, err := handle.GetCapabilities(); err == nil {
		fmt.Printf("   🛠️  usbfs capabilities: 0x%08x\n", caps)
		
		if caps&0x08 != 0 {
			fmt.Printf("      ✅ Bulk streams supported (USB 3.0+ feature)\n")
		}
		
		if caps&0x04 != 0 {
			fmt.Printf("      ✅ No packet size limit (high-bandwidth capable)\n")
		}
	}
	
	// Try to get device speed
	if speed, err := handle.GetSpeed(); err == nil {
		speedNames := map[uint8]string{
			1: "Low Speed (1.5 Mbps)",
			2: "Full Speed (12 Mbps)",
			3: "High Speed (480 Mbps)",
			4: "Wireless (480 Mbps)",
			5: "Super Speed (5 Gbps)",
			6: "Super Speed+ (10 Gbps)",
		}
		
		if speedName, ok := speedNames[speed]; ok {
			fmt.Printf("   🚀 Speed: %s\n", speedName)
		}
		
		if speed >= 5 { // SuperSpeed+
			fmt.Printf("      ✨ High-speed capability suitable for video alt mode\n")
		}
	}
}

func simulateDisplayPortDetection(handle *usb.DeviceHandle) {
	fmt.Printf("   🖥️  Simulating DisplayPort Alt Mode detection...\n")
	
	// In a real implementation, this would involve:
	// 1. USB Power Delivery communication
	// 2. Structured VDM (Vendor Defined Message) exchange
	// 3. SVID discovery and mode negotiation
	
	// For demo purposes, we'll check device characteristics
	device := handle.GetDevice()
	
	// Simulate VDM discovery based on device properties
	if device.Descriptor.USBVersion >= 0x0300 {
		fmt.Printf("      📡 USB 3.0+ detected - Alt Mode communication possible\n")
		
		// Simulate discovering DisplayPort SVID
		fmt.Printf("      🔍 Discovering Structured VDM support...\n")
		time.Sleep(100 * time.Millisecond) // Simulate communication delay
		
		// Check if device might support DisplayPort
		if couldSupportDisplayPort(device) {
			fmt.Printf("      🎯 DisplayPort Alt Mode potentially supported!\n")
			fmt.Printf("         Pin assignments: C, D, E (simulated)\n")
			fmt.Printf("         Max resolution: 4K@60Hz (simulated)\n")
			fmt.Printf("         Multi-function: Yes (simulated)\n")
		} else {
			fmt.Printf("      ❌ DisplayPort Alt Mode not detected\n")
		}
	} else {
		fmt.Printf("      ⚠️  USB 2.0 device - Alt Mode requires USB 3.0+\n")
	}
}

func couldSupportDisplayPort(device *usb.Device) bool {
	// Heuristics for DisplayPort Alt Mode capability
	desc := device.Descriptor
	
	// Hub devices often support Alt Mode
	if desc.DeviceClass == 9 {
		return true
	}
	
	// High-speed devices with multiple configurations
	if desc.USBVersion >= 0x0300 && desc.NumConfigurations > 1 {
		return true
	}
	
	// Known DisplayPort-capable vendors (example)
	dpVendors := []uint16{0x17EF, 0x2109, 0x0BDA}
	for _, vendor := range dpVendors {
		if desc.VendorID == vendor {
			return true
		}
	}
	
	return false
}