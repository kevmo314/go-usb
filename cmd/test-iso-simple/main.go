package main

import (
	"fmt"
	"log"
	"os"
	
	usb "github.com/kevmo314/go-usb"
)

func main() {
	if os.Getuid() != 0 {
		log.Fatal("This program requires root privileges")
	}
	
	fmt.Println("🧪 Simple Isochronous Transfer Test")
	fmt.Println("====================================")
	
	ctx, err := usb.NewContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()
	
	devices, err := ctx.GetDeviceList()
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("📋 Found %d USB devices\n", len(devices))
	
	// Test isochronous transfer creation with each device
	tested := 0
	for i, dev := range devices {
		// Skip hubs
		if dev.Descriptor.DeviceClass == 9 {
			continue
		}
		
		fmt.Printf("\n🔍 Device %d: %04x:%04x\n", i, 
			dev.Descriptor.VendorID, dev.Descriptor.ProductID)
		
		handle, err := dev.Open()
		if err != nil {
			fmt.Printf("   ⚠️  Could not open: %v\n", err)
			continue
		}
		
		// Test isochronous transfer creation
		testIsochronousCreation(handle)
		
		handle.Close()
		tested++
		
		if tested >= 3 {
			break
		}
	}
	
	fmt.Println("\n✅ Test completed")
}

func testIsochronousCreation(handle *usb.DeviceHandle) {
	// Try to create an isochronous transfer
	// Use a typical endpoint (0x81 = EP1 IN)
	const (
		endpoint   = 0x81
		numPackets = 8
		packetSize = 1024
	)
	
	fmt.Printf("   📦 Creating isochronous transfer...\n")
	
	transfer, err := handle.NewIsochronousTransfer(endpoint, numPackets, packetSize)
	if err != nil {
		fmt.Printf("   ❌ Failed to create transfer: %v\n", err)
		return
	}
	
	fmt.Printf("   ✅ Transfer created successfully\n")
	fmt.Printf("      Endpoint: 0x%02x\n", endpoint)
	fmt.Printf("      Packets: %d × %d bytes\n", numPackets, packetSize)
	fmt.Printf("      Buffer size: %d bytes\n", numPackets*packetSize)
	
	// Test transfer properties
	buffer := transfer.GetBuffer()
	packets := transfer.GetPackets()
	
	fmt.Printf("   📊 Transfer properties:\n")
	fmt.Printf("      Buffer allocated: %d bytes\n", len(buffer))
	fmt.Printf("      Packet descriptors: %d\n", len(packets))
	
	// Verify packet initialization
	for i, packet := range packets {
		if packet.Length != uint32(packetSize) {
			fmt.Printf("   ⚠️  Packet %d has wrong size: %d\n", i, packet.Length)
		}
	}
	
	fmt.Printf("   ✅ Transfer structure valid\n")
	
	// Don't submit the transfer since we don't have the interface claimed
	// This is just testing the creation and memory allocation
}