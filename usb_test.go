package usb

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestNewContext(t *testing.T) {
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	if ctx == nil {
		t.Fatal("Context is nil")
	}
	
	if ctx.devices == nil {
		t.Fatal("Devices slice is nil")
	}
}

func TestSetDebug(t *testing.T) {
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	ctx.SetDebug(true)
	if !ctx.debug {
		t.Error("Debug mode not set")
	}
	
	ctx.SetDebug(false)
	if ctx.debug {
		t.Error("Debug mode not unset")
	}
}

func TestGetVersion(t *testing.T) {
	version := GetVersion()
	if version == "" {
		t.Error("Version string is empty")
	}
	
	expected := "1.0.0"
	if version != expected {
		t.Errorf("Version mismatch: got %s, expected %s", version, expected)
	}
}

func TestGetCapabilities(t *testing.T) {
	caps := GetCapabilities()
	
	if caps == nil {
		t.Fatal("Capabilities map is nil")
	}
	
	expectedCaps := []string{
		"has_capability",
		"has_hid_access",
		"supports_detach_kernel_driver",
	}
	
	for _, cap := range expectedCaps {
		if _, ok := caps[cap]; !ok {
			t.Errorf("Missing capability: %s", cap)
		}
	}
}

func TestIsValidDevicePath(t *testing.T) {
	tests := []struct {
		path  string
		valid bool
	}{
		{"/dev/bus/usb/001/001", true},
		{"/dev/bus/usb/255/255", true},
		{"/dev/bus/usb/001/256", false},
		{"/dev/bus/usb/256/001", false},
		{"/dev/bus/usb/001", false},
		{"/dev/bus/usb/", false},
		{"/dev/bus/001/001", false},
		{"/tmp/001/001", false},
		{"", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsValidDevicePath(tt.path); got != tt.valid {
				t.Errorf("IsValidDevicePath(%q) = %v, want %v", tt.path, got, tt.valid)
			}
		})
	}
}

func TestGetDeviceList(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}
	
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	devices, err := ctx.GetDeviceList()
	if err != nil {
		t.Fatalf("Failed to get device list: %v", err)
	}
	
	if len(devices) == 0 {
		t.Log("No USB devices found (this might be expected in some environments)")
	} else {
		t.Logf("Found %d USB devices", len(devices))
		
		for i, dev := range devices {
			if dev == nil {
				t.Errorf("Device %d is nil", i)
				continue
			}
			
			if dev.Path == "" {
				t.Errorf("Device %d has empty path", i)
			}
			
			if dev.Bus == 0 && dev.Address == 0 {
				t.Errorf("Device %d has invalid bus/address", i)
			}
			
			t.Logf("Device %d: Bus=%03d Address=%03d VID=0x%04x PID=0x%04x Path=%s",
				i, dev.Bus, dev.Address, dev.Descriptor.VendorID, dev.Descriptor.ProductID, dev.Path)
		}
	}
}

func TestOpenDevice(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}
	
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	handle, err := ctx.OpenDevice(0xFFFF, 0xFFFF)
	if err != ErrDeviceNotFound {
		if handle != nil {
			handle.Close()
		}
		t.Errorf("Expected ErrDeviceNotFound for non-existent device, got: %v", err)
	}
	
	devices, err := ctx.GetDeviceList()
	if err != nil || len(devices) == 0 {
		t.Skip("No USB devices available for testing")
	}
	
	firstDevice := devices[0]
	handle, err = ctx.OpenDevice(firstDevice.Descriptor.VendorID, firstDevice.Descriptor.ProductID)
	if err != nil {
		if err == ErrPermissionDenied {
			t.Skip("Permission denied to open USB device")
		}
		t.Errorf("Failed to open device: %v", err)
	} else {
		defer handle.Close()
		
		desc := handle.GetDescriptor()
		if desc.VendorID != firstDevice.Descriptor.VendorID {
			t.Errorf("VendorID mismatch: got 0x%04x, expected 0x%04x",
				desc.VendorID, firstDevice.Descriptor.VendorID)
		}
	}
}

func TestDeviceDescriptor(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}
	
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	devices, err := ctx.GetDeviceList()
	if err != nil || len(devices) == 0 {
		t.Skip("No USB devices available for testing")
	}
	
	for _, dev := range devices {
		desc := dev.Descriptor
		
		if desc.Length == 0 {
			t.Errorf("Device descriptor length is 0")
		}
		
		if desc.DescriptorType != 0x01 {
			t.Errorf("Invalid descriptor type: 0x%02x (expected 0x01)", desc.DescriptorType)
		}
		
		if desc.USBVersion == 0 {
			t.Errorf("USB version is 0")
		}
		
		if desc.MaxPacketSize0 == 0 {
			t.Errorf("MaxPacketSize0 is 0")
		}
		
		if desc.NumConfigurations == 0 {
			t.Errorf("NumConfigurations is 0")
		}
		
		break
	}
}

func TestTransferTypes(t *testing.T) {
	tests := []struct {
		tt       TransferType
		expected uint8
	}{
		{TransferTypeControl, 0},
		{TransferTypeIsochronous, 1},
		{TransferTypeBulk, 2},
		{TransferTypeInterrupt, 3},
	}
	
	for _, test := range tests {
		if uint8(test.tt) != test.expected {
			t.Errorf("TransferType %v != %d", test.tt, test.expected)
		}
	}
}

func TestEndpointDirection(t *testing.T) {
	if uint8(EndpointDirectionOut) != 0 {
		t.Errorf("EndpointDirectionOut should be 0, got %d", EndpointDirectionOut)
	}
	
	if uint8(EndpointDirectionIn) != 0x80 {
		t.Errorf("EndpointDirectionIn should be 0x80, got 0x%02x", EndpointDirectionIn)
	}
}

func TestControlTransfer(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}
	
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	devices, err := ctx.GetDeviceList()
	if err != nil || len(devices) == 0 {
		t.Skip("No USB devices available for testing")
	}
	
	handle, err := devices[0].Open()
	if err != nil {
		if err == ErrPermissionDenied {
			t.Skip("Permission denied to open USB device")
		}
		t.Fatalf("Failed to open device: %v", err)
	}
	defer handle.Close()
	
	buf := make([]byte, 18)
	n, err := handle.ControlTransfer(
		0x80,
		0x06,
		0x0100,
		0x0000,
		buf,
		time.Second*5,
	)
	
	if err != nil {
		t.Errorf("Control transfer failed: %v", err)
	} else if n != 18 {
		t.Errorf("Expected 18 bytes, got %d", n)
	} else {
		if buf[0] != 18 {
			t.Errorf("Invalid descriptor length: %d", buf[0])
		}
		if buf[1] != 0x01 {
			t.Errorf("Invalid descriptor type: 0x%02x", buf[1])
		}
	}
}

func TestStringDescriptor(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}
	
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	devices, err := ctx.GetDeviceList()
	if err != nil || len(devices) == 0 {
		t.Skip("No USB devices available for testing")
	}
	
	var testDevice *Device
	for _, dev := range devices {
		if dev.Descriptor.ManufacturerIndex > 0 || dev.Descriptor.ProductIndex > 0 {
			testDevice = dev
			break
		}
	}
	
	if testDevice == nil {
		t.Skip("No device with string descriptors found")
	}
	
	handle, err := testDevice.Open()
	if err != nil {
		if err == ErrPermissionDenied {
			t.Skip("Permission denied to open USB device")
		}
		t.Fatalf("Failed to open device: %v", err)
	}
	defer handle.Close()
	
	if testDevice.Descriptor.ManufacturerIndex > 0 {
		manufacturer, err := handle.GetStringDescriptor(testDevice.Descriptor.ManufacturerIndex)
		if err != nil {
			t.Errorf("Failed to get manufacturer string: %v", err)
		} else {
			t.Logf("Manufacturer: %s", manufacturer)
		}
	}
	
	if testDevice.Descriptor.ProductIndex > 0 {
		product, err := handle.GetStringDescriptor(testDevice.Descriptor.ProductIndex)
		if err != nil {
			t.Errorf("Failed to get product string: %v", err)
		} else {
			t.Logf("Product: %s", product)
		}
	}
}

func BenchmarkGetDeviceList(b *testing.B) {
	ctx, err := NewContext()
	if err != nil {
		b.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ctx.GetDeviceList()
		if err != nil {
			b.Fatalf("Failed to get device list: %v", err)
		}
	}
}

func ExampleContext_GetDeviceList() {
	ctx, err := NewContext()
	if err != nil {
		panic(err)
	}
	defer ctx.Close()
	
	devices, err := ctx.GetDeviceList()
	if err != nil {
		panic(err)
	}
	
	for _, dev := range devices {
		fmt.Printf("Device: VID=0x%04x PID=0x%04x\n",
			dev.Descriptor.VendorID, dev.Descriptor.ProductID)
	}
}