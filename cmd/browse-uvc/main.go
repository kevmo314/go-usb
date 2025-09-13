package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	usb "github.com/kevmo314/go-usb"
)

// UVC Class codes
const (
	CC_VIDEO = 0x0E

	// Video Interface Subclass Codes
	SC_UNDEFINED                  = 0x00
	SC_VIDEOCONTROL               = 0x01
	SC_VIDEOSTREAMING             = 0x02
	SC_VIDEO_INTERFACE_COLLECTION = 0x03

	// Video Class-Specific Descriptor Types
	CS_UNDEFINED     = 0x20
	CS_DEVICE        = 0x21
	CS_CONFIGURATION = 0x22
	CS_STRING        = 0x23
	CS_INTERFACE     = 0x24
	CS_ENDPOINT      = 0x25

	// Video Class-Specific VC Interface Descriptor Subtypes
	VC_DESCRIPTOR_UNDEFINED = 0x00
	VC_HEADER               = 0x01
	VC_INPUT_TERMINAL       = 0x02
	VC_OUTPUT_TERMINAL      = 0x03
	VC_SELECTOR_UNIT        = 0x04
	VC_PROCESSING_UNIT      = 0x05
	VC_EXTENSION_UNIT       = 0x06

	// Video Class-Specific VS Interface Descriptor Subtypes
	VS_UNDEFINED           = 0x00
	VS_INPUT_HEADER        = 0x01
	VS_OUTPUT_HEADER       = 0x02
	VS_STILL_IMAGE_FRAME   = 0x03
	VS_FORMAT_UNCOMPRESSED = 0x04
	VS_FRAME_UNCOMPRESSED  = 0x05
	VS_FORMAT_MJPEG        = 0x06
	VS_FRAME_MJPEG         = 0x07
	VS_FORMAT_MPEG2TS      = 0x0A
	VS_FORMAT_DV           = 0x0C
	VS_COLORFORMAT         = 0x0D
	VS_FORMAT_FRAME_BASED  = 0x10
	VS_FRAME_FRAME_BASED   = 0x11
	VS_FORMAT_STREAM_BASED = 0x12

	// Terminal Types
	TT_VENDOR_SPECIFIC        = 0x0100
	TT_STREAMING              = 0x0101
	ITT_VENDOR_SPECIFIC       = 0x0200
	ITT_CAMERA                = 0x0201
	ITT_MEDIA_TRANSPORT_INPUT = 0x0202

	// USB Video Class Control Selectors
	VC_CONTROL_UNDEFINED          = 0x00
	VC_VIDEO_POWER_MODE_CONTROL   = 0x01
	VC_REQUEST_ERROR_CODE_CONTROL = 0x02

	// Camera Terminal Control Selectors
	CT_CONTROL_UNDEFINED              = 0x00
	CT_SCANNING_MODE_CONTROL          = 0x01
	CT_AE_MODE_CONTROL                = 0x02
	CT_AE_PRIORITY_CONTROL            = 0x03
	CT_EXPOSURE_TIME_ABSOLUTE_CONTROL = 0x04
	CT_EXPOSURE_TIME_RELATIVE_CONTROL = 0x05
	CT_FOCUS_ABSOLUTE_CONTROL         = 0x06
	CT_FOCUS_RELATIVE_CONTROL         = 0x07
	CT_FOCUS_AUTO_CONTROL             = 0x08
	CT_IRIS_ABSOLUTE_CONTROL          = 0x09
	CT_IRIS_RELATIVE_CONTROL          = 0x0A
	CT_ZOOM_ABSOLUTE_CONTROL          = 0x0B
	CT_ZOOM_RELATIVE_CONTROL          = 0x0C
	CT_PANTILT_ABSOLUTE_CONTROL       = 0x0D
	CT_PANTILT_RELATIVE_CONTROL       = 0x0E
	CT_ROLL_ABSOLUTE_CONTROL          = 0x0F
	CT_ROLL_RELATIVE_CONTROL          = 0x10

	// Processing Unit Control Selectors
	PU_CONTROL_UNDEFINED                      = 0x00
	PU_BACKLIGHT_COMPENSATION_CONTROL         = 0x01
	PU_BRIGHTNESS_CONTROL                     = 0x02
	PU_CONTRAST_CONTROL                       = 0x03
	PU_GAIN_CONTROL                           = 0x04
	PU_POWER_LINE_FREQUENCY_CONTROL           = 0x05
	PU_HUE_CONTROL                            = 0x06
	PU_SATURATION_CONTROL                     = 0x07
	PU_SHARPNESS_CONTROL                      = 0x08
	PU_GAMMA_CONTROL                          = 0x09
	PU_WHITE_BALANCE_TEMPERATURE_CONTROL      = 0x0A
	PU_WHITE_BALANCE_TEMPERATURE_AUTO_CONTROL = 0x0B
	PU_WHITE_BALANCE_COMPONENT_CONTROL        = 0x0C
	PU_WHITE_BALANCE_COMPONENT_AUTO_CONTROL   = 0x0D
	PU_DIGITAL_MULTIPLIER_CONTROL             = 0x0E
	PU_DIGITAL_MULTIPLIER_LIMIT_CONTROL       = 0x0F
	PU_HUE_AUTO_CONTROL                       = 0x10
	PU_ANALOG_VIDEO_STANDARD_CONTROL          = 0x11
	PU_ANALOG_LOCK_STATUS_CONTROL             = 0x12

	// Request codes
	RC_UNDEFINED = 0x00
	SET_CUR      = 0x01
	GET_CUR      = 0x81
	GET_MIN      = 0x82
	GET_MAX      = 0x83
	GET_RES      = 0x84
	GET_LEN      = 0x85
	GET_INFO     = 0x86
	GET_DEF      = 0x87
)

// UVC Control capabilities bits (from GET_INFO)
const (
	UVC_CONTROL_CAP_GET      = 0x01
	UVC_CONTROL_CAP_SET      = 0x02
	UVC_CONTROL_CAP_DISABLED = 0x04
	UVC_CONTROL_CAP_AUTO     = 0x08
	UVC_CONTROL_CAP_ASYNC    = 0x10
)

// VideoControlHeader represents the VC Interface Header Descriptor
type VideoControlHeader struct {
	Length            uint8
	DescriptorType    uint8
	DescriptorSubtype uint8
	BcdUVC            uint16
	TotalLength       uint16
	ClockFrequency    uint32
	InCollection      uint8
	// Followed by array of interface numbers
}

// InputTerminal represents the Input Terminal Descriptor
type InputTerminal struct {
	Length            uint8
	DescriptorType    uint8
	DescriptorSubtype uint8
	TerminalID        uint8
	TerminalType      uint16
	AssocTerminal     uint8
	Terminal          uint8
	// Camera-specific fields follow for Camera Terminal
}

// CameraTerminal extends InputTerminal for Camera terminals
type CameraTerminal struct {
	InputTerminal
	ObjectiveFocalLengthMin uint16
	ObjectiveFocalLengthMax uint16
	OcularFocalLength       uint16
	ControlSize             uint8
	// Controls bitmap follows
}

// ProcessingUnit represents the Processing Unit Descriptor
type ProcessingUnit struct {
	Length            uint8
	DescriptorType    uint8
	DescriptorSubtype uint8
	UnitID            uint8
	SourceID          uint8
	MaxMultiplier     uint16
	ControlSize       uint8
	// Controls bitmap follows
	// Processing field follows
}

// VideoStreamingHeader represents the VS Interface Input Header Descriptor
type VideoStreamingHeader struct {
	Length             uint8
	DescriptorType     uint8
	DescriptorSubtype  uint8
	NumFormats         uint8
	TotalLength        uint16
	EndpointAddress    uint8
	Info               uint8
	TerminalLink       uint8
	StillCaptureMethod uint8
	TriggerSupport     uint8
	TriggerUsage       uint8
	ControlSize        uint8
	// Controls follow
}

// FormatDescriptor represents format descriptors (uncompressed/MJPEG)
type FormatDescriptor struct {
	Length              uint8
	DescriptorType      uint8
	DescriptorSubtype   uint8
	FormatIndex         uint8
	NumFrameDescriptors uint8
	// Format-specific fields follow
}

// FrameDescriptor represents frame descriptors
type FrameDescriptor struct {
	Length                  uint8
	DescriptorType          uint8
	DescriptorSubtype       uint8
	FrameIndex              uint8
	Capabilities            uint8
	Width                   uint16
	Height                  uint16
	MinBitRate              uint32
	MaxBitRate              uint32
	MaxVideoFrameBufferSize uint32
	DefaultFrameInterval    uint32
	FrameIntervalType       uint8
	// Frame intervals follow
}

type UVCDevice struct {
	handle             *usb.DeviceHandle
	device             *usb.Device
	controlInterface   uint8
	streamingInterface uint8
	inputTerminalID    uint8
	processingUnitID   uint8
	outputTerminalID   uint8
}

func main() {
	// Parse command-line flags
	var (
		vendorID    = flag.String("vid", "", "USB Vendor ID in hex (e.g., 046d for Logitech)")
		productID   = flag.String("pid", "", "USB Product ID in hex (e.g., 08e5 for C920)")
		listDevices = flag.Bool("list", false, "List all UVC video devices")
		autoDetect  = flag.Bool("auto", false, "Auto-detect any UVC webcam")
	)
	flag.Parse()

	fmt.Println("USB Video Class (UVC) Browser")
	fmt.Println("==============================")

	// If list flag is set, show all UVC devices
	if *listDevices {
		listUVCDevices()
		return
	}

	var handle *usb.DeviceHandle
	var device *usb.Device

	// Determine which device to open
	if *vendorID != "" && *productID != "" {
		// Use specified VID/PID
		var vid, pid uint16
		if _, err := fmt.Sscanf(*vendorID, "%x", &vid); err != nil {
			log.Fatalf("Invalid vendor ID format: %s (should be hex, e.g., 046d)", *vendorID)
		}
		if _, err := fmt.Sscanf(*productID, "%x", &pid); err != nil {
			log.Fatalf("Invalid product ID format: %s (should be hex, e.g., 08e5)", *productID)
		}

		fmt.Printf("Looking for device VID:PID = %04x:%04x\n", vid, pid)

		// Try to unbind from uvcvideo driver
		unbindUVCDriver()

		var err error
		handle, err = usb.OpenDevice(vid, pid)
		if err != nil {
			log.Fatalf("Failed to open device %04x:%04x: %v\n", vid, pid, err)
		}

		// Find device in list
		devices, _ := usb.GetDeviceList()
		for _, d := range devices {
			if d.Descriptor.VendorID == vid && d.Descriptor.ProductID == pid {
				device = d
				break
			}
		}
	} else if *autoDetect {
		// Auto-detect any webcam
		fmt.Println("Auto-detecting UVC webcam...")

		// Try to unbind from uvcvideo driver
		unbindUVCDriver()

		var err error
		handle, device, err = findAnyWebcamWithDevice()
		if err != nil {
			log.Fatal("No UVC webcam found. Make sure a webcam is connected.")
		}
	} else {
		// Default: try Logitech C920
		fmt.Println("No device specified, trying Logitech C920 (046d:08e5)...")
		fmt.Println("Use -list to see available devices, or -vid/-pid to specify a device")

		// Try to unbind from uvcvideo driver
		unbindUVCDriver()

		var err error
		handle, err = usb.OpenDevice(0x046d, 0x08e5)
		if err != nil {
			fmt.Println("Logitech C920 not found, searching for other webcams...")
			var err error
			handle, device, err = findAnyWebcamWithDevice()
			if err != nil {
				log.Fatal("No UVC webcam found. Use -list to see available devices.")
			}
		} else {
			// Find device in list
			devices, _ := usb.GetDeviceList()
			for _, d := range devices {
				if d.Descriptor.VendorID == 0x046d && d.Descriptor.ProductID == 0x08e5 {
					device = d
					break
				}
			}
		}
	}

	defer handle.Close()
	fmt.Println("✓ Found and opened UVC webcam")

	// Ensure we have a device reference
	if device == nil {
		log.Fatal("Could not find device in list")
	}

	// Display basic device info
	fmt.Printf("\nDevice Information:\n")
	fmt.Printf("  Vendor ID:  0x%04x\n", device.Descriptor.VendorID)
	fmt.Printf("  Product ID: 0x%04x\n", device.Descriptor.ProductID)

	if product, err := handle.GetStringDescriptor(device.Descriptor.ProductIndex); err == nil {
		fmt.Printf("  Product:    %s\n", product)
	}

	if serial, err := handle.GetStringDescriptor(device.Descriptor.SerialNumberIndex); err == nil {
		fmt.Printf("  Serial:     %s\n", serial)
	}

	// Create UVC device wrapper
	uvc := &UVCDevice{
		handle: handle,
		device: device,
	}

	// Get and parse configuration descriptor to find UVC interfaces
	fmt.Println("\n--- Analyzing UVC Descriptors ---")
	if err := uvc.parseDescriptors(); err != nil {
		log.Printf("Warning: Failed to parse descriptors: %v", err)
	}

	// Try to detach kernel driver first
	fmt.Printf("Attempting to detach kernel driver from interface %d...\n", uvc.controlInterface)
	if err := handle.DetachKernelDriver(uvc.controlInterface); err != nil {
		fmt.Printf("Note: Kernel driver detach result: %v\n", err)
	} else {
		fmt.Println("✓ Detached kernel driver")
		defer func() {
			// Re-attach kernel driver when done
			handle.AttachKernelDriver(uvc.controlInterface)
		}()
	}

	// Try to claim the control interface
	if err := handle.ClaimInterface(uvc.controlInterface); err != nil {
		fmt.Printf("Warning: Could not claim control interface: %v\n", err)
		fmt.Println("Some controls may not be accessible.")
	} else {
		fmt.Println("✓ Claimed control interface")
		defer handle.ReleaseInterface(uvc.controlInterface)
	}

	// Query camera controls
	fmt.Println("\n--- Camera Controls ---")
	uvc.queryControls()

	// Display supported formats
	fmt.Println("\n--- Supported Video Formats ---")
	uvc.displayFormats()

	fmt.Println("\n✓ UVC device information retrieved successfully")
}

// findAnyWebcamWithDevice searches for any UVC device and returns handle and device
func findAnyWebcamWithDevice() (*usb.DeviceHandle, *usb.Device, error) {
	devices, err := usb.GetDeviceList()
	if err != nil {
		return nil, nil, err
	}

	for _, device := range devices {
		if isWebcam(device) {
			handle, err := device.Open()
			if err == nil {
				fmt.Printf("Found webcam: VID=0x%04x PID=0x%04x\n",
					device.Descriptor.VendorID, device.Descriptor.ProductID)
				return handle, device, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("no UVC webcam found")
}

// isWebcam checks if a device might be a webcam
func isWebcam(device *usb.Device) bool {
	// Check if device class is Video (0x0E) or Miscellaneous (0xEF) with IAD
	if device.Descriptor.DeviceClass == CC_VIDEO {
		return true
	}
	if device.Descriptor.DeviceClass == 0xEF &&
		device.Descriptor.DeviceSubClass == 0x02 &&
		device.Descriptor.DeviceProtocol == 0x01 {
		// Miscellaneous device with Interface Association
		return true
	}
	return false
}

// parseDescriptors parses UVC-specific descriptors
func (u *UVCDevice) parseDescriptors() error {
	// Get configuration descriptor
	config := make([]byte, 4096)
	n, err := u.handle.ControlTransfer(
		0x80,   // Device to host
		0x06,   // GET_DESCRIPTOR
		0x0200, // Configuration descriptor
		0,
		config,
		5*time.Second,
	)
	if err != nil {
		return fmt.Errorf("failed to get configuration descriptor: %w", err)
	}

	config = config[:n]

	// Parse configuration to find Video Control and Streaming interfaces
	offset := 0
	for offset < len(config) {
		if offset+2 > len(config) {
			break
		}

		length := int(config[offset])
		descType := config[offset+1]

		if length == 0 || offset+length > len(config) {
			break
		}

		// Check for Interface Association Descriptor
		if descType == 0x0B && length >= 8 {
			functionClass := config[offset+4]
			if functionClass == CC_VIDEO {
				fmt.Println("✓ Found Video Interface Collection")
				u.controlInterface = config[offset+2] // First interface
				u.streamingInterface = config[offset+2] + 1
			}
		}

		// Check for Interface Descriptor
		if descType == 0x04 && length >= 9 {
			interfaceClass := config[offset+5]
			interfaceSubclass := config[offset+6]

			if interfaceClass == CC_VIDEO {
				if interfaceSubclass == SC_VIDEOCONTROL {
					u.controlInterface = config[offset+2]
					fmt.Printf("✓ Found Video Control Interface: %d\n", u.controlInterface)
				} else if interfaceSubclass == SC_VIDEOSTREAMING {
					u.streamingInterface = config[offset+2]
					fmt.Printf("✓ Found Video Streaming Interface: %d\n", u.streamingInterface)
				}
			}
		}

		// Check for Class-specific VC Interface descriptors
		if descType == CS_INTERFACE && offset+3 < len(config) {
			subtype := config[offset+2]

			switch subtype {
			case VC_INPUT_TERMINAL:
				if length >= 8 {
					u.inputTerminalID = config[offset+3]
					terminalType := binary.LittleEndian.Uint16(config[offset+4 : offset+6])
					fmt.Printf("  Input Terminal ID=%d, Type=0x%04x", u.inputTerminalID, terminalType)
					if terminalType == ITT_CAMERA {
						fmt.Print(" (Camera)")
					}
					fmt.Println()
				}

			case VC_PROCESSING_UNIT:
				if length >= 8 && u.processingUnitID == 0 { // Only set first one
					u.processingUnitID = config[offset+3]
					sourceID := config[offset+4]
					fmt.Printf("  Processing Unit ID=%d, Source=%d\n", u.processingUnitID, sourceID)
				}

			case VC_OUTPUT_TERMINAL:
				if length >= 9 {
					u.outputTerminalID = config[offset+3]
					fmt.Printf("  Output Terminal ID=%d\n", u.outputTerminalID)
				}
			}
		}

		offset += length
	}

	return nil
}

// queryControls queries various camera controls
func (u *UVCDevice) queryControls() {
	// Processing Unit controls
	if u.processingUnitID != 0 {
		fmt.Println("\nProcessing Unit Controls:")

		// Brightness
		u.queryControl("Brightness", u.processingUnitID, PU_BRIGHTNESS_CONTROL, 2, true)

		// Contrast
		u.queryControl("Contrast", u.processingUnitID, PU_CONTRAST_CONTROL, 2, true)

		// Saturation
		u.queryControl("Saturation", u.processingUnitID, PU_SATURATION_CONTROL, 2, true)

		// Sharpness
		u.queryControl("Sharpness", u.processingUnitID, PU_SHARPNESS_CONTROL, 2, true)

		// White Balance Temperature
		u.queryControl("White Balance Temp", u.processingUnitID, PU_WHITE_BALANCE_TEMPERATURE_CONTROL, 2, true)

		// Gain
		u.queryControl("Gain", u.processingUnitID, PU_GAIN_CONTROL, 2, true)
	}

	// Camera Terminal controls
	if u.inputTerminalID != 0 {
		fmt.Println("\nCamera Terminal Controls:")

		// Auto Exposure Mode
		u.queryControl("Auto-Exposure Mode", u.inputTerminalID, CT_AE_MODE_CONTROL, 1, false)

		// Exposure Time
		u.queryControl("Exposure Time", u.inputTerminalID, CT_EXPOSURE_TIME_ABSOLUTE_CONTROL, 4, true)

		// Focus
		u.queryControl("Focus (Absolute)", u.inputTerminalID, CT_FOCUS_ABSOLUTE_CONTROL, 2, true)

		// Auto Focus
		u.queryControl("Auto-Focus", u.inputTerminalID, CT_FOCUS_AUTO_CONTROL, 1, false)

		// Zoom
		u.queryControl("Zoom", u.inputTerminalID, CT_ZOOM_ABSOLUTE_CONTROL, 2, true)
	}
}

// queryControl queries a specific control's current, min, max, and default values
func (u *UVCDevice) queryControl(name string, unitID uint8, controlSelector uint8, size int, showRange bool) {
	// First check if control is supported
	info := make([]byte, 1)
	err := u.getControl(unitID, controlSelector, GET_INFO, info)
	if err != nil {
		fmt.Printf("  %s: Not supported\n", name)
		return
	}

	capabilities := info[0]
	if capabilities&UVC_CONTROL_CAP_GET == 0 {
		fmt.Printf("  %s: Not readable\n", name)
		return
	}

	// Get current value
	current := make([]byte, size)
	err = u.getControl(unitID, controlSelector, GET_CUR, current)
	if err != nil {
		fmt.Printf("  %s: Error reading current value: %v\n", name, err)
		return
	}

	fmt.Printf("  %s: ", name)

	// Display based on size
	var curVal int32
	switch size {
	case 1:
		curVal = int32(current[0])
		fmt.Printf("Current=%d", current[0])
	case 2:
		curVal = int32(binary.LittleEndian.Uint16(current))
		fmt.Printf("Current=%d", curVal)
	case 4:
		curVal = int32(binary.LittleEndian.Uint32(current))
		fmt.Printf("Current=%d", curVal)
	}

	if showRange {
		// Get min value
		min := make([]byte, size)
		if err := u.getControl(unitID, controlSelector, GET_MIN, min); err == nil {
			switch size {
			case 1:
				fmt.Printf(", Min=%d", min[0])
			case 2:
				fmt.Printf(", Min=%d", binary.LittleEndian.Uint16(min))
			case 4:
				fmt.Printf(", Min=%d", binary.LittleEndian.Uint32(min))
			}
		}

		// Get max value
		max := make([]byte, size)
		if err := u.getControl(unitID, controlSelector, GET_MAX, max); err == nil {
			switch size {
			case 1:
				fmt.Printf(", Max=%d", max[0])
			case 2:
				fmt.Printf(", Max=%d", binary.LittleEndian.Uint16(max))
			case 4:
				fmt.Printf(", Max=%d", binary.LittleEndian.Uint32(max))
			}
		}

		// Get default value
		def := make([]byte, size)
		if err := u.getControl(unitID, controlSelector, GET_DEF, def); err == nil {
			switch size {
			case 1:
				fmt.Printf(", Default=%d", def[0])
			case 2:
				fmt.Printf(", Default=%d", binary.LittleEndian.Uint16(def))
			case 4:
				fmt.Printf(", Default=%d", binary.LittleEndian.Uint32(def))
			}
		}
	}

	// Show capabilities
	if capabilities&UVC_CONTROL_CAP_SET != 0 {
		fmt.Print(" [Read/Write]")
	} else {
		fmt.Print(" [Read-only]")
	}

	if capabilities&UVC_CONTROL_CAP_AUTO != 0 {
		fmt.Print(" [Auto]")
	}

	fmt.Println()
}

// getControl performs a UVC GET control request
func (u *UVCDevice) getControl(unitID uint8, controlSelector uint8, request uint8, data []byte) error {
	// UVC control requests are sent to the control interface
	// wValue: Control Selector << 8
	// wIndex: Unit ID << 8 | Interface Number

	wValue := uint16(controlSelector) << 8
	wIndex := uint16(unitID)<<8 | uint16(u.controlInterface)

	_, err := u.handle.ControlTransfer(
		0xA1, // Class-specific request, interface target, device-to-host
		request,
		wValue,
		wIndex,
		data,
		time.Second,
	)

	return err
}

// displayFormats displays supported video formats
func (u *UVCDevice) displayFormats() {
	// This would require parsing the full configuration descriptor
	// to extract VS_FORMAT and VS_FRAME descriptors
	// For now, we'll show a simplified version

	fmt.Println("\nNote: Full format parsing requires extended descriptor analysis")
	fmt.Println("Common formats for UVC cameras:")
	fmt.Println("  • YUV 4:2:2 (YUYV)")
	fmt.Println("  • Motion-JPEG (MJPEG)")
	fmt.Println("  • H.264 (if supported)")

	fmt.Println("\nTypical resolutions:")
	fmt.Println("  • 640x480 @ 30 fps")
	fmt.Println("  • 1280x720 @ 30 fps")
	fmt.Println("  • 1920x1080 @ 30 fps")

	// Try to get video probe control to see current format
	// This requires the streaming interface which may need different handling
}

// Helper function to parse GUID
func parseGUID(data []byte) string {
	if len(data) < 16 {
		return "Invalid GUID"
	}
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		binary.LittleEndian.Uint32(data[0:4]),
		binary.LittleEndian.Uint16(data[4:6]),
		binary.LittleEndian.Uint16(data[6:8]),
		data[8], data[9],
		data[10], data[11], data[12], data[13], data[14], data[15])
}

// unbindUVCDriver attempts to unbind the device from uvcvideo kernel driver
func unbindUVCDriver() {
	// Common bindings for webcams
	unbindPath := "/sys/bus/usb/drivers/uvcvideo/unbind"
	possibleBindings := []string{"1-8:1.0", "1-8:1.1", "1-7:1.0", "1-7:1.1"}

	for _, binding := range possibleBindings {
		err := os.WriteFile(unbindPath, []byte(binding), 0200)
		if err == nil {
			fmt.Printf("✓ Unbound device %s from uvcvideo driver\n", binding)
			time.Sleep(100 * time.Millisecond)
			return
		}
	}
}

// listUVCDevices lists all UVC video devices
func listUVCDevices() {
	fmt.Println("\nSearching for UVC video devices...")
	fmt.Println()

	devices, err := usb.GetDeviceList()
	if err != nil {
		log.Fatal("Failed to get device list:", err)
	}

	found := false
	for _, device := range devices {
		if isWebcam(device) {
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
				if serial, err := handle.GetStringDescriptor(device.Descriptor.SerialNumberIndex); err == nil {
					fmt.Printf("  Serial: %s\n", serial)
				}
				handle.Close()
			}

			if device.Descriptor.DeviceClass == CC_VIDEO {
				fmt.Println("  Type: USB Video Class device")
			} else if device.Descriptor.DeviceClass == 0xEF {
				fmt.Println("  Type: Composite device with video")
			}

			fmt.Println()
		}
	}

	if !found {
		fmt.Println("No UVC video devices found.")
		fmt.Println("Note: Some devices may not be detected if they're in use by the kernel.")
	}
}
