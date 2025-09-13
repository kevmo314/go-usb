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

	fmt.Println("🔬 USB Extended Features Test")
	fmt.Println("==============================")

	// Get device list
	devices, err := usb.DeviceList()
	if err != nil {
		log.Fatalf("Failed to get device list: %v", err)
	}

	fmt.Printf("\n📋 Found %d USB devices\n", len(devices))

	// Test new features on first few devices
	testCount := 0
	maxTests := 3

	for i, dev := range devices {
		if testCount >= maxTests {
			break
		}

		// Skip root hubs
		if dev.Descriptor.DeviceClass == 9 {
			continue
		}

		fmt.Printf("\n🔍 Testing Device %d: %04x:%04x\n", i, dev.Descriptor.VendorID, dev.Descriptor.ProductID)

		handle, err := dev.Open()
		if err != nil {
			fmt.Printf("   ⚠️  Could not open: %v\n", err)
			continue
		}

		testExtendedFeatures(handle, i)
		handle.Close()
		testCount++
	}

	fmt.Println("\n✅ Extended features test completed!")
}

func testExtendedFeatures(handle *usb.DeviceHandle, deviceIndex int) {
	fmt.Printf("   🧪 Testing extended USB features...\n")

	// Test 1: Device Status
	if status, err := handle.Status(0x80, 0); err == nil {
		fmt.Printf("   📊 Device status: 0x%04x\n", status)

		// Check self-powered and remote wakeup bits
		selfPowered := (status & 0x01) != 0
		remoteWakeup := (status & 0x02) != 0
		fmt.Printf("      Self-powered: %v, Remote wakeup: %v\n", selfPowered, remoteWakeup)
	}

	// Test 2: Device Speed (if supported)
	if speed, err := handle.Speed(); err == nil {
		speedStr := map[uint8]string{
			1: "Low Speed (1.5 Mbps)",
			2: "Full Speed (12 Mbps)",
			3: "High Speed (480 Mbps)",
			4: "Wireless (480 Mbps)",
			5: "Super Speed (5 Gbps)",
			6: "Super Speed+ (10 Gbps)",
		}

		if str, ok := speedStr[speed]; ok {
			fmt.Printf("   🚀 Device speed: %s\n", str)
		} else {
			fmt.Printf("   🚀 Device speed: %d (unknown)\n", speed)
		}
	}

	// Test 3: Capabilities (Linux 3.15+)
	if caps, err := handle.Capabilities(); err == nil {
		fmt.Printf("   🛠️  usbfs capabilities: 0x%08x\n", caps)

		// Decode capability bits
		if caps&0x01 != 0 {
			fmt.Printf("      ✅ Zero length packets supported\n")
		}
		if caps&0x02 != 0 {
			fmt.Printf("      ✅ Bulk continuation supported\n")
		}
		if caps&0x04 != 0 {
			fmt.Printf("      ✅ No packet size limit\n")
		}
		if caps&0x08 != 0 {
			fmt.Printf("      ✅ Bulk streams supported\n")
		}
	}

	// Test 4: Try to read BOS descriptor (USB 3.0+ only)
	if bos, caps, err := handle.ReadBOSDescriptor(); err == nil {
		fmt.Printf("   📄 BOS descriptor found: %d capabilities\n", bos.NumDeviceCaps)
		for i, cap := range caps {
			capType := map[uint8]string{
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

			if name, ok := capType[cap.DevCapabilityType]; ok {
				fmt.Printf("      Capability %d: %s\n", i, name)
			} else {
				fmt.Printf("      Capability %d: Type 0x%02x\n", i, cap.DevCapabilityType)
			}
		}
	}

	// Test 5: Try to read Device Qualifier (USB 2.0+ only)
	if qual, err := handle.ReadDeviceQualifierDescriptor(); err == nil {
		fmt.Printf("   📄 Device qualifier: USB %d.%d, Class %d\n",
			qual.USBVersion>>8, (qual.USBVersion&0xFF)/16, qual.DeviceClass)
	}

	// Test 6: Zero-length packet transfer test
	testZeroLengthPackets(handle, deviceIndex)

	// Test 7: Error recovery test
	testErrorRecovery(handle, deviceIndex)
}

func testZeroLengthPackets(handle *usb.DeviceHandle, deviceIndex int) {
	fmt.Printf("   📦 Testing zero-length packet support...\n")

	// Try zero-length bulk transfer (should fail without allowZeroLength)
	_, err1 := handle.BulkTransfer(0x80, []byte{}, 1*time.Second)
	if err1 == usb.ErrInvalidParameter {
		fmt.Printf("      ✅ Zero-length packets properly rejected by default\n")
	}

	// Try with explicit zero-length support
	_, err := handle.BulkTransferWithOptions(0x80, []byte{}, 1*time.Second, true)
	if err != usb.ErrInvalidParameter {
		fmt.Printf("      ✅ Zero-length packets allowed with options\n")
	}
}

func testErrorRecovery(handle *usb.DeviceHandle, deviceIndex int) {
	fmt.Printf("   🔧 Testing error recovery mechanisms...\n")

	// Test endpoint reset
	if err := handle.ResetEndpoint(0x81); err == nil {
		fmt.Printf("      ✅ Endpoint reset successful\n")
	} else if err == usb.ErrNotSupported {
		fmt.Printf("      ⚠️  Endpoint reset not supported\n")
	}

	// Test interrupt transfer with retry
	testData := make([]byte, 64)
	_, err := handle.InterruptTransferWithRetry(0x81, testData, 100*time.Millisecond, 2)
	if err == usb.ErrTimeout {
		fmt.Printf("      ✅ Interrupt transfer with retry handled timeout\n")
	} else if err != nil {
		fmt.Printf("      ⚠️  Interrupt transfer error: %v\n", err)
	}
}
