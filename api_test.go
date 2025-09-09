package usb

import (
	"os"
	"testing"
	"time"
)

func TestAsyncTransferManager(t *testing.T) {
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
	
	// Find a suitable test device
	var testDevice *Device
	for _, dev := range devices {
		// Skip root hubs, look for actual devices
		if dev.Descriptor.VendorID != 0x1d6b {
			testDevice = dev
			break
		}
	}
	
	if testDevice == nil {
		t.Skip("No suitable test device found")
	}
	
	handle, err := testDevice.Open()
	if err != nil {
		if err == ErrPermissionDenied {
			t.Skip("Permission denied - run as root")
		}
		t.Fatalf("Failed to open device: %v", err)
	}
	defer handle.Close()
	
	// Test async transfer manager creation
	manager, err := handle.NewAsyncTransferManager()
	if err != nil {
		t.Fatalf("Failed to create async transfer manager: %v", err)
	}
	defer manager.Close()
	
	// Test transfer creation
	transfer := manager.NewAsyncTransfer(0x80, TransferTypeBulk, 512, 0)
	if transfer == nil {
		t.Fatal("Failed to create transfer")
	}
	
	// Test transfer properties
	transfer.SetTimeout(1 * time.Second)
	transfer.SetUserData("test data")
	
	if transfer.GetStatus() != TransferCompleted {
		t.Errorf("Expected initial status TransferCompleted, got %v", transfer.GetStatus())
	}
	
	// Test callback
	transfer.SetCallback(func(t *AsyncTransfer) {
		// Callback is set but may not be called in this test
	})
	
	// Start manager
	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	
	t.Logf("Created async transfer manager and transfer successfully")
}

func TestDetachKernelDriver(t *testing.T) {
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
	
	// Find a device with interfaces
	var testDevice *Device
	for _, dev := range devices {
		if dev.Descriptor.VendorID != 0x1d6b && dev.Descriptor.NumConfigurations > 0 {
			testDevice = dev
			break
		}
	}
	
	if testDevice == nil {
		t.Skip("No suitable test device found")
	}
	
	handle, err := testDevice.Open()
	if err != nil {
		if err == ErrPermissionDenied {
			t.Skip("Permission denied - run as root")
		}
		t.Fatalf("Failed to open device: %v", err)
	}
	defer handle.Close()
	
	// Test kernel driver detach (should not fail even if no driver)
	err = handle.DetachKernelDriver(0)
	if err != nil && err != ErrNotSupported {
		t.Logf("Detach kernel driver returned: %v (this may be expected)", err)
	}
	
	t.Logf("Kernel driver detach test completed")
}

func TestEventHandling(t *testing.T) {
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	// Test basic event handling
	err = ctx.HandleEvents()
	if err != nil {
		t.Errorf("HandleEvents failed: %v", err)
	}
	
	// Test event handling with timeout
	start := time.Now()
	err = ctx.HandleEventsTimeout(100 * time.Millisecond)
	duration := time.Since(start)
	
	if err != nil {
		t.Errorf("HandleEventsTimeout failed: %v", err)
	}
	
	// Should have waited approximately the timeout duration
	if duration < 50*time.Millisecond || duration > 200*time.Millisecond {
		t.Logf("HandleEventsTimeout duration: %v (expected ~100ms)", duration)
	}
	
	t.Logf("Event handling test completed")
}

func TestIsochronousTransfer(t *testing.T) {
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
	
	// Find a device that might support isochronous transfers (like a camera)
	var testDevice *Device
	for _, dev := range devices {
		// Look for video devices
		if dev.Descriptor.DeviceClass == 14 || // Video class
			(dev.Descriptor.VendorID == 0x046d && dev.Descriptor.ProductID == 0x08e5) { // Logitech camera
			testDevice = dev
			break
		}
	}
	
	if testDevice == nil {
		t.Skip("No suitable device for isochronous transfer testing")
	}
	
	handle, err := testDevice.Open()
	if err != nil {
		if err == ErrPermissionDenied {
			t.Skip("Permission denied - run as root")
		}
		t.Skip("Could not open test device")
	}
	defer handle.Close()
	
	manager, err := handle.NewAsyncTransferManager()
	if err != nil {
		t.Fatalf("Failed to create async transfer manager: %v", err)
	}
	defer manager.Close()
	
	// Test isochronous transfer creation
	transfer := manager.NewAsyncTransfer(0x81, TransferTypeIsochronous, 1024, 8)
	if transfer == nil {
		t.Fatal("Failed to create isochronous transfer")
	}
	
	// Check iso packets were created
	packets := transfer.GetIsoPackets()
	if len(packets) != 8 {
		t.Errorf("Expected 8 iso packets, got %d", len(packets))
	}
	
	for i, packet := range packets {
		if packet.Length != 128 { // 1024 / 8
			t.Errorf("Packet %d: expected length 128, got %d", i, packet.Length)
		}
	}
	
	t.Logf("Isochronous transfer test completed")
}

func BenchmarkAsyncTransferCreation(b *testing.B) {
	ctx, err := NewContext()
	if err != nil {
		b.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	devices, err := ctx.GetDeviceList()
	if err != nil || len(devices) == 0 {
		b.Skip("No USB devices available for benchmarking")
	}
	
	device := devices[0]
	handle, err := device.Open()
	if err != nil {
		b.Skip("Could not open device for benchmarking")
	}
	defer handle.Close()
	
	manager, err := handle.NewAsyncTransferManager()
	if err != nil {
		b.Fatalf("Failed to create async transfer manager: %v", err)
	}
	defer manager.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transfer := manager.NewAsyncTransfer(0x80, TransferTypeBulk, 512, 0)
		if transfer == nil {
			b.Fatal("Failed to create transfer")
		}
	}
}

func TestTransferStatusProgression(t *testing.T) {
	ctx, err := NewContext()
	if err != nil {
		t.Fatalf("Failed to create context: %v", err)
	}
	defer ctx.Close()
	
	devices, err := ctx.GetDeviceList()
	if err != nil || len(devices) == 0 {
		t.Skip("No USB devices available for testing")
	}
	
	device := devices[0]
	handle, err := device.Open()
	if err != nil {
		t.Skip("Could not open device")
	}
	defer handle.Close()
	
	manager, err := handle.NewAsyncTransferManager()
	if err != nil {
		t.Fatalf("Failed to create async transfer manager: %v", err)
	}
	defer manager.Close()
	
	transfer := manager.NewAsyncTransfer(0x80, TransferTypeBulk, 512, 0)
	
	// Test initial status
	if status := transfer.GetStatus(); status != TransferCompleted {
		t.Errorf("Expected initial status TransferCompleted, got %v", status)
	}
	
	// Test cancellation
	err = transfer.Cancel()
	if err != nil {
		t.Errorf("Cancel failed: %v", err)
	}
	
	if status := transfer.GetStatus(); status != TransferCancelled {
		t.Errorf("Expected status TransferCancelled after cancel, got %v", status)
	}
}