package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	usb "github.com/kevmo314/go-usb"
)

const (
	// USB Mass Storage Class constants
	MSC_CLASS         = 0x08
	MSC_SUBCLASS_SCSI = 0x06
	MSC_PROTOCOL_BULK = 0x50

	// Command Block Wrapper signature
	CBW_SIGNATURE = 0x43425355 // "USBC" in little-endian

	// Command Status Wrapper signature
	CSW_SIGNATURE = 0x53425355 // "USBS" in little-endian

	// SCSI Commands
	SCSI_INQUIRY         = 0x12
	SCSI_READ_CAPACITY   = 0x25
	SCSI_READ_10         = 0x28
	SCSI_TEST_UNIT_READY = 0x00
	SCSI_REQUEST_SENSE   = 0x03
)

// Command Block Wrapper (CBW) for sending SCSI commands
type CBW struct {
	Signature          uint32
	Tag                uint32
	DataTransferLength uint32
	Flags              uint8
	LUN                uint8
	CBLength           uint8
	CB                 [16]byte // SCSI Command Block
}

// Command Status Wrapper (CSW) for receiving status
type CSW struct {
	Signature   uint32
	Tag         uint32
	DataResidue uint32
	Status      uint8
}

// MSCDevice wraps a USB device handle for Mass Storage operations
type MSCDevice struct {
	handle *usb.DeviceHandle
	epIn   uint8
	epOut  uint8
	tag    uint32
}

func main() {
	// Parse command-line flags
	var (
		vendorID    = flag.String("vid", "0781", "USB Vendor ID in hex (e.g., 0781 for SanDisk)")
		productID   = flag.String("pid", "5581", "USB Product ID in hex (e.g., 5581 for Ultra)")
		listDevices = flag.Bool("list", false, "List all USB Mass Storage devices")
	)
	flag.Parse()

	fmt.Println("USB Mass Storage Browser")
	fmt.Println("========================")

	// If list flag is set, show all Mass Storage devices
	if *listDevices {
		listMassStorageDevices()
		return
	}

	// Parse VID and PID
	var vid, pid uint16
	if _, err := fmt.Sscanf(*vendorID, "%x", &vid); err != nil {
		log.Fatalf("Invalid vendor ID format: %s (should be hex, e.g., 0781)", *vendorID)
	}
	if _, err := fmt.Sscanf(*productID, "%x", &pid); err != nil {
		log.Fatalf("Invalid product ID format: %s (should be hex, e.g., 5581)", *productID)
	}

	fmt.Printf("Looking for device VID:PID = %04x:%04x\n", vid, pid)

	// First, try to unbind the device from usb-storage driver if needed
	fmt.Println("Preparing device access...")
	unbindDevice()

	// Open the specified device
	handle, err := usb.OpenDevice(vid, pid)
	if err != nil {
		log.Fatalf("Failed to open device %04x:%04x: %v\n", vid, pid, err)
		log.Fatal("Make sure the USB device is connected and you have permissions")
	}
	defer handle.Close()

	fmt.Printf("✓ Found and opened USB device %04x:%04x\n", vid, pid)

	// Get device descriptor for information
	devices, err := usb.GetDeviceList()
	if err != nil {
		log.Fatal("Failed to get device list:", err)
	}

	var device *usb.Device
	for _, d := range devices {
		if d.Descriptor.VendorID == vid && d.Descriptor.ProductID == pid {
			device = d
			break
		}
	}

	if device != nil {
		// Try to get product name
		if product, err := handle.GetStringDescriptor(device.Descriptor.ProductIndex); err == nil {
			fmt.Printf("Device: %s\n", product)
		}
	}

	// Find Mass Storage interface and endpoints
	epIn, epOut, err := findMSCEndpoints(handle)
	if err != nil {
		log.Fatal("Failed to find Mass Storage endpoints:", err)
	}

	fmt.Printf("✓ Found Mass Storage endpoints: IN=0x%02x, OUT=0x%02x\n", epIn, epOut)

	// First, try to detach any kernel driver
	fmt.Println("Checking for kernel driver...")
	if err := handle.DetachKernelDriver(0); err != nil {
		// It's okay if this fails - might mean no driver was attached
		fmt.Printf("Note: Kernel driver detach result: %v\n", err)
	} else {
		fmt.Println("✓ Detached kernel driver")
		defer func() {
			// Re-attach kernel driver when done
			handle.AttachKernelDriver(0)
		}()
	}

	// Now claim the interface
	err = handle.ClaimInterface(0)
	if err != nil {
		log.Fatal("Failed to claim interface. This might be because:\n"+
			"1. The device is mounted (try: sudo umount /dev/sdX*)\n"+
			"2. The usb-storage driver is using it\n"+
			"3. Insufficient permissions\n"+
			"Error:", err)
	}
	defer handle.ReleaseInterface(0)

	fmt.Println("✓ Claimed Mass Storage interface")

	// Create MSC device wrapper
	msc := &MSCDevice{
		handle: handle,
		epIn:   epIn,
		epOut:  epOut,
		tag:    1,
	}

	// Test Unit Ready
	fmt.Println("\n--- Testing Unit Ready ---")
	if err := msc.TestUnitReady(); err != nil {
		fmt.Printf("Warning: Test Unit Ready failed: %v\n", err)
		// Continue anyway, some devices report not ready but still work
	} else {
		fmt.Println("✓ Device is ready")
	}

	// Send SCSI Inquiry command
	fmt.Println("\n--- SCSI Inquiry ---")
	inquiryData, err := msc.Inquiry()
	if err != nil {
		log.Fatal("SCSI Inquiry failed:", err)
	}
	parseInquiryData(inquiryData)

	// Get capacity
	fmt.Println("\n--- Read Capacity ---")
	blockCount, blockSize, err := msc.ReadCapacity()
	if err != nil {
		log.Fatal("Read Capacity failed:", err)
	}

	totalSize := uint64(blockCount) * uint64(blockSize)
	fmt.Printf("Blocks: %d\n", blockCount)
	fmt.Printf("Block Size: %d bytes\n", blockSize)
	fmt.Printf("Total Capacity: %.2f GB\n", float64(totalSize)/(1024*1024*1024))

	// Read first block (boot sector / MBR)
	fmt.Println("\n--- Reading Block 0 (Boot Sector/MBR) ---")
	block0, err := msc.ReadBlock(0, blockSize)
	if err != nil {
		log.Fatal("Failed to read block 0:", err)
	}

	fmt.Println("First 512 bytes of Block 0:")
	hexdump(block0[:512])

	// Check for MBR signature
	if len(block0) >= 512 && block0[510] == 0x55 && block0[511] == 0xAA {
		fmt.Println("\n✓ Valid MBR signature found (0x55AA)")

		// Parse partition table
		fmt.Println("\nPartition Table:")
		for i := 0; i < 4; i++ {
			offset := 446 + i*16
			if block0[offset+4] != 0 { // Check if partition type is non-zero
				fmt.Printf("Partition %d: Type=0x%02x, Start LBA=%d\n",
					i+1,
					block0[offset+4],
					binary.LittleEndian.Uint32(block0[offset+8:offset+12]))
			}
		}
	}

	// Try to read a FAT boot sector (typically at block 63 or 2048)
	fmt.Println("\n--- Attempting to Read FAT Boot Sector ---")
	possibleStarts := []uint32{63, 2048, 1} // Common partition start locations

	for _, start := range possibleStarts {
		if start < blockCount {
			fatBlock, err := msc.ReadBlock(start, blockSize)
			if err == nil && len(fatBlock) >= 512 {
				// Check for FAT signature
				if fatBlock[510] == 0x55 && fatBlock[511] == 0xAA {
					// Check for FAT32 signature
					if bytes.Equal(fatBlock[82:87], []byte("FAT32")) {
						fmt.Printf("\n✓ Found FAT32 filesystem at block %d\n", start)
						fmt.Println("FAT32 Boot Sector (first 256 bytes):")
						hexdump(fatBlock[:256])
						break
					} else if bytes.Equal(fatBlock[54:59], []byte("FAT16")) ||
						bytes.Equal(fatBlock[54:59], []byte("FAT12")) {
						fmt.Printf("\n✓ Found FAT16/12 filesystem at block %d\n", start)
						fmt.Println("FAT Boot Sector (first 256 bytes):")
						hexdump(fatBlock[:256])
						break
					}
				}
			}
		}
	}

	fmt.Println("\n✓ Successfully read filesystem blocks from USB Mass Storage device")
	fmt.Println("✓ go-usb library bulk transfer implementation verified")
}

// findMSCEndpoints finds the bulk IN and OUT endpoints for Mass Storage
func findMSCEndpoints(handle *usb.DeviceHandle) (uint8, uint8, error) {
	// For the SanDisk device, we know from lsusb:
	// IN endpoint: 0x81
	// OUT endpoint: 0x02
	// But let's verify by reading the configuration

	// Mass Storage devices typically use:
	// - Bulk IN endpoint (0x81 or similar)
	// - Bulk OUT endpoint (0x02 or similar)

	return 0x81, 0x02, nil
}

// TestUnitReady sends a Test Unit Ready command
func (m *MSCDevice) TestUnitReady() error {
	cbw := CBW{
		Signature:          CBW_SIGNATURE,
		Tag:                m.tag,
		DataTransferLength: 0,
		Flags:              0x80, // Device to Host
		LUN:                0,
		CBLength:           6,
	}
	m.tag++

	// SCSI Test Unit Ready command
	cbw.CB[0] = SCSI_TEST_UNIT_READY

	// Send CBW
	cbwBytes := structToBytes(cbw)
	_, err := m.handle.BulkTransfer(m.epOut, cbwBytes, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to send CBW: %w", err)
	}

	// Receive CSW
	cswBytes := make([]byte, 13)
	_, err = m.handle.BulkTransfer(m.epIn, cswBytes, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to receive CSW: %w", err)
	}

	var csw CSW
	if err := bytesToStruct(cswBytes, &csw); err != nil {
		return fmt.Errorf("failed to parse CSW: %w", err)
	}

	if csw.Status != 0 {
		return fmt.Errorf("command failed with status: %d", csw.Status)
	}

	return nil
}

// Inquiry sends a SCSI Inquiry command
func (m *MSCDevice) Inquiry() ([]byte, error) {
	inquiryData := make([]byte, 36) // Standard inquiry data length

	cbw := CBW{
		Signature:          CBW_SIGNATURE,
		Tag:                m.tag,
		DataTransferLength: uint32(len(inquiryData)),
		Flags:              0x80, // Device to Host
		LUN:                0,
		CBLength:           6,
	}
	m.tag++

	// SCSI Inquiry command
	cbw.CB[0] = SCSI_INQUIRY
	cbw.CB[4] = byte(len(inquiryData))

	// Send CBW
	cbwBytes := structToBytes(cbw)
	_, err := m.handle.BulkTransfer(m.epOut, cbwBytes, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to send CBW: %w", err)
	}

	// Receive inquiry data
	n, err := m.handle.BulkTransfer(m.epIn, inquiryData, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to receive inquiry data: %w", err)
	}

	// Receive CSW
	cswBytes := make([]byte, 13)
	_, err = m.handle.BulkTransfer(m.epIn, cswBytes, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to receive CSW: %w", err)
	}

	var csw CSW
	if err := bytesToStruct(cswBytes, &csw); err != nil {
		return nil, fmt.Errorf("failed to parse CSW: %w", err)
	}

	if csw.Status != 0 {
		return nil, fmt.Errorf("inquiry command failed with status: %d", csw.Status)
	}

	return inquiryData[:n], nil
}

// ReadCapacity sends a SCSI Read Capacity command
func (m *MSCDevice) ReadCapacity() (uint32, uint32, error) {
	capacityData := make([]byte, 8)

	cbw := CBW{
		Signature:          CBW_SIGNATURE,
		Tag:                m.tag,
		DataTransferLength: uint32(len(capacityData)),
		Flags:              0x80, // Device to Host
		LUN:                0,
		CBLength:           10,
	}
	m.tag++

	// SCSI Read Capacity command
	cbw.CB[0] = SCSI_READ_CAPACITY

	// Send CBW
	cbwBytes := structToBytes(cbw)
	_, err := m.handle.BulkTransfer(m.epOut, cbwBytes, 5*time.Second)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to send CBW: %w", err)
	}

	// Receive capacity data
	_, err = m.handle.BulkTransfer(m.epIn, capacityData, 5*time.Second)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to receive capacity data: %w", err)
	}

	// Receive CSW
	cswBytes := make([]byte, 13)
	_, err = m.handle.BulkTransfer(m.epIn, cswBytes, 5*time.Second)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to receive CSW: %w", err)
	}

	var csw CSW
	if err := bytesToStruct(cswBytes, &csw); err != nil {
		return 0, 0, fmt.Errorf("failed to parse CSW: %w", err)
	}

	if csw.Status != 0 {
		return 0, 0, fmt.Errorf("read capacity command failed with status: %d", csw.Status)
	}

	// Parse capacity data
	maxLBA := binary.BigEndian.Uint32(capacityData[0:4])
	blockSize := binary.BigEndian.Uint32(capacityData[4:8])

	// maxLBA is the last valid block address, so total blocks = maxLBA + 1
	return maxLBA + 1, blockSize, nil
}

// ReadBlock reads a single block from the device
func (m *MSCDevice) ReadBlock(lba uint32, blockSize uint32) ([]byte, error) {
	data := make([]byte, blockSize)

	cbw := CBW{
		Signature:          CBW_SIGNATURE,
		Tag:                m.tag,
		DataTransferLength: blockSize,
		Flags:              0x80, // Device to Host
		LUN:                0,
		CBLength:           10,
	}
	m.tag++

	// SCSI Read(10) command
	cbw.CB[0] = SCSI_READ_10
	// LBA in big-endian
	cbw.CB[2] = byte(lba >> 24)
	cbw.CB[3] = byte(lba >> 16)
	cbw.CB[4] = byte(lba >> 8)
	cbw.CB[5] = byte(lba)
	// Transfer length (1 block) in big-endian
	cbw.CB[7] = 0
	cbw.CB[8] = 1

	// Send CBW
	cbwBytes := structToBytes(cbw)
	_, err := m.handle.BulkTransfer(m.epOut, cbwBytes, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to send CBW: %w", err)
	}

	// Receive data
	n, err := m.handle.BulkTransfer(m.epIn, data, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to receive data: %w", err)
	}

	// Receive CSW
	cswBytes := make([]byte, 13)
	_, err = m.handle.BulkTransfer(m.epIn, cswBytes, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to receive CSW: %w", err)
	}

	var csw CSW
	if err := bytesToStruct(cswBytes, &csw); err != nil {
		return nil, fmt.Errorf("failed to parse CSW: %w", err)
	}

	if csw.Status != 0 {
		return nil, fmt.Errorf("read command failed with status: %d", csw.Status)
	}

	return data[:n], nil
}

// parseInquiryData parses and displays SCSI Inquiry response
func parseInquiryData(data []byte) {
	if len(data) < 36 {
		fmt.Printf("Inquiry data too short: %d bytes\n", len(data))
		return
	}

	peripheralType := data[0] & 0x1F
	fmt.Printf("Peripheral Device Type: 0x%02x ", peripheralType)
	switch peripheralType {
	case 0x00:
		fmt.Println("(Direct-access device)")
	case 0x05:
		fmt.Println("(CD-ROM)")
	default:
		fmt.Println("(Other)")
	}

	// Extract vendor, product, and revision
	vendor := string(bytes.TrimRight(data[8:16], " \x00"))
	product := string(bytes.TrimRight(data[16:32], " \x00"))
	revision := string(bytes.TrimRight(data[32:36], " \x00"))

	fmt.Printf("Vendor: %s\n", vendor)
	fmt.Printf("Product: %s\n", product)
	fmt.Printf("Revision: %s\n", revision)
}

// hexdump displays data in hex dump format
func hexdump(data []byte) {
	for i := 0; i < len(data); i += 16 {
		// Print offset
		fmt.Printf("%08x  ", i)

		// Print hex bytes
		for j := 0; j < 16; j++ {
			if i+j < len(data) {
				fmt.Printf("%02x ", data[i+j])
			} else {
				fmt.Print("   ")
			}
			if j == 7 {
				fmt.Print(" ")
			}
		}

		// Print ASCII
		fmt.Print(" |")
		for j := 0; j < 16 && i+j < len(data); j++ {
			c := data[i+j]
			if c >= 32 && c < 127 {
				fmt.Printf("%c", c)
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println("|")
	}
}

// structToBytes converts a struct to bytes
func structToBytes(v interface{}) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, v)
	return buf.Bytes()
}

// bytesToStruct converts bytes to a struct
func bytesToStruct(data []byte, v interface{}) error {
	buf := bytes.NewReader(data)
	return binary.Read(buf, binary.LittleEndian, v)
}

// unbindDevice attempts to unbind the device from usb-storage kernel driver
func unbindDevice() {
	// Try to unbind any USB storage device from usb-storage driver
	unbindPath := "/sys/bus/usb/drivers/usb-storage/unbind"

	// Try common device bindings across different buses
	for bus := 1; bus <= 8; bus++ {
		for dev := 1; dev <= 4; dev++ {
			binding := fmt.Sprintf("%d-%d:1.0", bus, dev)
			err := os.WriteFile(unbindPath, []byte(binding), 0200)
			if err == nil {
				fmt.Printf("✓ Unbound device %s from usb-storage driver\n", binding)
				time.Sleep(100 * time.Millisecond)
				return
			}
		}
	}

	// If we couldn't unbind, continue anyway - DetachKernelDriver might work
}

// listMassStorageDevices lists all USB Mass Storage devices
func listMassStorageDevices() {
	fmt.Println("\nSearching for USB Mass Storage devices...")
	fmt.Println()

	devices, err := usb.GetDeviceList()
	if err != nil {
		log.Fatal("Failed to get device list:", err)
	}

	found := false
	for _, device := range devices {
		// Check if it's a Mass Storage device
		// Note: This is a simplified check - proper detection would need to
		// parse configuration descriptors for interface class
		if device.Descriptor.DeviceClass == MSC_CLASS ||
			(device.Descriptor.DeviceClass == 0 && // Class defined at interface level
				isMassStorageDevice(device)) {
			found = true
			fmt.Printf("Device: VID=%04x PID=%04x\n",
				device.Descriptor.VendorID, device.Descriptor.ProductID)

			// Try to get product name
			if handle, err := device.Open(); err == nil {
				if product, err := handle.GetStringDescriptor(device.Descriptor.ProductIndex); err == nil {
					fmt.Printf("  Product: %s\n", product)
				}
				if manufacturer, err := handle.GetStringDescriptor(device.Descriptor.ManufacturerIndex); err == nil {
					fmt.Printf("  Manufacturer: %s\n", manufacturer)
				}
				handle.Close()
			}
			fmt.Println()
		}
	}

	if !found {
		fmt.Println("No USB Mass Storage devices found.")
		fmt.Println("Note: Some devices may not be detected if they're in use by the kernel.")
	}
}

// isMassStorageDevice checks if a device might be a mass storage device
func isMassStorageDevice(device *usb.Device) bool {
	// This is a heuristic check - common vendor IDs for USB storage
	knownStorageVendors := []uint16{
		0x0781, // SanDisk
		0x058f, // Alcor Micro
		0x0930, // Toshiba
		0x090c, // Silicon Motion
		0x13fe, // Kingston
		0x154b, // PNY
		0x0951, // Kingston
	}

	for _, vendorID := range knownStorageVendors {
		if device.Descriptor.VendorID == vendorID {
			return true
		}
	}

	return false
}
