package usb

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// URB types
const (
	USBDEVFS_URB_TYPE_ISO       = 0
	USBDEVFS_URB_TYPE_INTERRUPT = 1
	USBDEVFS_URB_TYPE_CONTROL   = 2
	USBDEVFS_URB_TYPE_BULK      = 3
)

// URB flags
const (
	USBDEVFS_URB_SHORT_NOT_OK     = 0x01
	USBDEVFS_URB_ISO_ASAP         = 0x02
	USBDEVFS_URB_BULK_CONTINUATION = 0x04
	USBDEVFS_URB_NO_FSBR          = 0x20
	USBDEVFS_URB_ZERO_PACKET      = 0x40
	USBDEVFS_URB_NO_INTERRUPT     = 0x80
)

// IsoPacketDescriptor represents a single isochronous packet
type IsoPacketDescriptor struct {
	Length       uint32
	ActualLength uint32
	Status       int32
}

// URB represents a USB Request Block for kernel communication
type URB struct {
	Type          uint8
	Endpoint      uint8
	Status        int32
	Flags         uint32
	Buffer        unsafe.Pointer
	BufferLength  int32
	ActualLength  int32
	StartFrame    int32
	// Union field: either NumberOfPackets or StreamID
	NumberOfPackets int32 // For isochronous transfers
	ErrorCount      int32
	SignalNumber    uint32
	UserContext     uintptr
	// Iso packet descriptors follow the main struct
}

// IsochronousTransfer represents a complete isochronous transfer
type IsochronousTransfer struct {
	handle       *DeviceHandle
	endpoint     uint8
	numPackets   int
	packetSize   int
	buffer       []byte
	packets      []IsoPacketDescriptor
	urb          *URB
	urbBuffer    []byte // Holds URB + packet descriptors
	submitted    bool
	completed    chan bool
	callback     func(transfer *IsochronousTransfer)
}

// NewIsochronousTransfer creates a new isochronous transfer
func (h *DeviceHandle) NewIsochronousTransfer(endpoint uint8, numPackets int, packetSize int) (*IsochronousTransfer, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if h.closed {
		return nil, ErrDeviceNotFound
	}
	
	// Allocate buffer for all packets
	bufferSize := numPackets * packetSize
	buffer := make([]byte, bufferSize)
	
	// Allocate packet descriptors
	packets := make([]IsoPacketDescriptor, numPackets)
	for i := range packets {
		packets[i].Length = uint32(packetSize)
	}
	
	// Calculate total URB size: URB struct + iso packet descriptors
	urbSize := unsafe.Sizeof(URB{}) + uintptr(numPackets)*unsafe.Sizeof(IsoPacketDescriptor{})
	urbBuffer := make([]byte, urbSize)
	
	// Set up URB pointer
	urb := (*URB)(unsafe.Pointer(&urbBuffer[0]))
	urb.Type = USBDEVFS_URB_TYPE_ISO
	urb.Endpoint = endpoint
	urb.Flags = USBDEVFS_URB_ISO_ASAP // Start ASAP
	urb.Buffer = unsafe.Pointer(&buffer[0])
	urb.BufferLength = int32(bufferSize)
	urb.NumberOfPackets = int32(numPackets)
	urb.StartFrame = -1 // Let kernel choose start frame
	
	// Copy packet descriptors after URB struct
	isoPackets := (*[1 << 16]IsoPacketDescriptor)(unsafe.Pointer(
		uintptr(unsafe.Pointer(&urbBuffer[0])) + unsafe.Sizeof(URB{})))
	for i := range packets {
		isoPackets[i] = packets[i]
	}
	
	transfer := &IsochronousTransfer{
		handle:     h,
		endpoint:   endpoint,
		numPackets: numPackets,
		packetSize: packetSize,
		buffer:     buffer,
		packets:    packets,
		urb:        urb,
		urbBuffer:  urbBuffer,
		completed:  make(chan bool, 1),
	}
	
	return transfer, nil
}

// Submit submits the isochronous transfer to the kernel
func (t *IsochronousTransfer) Submit() error {
	if t.submitted {
		return fmt.Errorf("transfer already submitted")
	}
	
	t.handle.mu.RLock()
	defer t.handle.mu.RUnlock()
	
	if t.handle.closed {
		return ErrDeviceNotFound
	}
	
	// Submit URB to kernel
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(t.handle.fd),
		USBDEVFS_SUBMITURB,
		uintptr(unsafe.Pointer(t.urb)),
	)
	
	if errno != 0 {
		return fmt.Errorf("failed to submit URB: %v", errno)
	}
	
	t.submitted = true
	
	// Start a goroutine to reap the URB
	go t.reapURB()
	
	return nil
}

// reapURB waits for URB completion and processes the result
func (t *IsochronousTransfer) reapURB() {
	// Wait for URB completion using REAPURB ioctl
	var reapedURB *URB
	
	for {
		_, _, errno := syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(t.handle.fd),
			USBDEVFS_REAPURB,
			uintptr(unsafe.Pointer(&reapedURB)),
		)
		
		if errno == 0 && reapedURB == t.urb {
			// URB completed successfully
			break
		}
		
		if errno == syscall.EAGAIN {
			// No URB ready yet, try again
			time.Sleep(1 * time.Millisecond)
			continue
		}
		
		if errno != 0 {
			// Error occurred
			t.urb.Status = -int32(errno)
			break
		}
	}
	
	// Update packet descriptors from kernel data
	isoPackets := (*[1 << 16]IsoPacketDescriptor)(unsafe.Pointer(
		uintptr(unsafe.Pointer(t.urb)) + unsafe.Sizeof(URB{})))
	
	for i := 0; i < t.numPackets; i++ {
		t.packets[i] = isoPackets[i]
	}
	
	// Update actual length
	t.urb.ActualLength = 0
	for i := range t.packets {
		t.urb.ActualLength += int32(t.packets[i].ActualLength)
	}
	
	// Signal completion
	select {
	case t.completed <- true:
	default:
	}
	
	// Call callback if set
	if t.callback != nil {
		t.callback(t)
	}
}

// Cancel cancels a submitted transfer
func (t *IsochronousTransfer) Cancel() error {
	if !t.submitted {
		return fmt.Errorf("transfer not submitted")
	}
	
	t.handle.mu.RLock()
	defer t.handle.mu.RUnlock()
	
	if t.handle.closed {
		return ErrDeviceNotFound
	}
	
	// Discard the URB
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(t.handle.fd),
		USBDEVFS_DISCARDURB,
		uintptr(unsafe.Pointer(t.urb)),
	)
	
	if errno != 0 && errno != syscall.EINVAL {
		return fmt.Errorf("failed to cancel URB: %v", errno)
	}
	
	return nil
}

// Wait waits for the transfer to complete
func (t *IsochronousTransfer) Wait(timeout time.Duration) error {
	if !t.submitted {
		return fmt.Errorf("transfer not submitted")
	}
	
	select {
	case <-t.completed:
		return nil
	case <-time.After(timeout):
		return ErrTimeout
	}
}

// GetPackets returns the packet descriptors with actual transfer results
func (t *IsochronousTransfer) GetPackets() []IsoPacketDescriptor {
	return t.packets
}

// GetBuffer returns the transfer buffer
func (t *IsochronousTransfer) GetBuffer() []byte {
	return t.buffer
}

// GetActualLength returns the total actual bytes transferred
func (t *IsochronousTransfer) GetActualLength() int {
	return int(t.urb.ActualLength)
}

// GetStatus returns the transfer status
func (t *IsochronousTransfer) GetStatus() int32 {
	return t.urb.Status
}

// SetCallback sets a completion callback
func (t *IsochronousTransfer) SetCallback(callback func(*IsochronousTransfer)) {
	t.callback = callback
}

// StreamIsochronous provides a high-level streaming interface for isochronous transfers
func (h *DeviceHandle) StreamIsochronous(endpoint uint8, numPackets int, packetSize int, numTransfers int, callback func(data []byte, packets []IsoPacketDescriptor)) error {
	// Create a pool of transfers for double/triple buffering
	transfers := make([]*IsochronousTransfer, numTransfers)
	
	for i := range transfers {
		transfer, err := h.NewIsochronousTransfer(endpoint, numPackets, packetSize)
		if err != nil {
			return fmt.Errorf("failed to create transfer %d: %w", i, err)
		}
		
		transfers[i] = transfer
		
		// Set callback to re-submit and process data
		transfer.SetCallback(func(t *IsochronousTransfer) {
			// Process received data
			if callback != nil {
				callback(t.GetBuffer(), t.GetPackets())
			}
			
			// Re-submit for continuous streaming
			t.submitted = false
			t.Submit()
		})
	}
	
	// Submit all transfers to start streaming
	for _, transfer := range transfers {
		if err := transfer.Submit(); err != nil {
			return fmt.Errorf("failed to submit transfer: %w", err)
		}
	}
	
	return nil
}

// Helper function for webcam streaming
func (h *DeviceHandle) StartWebcamStream(endpoint uint8, callback func(frame []byte)) error {
	// Typical webcam parameters
	// 960x720 MJPEG at 30fps requires about 40KB per frame
	// Split into 128 packets of 1024 bytes each
	const (
		packetsPerTransfer = 128
		packetSize        = 1024
		numTransfers      = 3 // Triple buffering
	)
	
	frameBuffer := make([]byte, 0, packetsPerTransfer*packetSize)
	
	return h.StreamIsochronous(endpoint, packetsPerTransfer, packetSize, numTransfers,
		func(data []byte, packets []IsoPacketDescriptor) {
			// Accumulate data from packets
			for i, packet := range packets {
				if packet.Status == 0 && packet.ActualLength > 0 {
					start := i * packetSize
					end := start + int(packet.ActualLength)
					if end <= len(data) {
						frameBuffer = append(frameBuffer, data[start:end]...)
					}
				}
			}
			
			// Check for MJPEG frame boundary (FFD8 start, FFD9 end)
			if len(frameBuffer) >= 2 {
				// Look for JPEG end marker
				if frameBuffer[len(frameBuffer)-2] == 0xFF && frameBuffer[len(frameBuffer)-1] == 0xD9 {
					// Complete frame received
					if callback != nil {
						callback(frameBuffer)
					}
					frameBuffer = frameBuffer[:0] // Reset buffer
				}
			}
			
			// Prevent buffer overflow
			if len(frameBuffer) > 500*1024 { // 500KB max frame size
				frameBuffer = frameBuffer[:0]
			}
		})
}