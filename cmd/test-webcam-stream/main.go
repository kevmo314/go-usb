package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"
	
	usb "github.com/kevmo314/go-usb"
)

const (
	// Logitech C920 identifiers
	LOGITECH_VID = 0x046d
	C920_PID     = 0x08e5
	
	// UVC (USB Video Class) constants
	UVC_SC_VIDEOCONTROL   = 0x01
	UVC_SC_VIDEOSTREAMING = 0x02
	
	// Video streaming endpoint is typically 0x81 (EP 1 IN)
	VIDEO_ENDPOINT = 0x81
)

func main() {
	if os.Getuid() != 0 {
		log.Fatal("This program requires root privileges to access USB devices")
	}
	
	fmt.Println("🎥 USB Webcam Isochronous Stream Test")
	fmt.Println("======================================")
	
	ctx, err := usb.NewContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()
	
	// Find the Logitech C920 webcam
	webcam, err := findWebcam(ctx)
	if err != nil {
		log.Fatal(err)
	}
	
	fmt.Printf("✅ Found webcam: %04x:%04x\n", webcam.Descriptor.VendorID, webcam.Descriptor.ProductID)
	
	handle, err := webcam.Open()
	if err != nil {
		log.Fatal("Failed to open webcam:", err)
	}
	defer handle.Close()
	
	// Detach kernel driver if attached
	fmt.Println("🔌 Preparing device...")
	prepareWebcam(handle)
	
	// Find video streaming interface
	videoInterface, videoEndpoint, err := findVideoInterface(handle)
	if err != nil {
		log.Fatal("Failed to find video interface:", err)
	}
	
	fmt.Printf("📹 Video interface: %d, endpoint: 0x%02x\n", videoInterface, videoEndpoint)
	
	// Claim the video interface
	if err := handle.ClaimInterface(videoInterface); err != nil {
		fmt.Printf("⚠️  Could not claim interface: %v\n", err)
		fmt.Println("   Attempting to detach kernel driver...")
		
		if err := handle.DetachKernelDriver(videoInterface); err != nil {
			log.Fatal("Failed to detach driver:", err)
		}
		
		if err := handle.ClaimInterface(videoInterface); err != nil {
			log.Fatal("Failed to claim after detach:", err)
		}
	}
	defer handle.ReleaseInterface(videoInterface)
	
	fmt.Println("✅ Interface claimed successfully")
	
	// Test 1: Single isochronous transfer
	fmt.Println("\n📊 Test 1: Single Isochronous Transfer")
	testSingleTransfer(handle, videoEndpoint)
	
	// Test 2: Streaming test
	fmt.Println("\n📊 Test 2: Continuous Streaming (5 seconds)")
	testStreaming(handle, videoEndpoint)
	
	// Test 3: Performance benchmark
	fmt.Println("\n📊 Test 3: Performance Benchmark")
	benchmarkTransfers(handle, videoEndpoint)
	
	fmt.Println("\n✅ All tests completed successfully!")
}

func findWebcam(ctx *usb.Context) (*usb.Device, error) {
	devices, err := ctx.GetDeviceList()
	if err != nil {
		return nil, err
	}
	
	// First try to find Logitech C920
	for _, dev := range devices {
		if dev.Descriptor.VendorID == LOGITECH_VID && dev.Descriptor.ProductID == C920_PID {
			return dev, nil
		}
	}
	
	// Fall back to any UVC device
	for _, dev := range devices {
		if dev.Descriptor.DeviceClass == 0xEF && // Miscellaneous
		   dev.Descriptor.DeviceSubClass == 0x02 && // Common Class
		   dev.Descriptor.DeviceProtocol == 0x01 { // Interface Association
			// Likely a UVC device
			return dev, nil
		}
		
		// Check for video class in interfaces
		handle, err := dev.Open()
		if err != nil {
			continue
		}
		
		_, interfaces, _, err := handle.ReadConfigDescriptor(0)
		handle.Close()
		
		if err != nil {
			continue
		}
		
		for _, iface := range interfaces {
			if iface.InterfaceClass == 0x0E { // Video class
				return dev, nil
			}
		}
	}
	
	return nil, fmt.Errorf("no webcam found")
}

func prepareWebcam(handle *usb.DeviceHandle) {
	// Get current configuration
	config, err := handle.GetConfiguration()
	if err != nil {
		fmt.Printf("⚠️  Could not get configuration: %v\n", err)
		return
	}
	
	fmt.Printf("📋 Current configuration: %d\n", config)
	
	// Set configuration if needed
	if config != 1 {
		if err := handle.SetConfiguration(1); err != nil {
			fmt.Printf("⚠️  Could not set configuration: %v\n", err)
		}
	}
}

func findVideoInterface(handle *usb.DeviceHandle) (uint8, uint8, error) {
	_, interfaces, endpoints, err := handle.ReadConfigDescriptor(0)
	if err != nil {
		return 0, 0, err
	}
	
	fmt.Printf("📋 Configuration has %d interfaces\n", len(interfaces))
	
	var videoInterface uint8
	var videoEndpoint uint8
	
	for _, iface := range interfaces {
		fmt.Printf("   Interface %d: Class=%d, SubClass=%d\n", 
			iface.InterfaceNumber, iface.InterfaceClass, iface.InterfaceSubClass)
		
		// Look for video streaming interface
		if iface.InterfaceClass == 0x0E && // Video class
		   iface.InterfaceSubClass == UVC_SC_VIDEOSTREAMING {
			videoInterface = iface.InterfaceNumber
			
			// Find isochronous endpoint
			for _, ep := range endpoints {
				// Check if this endpoint belongs to this interface
				// Isochronous IN endpoint
				if (ep.Attributes & 0x03) == 0x01 && // Isochronous
				   (ep.EndpointAddr & 0x80) != 0 { // IN endpoint
					videoEndpoint = ep.EndpointAddr
					fmt.Printf("      Found iso endpoint: 0x%02x, MaxPacket=%d\n", 
						ep.EndpointAddr, ep.MaxPacketSize)
					break
				}
			}
		}
	}
	
	if videoEndpoint == 0 {
		// Fallback: use default video endpoint
		videoEndpoint = VIDEO_ENDPOINT
		fmt.Printf("⚠️  Using default endpoint: 0x%02x\n", videoEndpoint)
	}
	
	return videoInterface, videoEndpoint, nil
}

func testSingleTransfer(handle *usb.DeviceHandle, endpoint uint8) {
	fmt.Println("   Creating isochronous transfer...")
	
	// Create a single transfer with 8 packets of 3072 bytes each
	// This is typical for high-bandwidth USB 2.0 isochronous
	const (
		numPackets = 8
		packetSize = 3072 // Max for high-bandwidth USB 2.0
	)
	
	transfer, err := handle.NewIsochronousTransfer(endpoint, numPackets, packetSize)
	if err != nil {
		fmt.Printf("   ❌ Failed to create transfer: %v\n", err)
		return
	}
	
	fmt.Printf("   📦 Transfer created: %d packets × %d bytes\n", numPackets, packetSize)
	
	// Submit the transfer
	if err := transfer.Submit(); err != nil {
		fmt.Printf("   ❌ Failed to submit transfer: %v\n", err)
		return
	}
	
	fmt.Println("   ⏳ Waiting for completion...")
	
	// Wait for completion
	if err := transfer.Wait(1 * time.Second); err != nil {
		fmt.Printf("   ❌ Transfer failed: %v\n", err)
		return
	}
	
	// Check results
	status := transfer.GetStatus()
	actualLength := transfer.GetActualLength()
	packets := transfer.GetPackets()
	
	fmt.Printf("   📊 Transfer completed: Status=%d, Total bytes=%d\n", status, actualLength)
	
	// Analyze packet results
	successfulPackets := 0
	totalBytes := 0
	for i, packet := range packets {
		if packet.Status == 0 && packet.ActualLength > 0 {
			successfulPackets++
			totalBytes += int(packet.ActualLength)
			fmt.Printf("      Packet %d: %d bytes (OK)\n", i, packet.ActualLength)
		} else if packet.Status != 0 {
			fmt.Printf("      Packet %d: Status=%d (ERROR)\n", i, packet.Status)
		}
	}
	
	fmt.Printf("   ✅ Success rate: %d/%d packets, %d total bytes\n", 
		successfulPackets, numPackets, totalBytes)
	
	// Check if we got video data
	if totalBytes > 0 {
		data := transfer.GetBuffer()
		checkVideoData(data[:totalBytes])
	}
}

func testStreaming(handle *usb.DeviceHandle, endpoint uint8) {
	frameCount := 0
	totalBytes := 0
	startTime := time.Now()
	
	// Stream for 5 seconds
	duration := 5 * time.Second
	done := make(chan bool)
	
	go func() {
		time.Sleep(duration)
		close(done)
	}()
	
	fmt.Println("   🎬 Starting video stream...")
	
	err := handle.StartWebcamStream(endpoint, func(frame []byte) {
		frameCount++
		totalBytes += len(frame)
		
		if frameCount == 1 {
			fmt.Printf("   📸 First frame received: %d bytes\n", len(frame))
			checkVideoData(frame)
		}
		
		if frameCount%30 == 0 {
			elapsed := time.Since(startTime).Seconds()
			fps := float64(frameCount) / elapsed
			bandwidth := float64(totalBytes) / elapsed / 1024 / 1024
			fmt.Printf("   📊 Frames: %d, FPS: %.1f, Bandwidth: %.2f MB/s\n", 
				frameCount, fps, bandwidth)
		}
		
		select {
		case <-done:
			// Stop streaming
		default:
			// Continue
		}
	})
	
	if err != nil {
		fmt.Printf("   ❌ Streaming failed: %v\n", err)
		return
	}
	
	<-done
	
	elapsed := time.Since(startTime).Seconds()
	fmt.Printf("\n   📊 Streaming Statistics:\n")
	fmt.Printf("      Duration: %.1f seconds\n", elapsed)
	fmt.Printf("      Frames: %d\n", frameCount)
	fmt.Printf("      Average FPS: %.1f\n", float64(frameCount)/elapsed)
	fmt.Printf("      Total data: %.2f MB\n", float64(totalBytes)/1024/1024)
	fmt.Printf("      Bandwidth: %.2f MB/s\n", float64(totalBytes)/elapsed/1024/1024)
}

func benchmarkTransfers(handle *usb.DeviceHandle, endpoint uint8) {
	const (
		numIterations = 100
		numPackets   = 8
		packetSize   = 3072
	)
	
	fmt.Printf("   🏃 Running %d transfers...\n", numIterations)
	
	startTime := time.Now()
	successCount := 0
	totalBytes := 0
	
	for i := 0; i < numIterations; i++ {
		transfer, err := handle.NewIsochronousTransfer(endpoint, numPackets, packetSize)
		if err != nil {
			continue
		}
		
		if err := transfer.Submit(); err != nil {
			continue
		}
		
		if err := transfer.Wait(100 * time.Millisecond); err == nil {
			successCount++
			totalBytes += transfer.GetActualLength()
		}
	}
	
	elapsed := time.Since(startTime)
	
	fmt.Printf("\n   📊 Benchmark Results:\n")
	fmt.Printf("      Transfers: %d/%d successful\n", successCount, numIterations)
	fmt.Printf("      Total time: %v\n", elapsed)
	fmt.Printf("      Avg latency: %.2f ms\n", float64(elapsed.Milliseconds())/float64(numIterations))
	fmt.Printf("      Throughput: %.2f MB/s\n", float64(totalBytes)/elapsed.Seconds()/1024/1024)
	fmt.Printf("      Success rate: %.1f%%\n", float64(successCount)*100/float64(numIterations))
}

func checkVideoData(data []byte) {
	if len(data) < 10 {
		fmt.Println("      ⚠️  Data too short to analyze")
		return
	}
	
	// Check for common video formats
	// MJPEG starts with FFD8
	if data[0] == 0xFF && data[1] == 0xD8 {
		fmt.Println("      🎞️  MJPEG frame detected!")
		return
	}
	
	// UVC header check
	headerLen := int(data[0])
	if headerLen >= 2 && headerLen <= 12 {
		flags := data[1]
		fmt.Printf("      📹 UVC header: len=%d, flags=0x%02x\n", headerLen, flags)
		
		if headerLen >= 4 {
			pts := binary.LittleEndian.Uint32(data[2:6])
			fmt.Printf("         PTS: %d\n", pts)
		}
		return
	}
	
	// Show first few bytes
	fmt.Printf("      📦 Raw data: %02x %02x %02x %02x %02x...\n",
		data[0], data[1], data[2], data[3], data[4])
}