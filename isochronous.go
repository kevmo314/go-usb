package usb

import (
	"fmt"
	"sync"
	"syscall"
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
	USBDEVFS_URB_SHORT_NOT_OK      = 0x01
	USBDEVFS_URB_ISO_ASAP          = 0x02
	USBDEVFS_URB_BULK_CONTINUATION = 0x04
	USBDEVFS_URB_NO_FSBR           = 0x20
	USBDEVFS_URB_ZERO_PACKET       = 0x40
	USBDEVFS_URB_NO_INTERRUPT      = 0x80
)

// IsoPacketDescriptor represents a single isochronous packet
type IsoPacketDescriptor struct {
	Length       uint32
	ActualLength uint32
	Status       int32
}

// URB represents a USB Request Block for kernel communication
type URB struct {
	Type         uint8
	Endpoint     uint8
	Status       int32
	Flags        uint32
	Buffer       unsafe.Pointer
	BufferLength int32
	ActualLength int32
	StartFrame   int32
	// Union field: either NumberOfPackets or StreamID
	NumberOfPackets int32 // For isochronous transfers
	ErrorCount      int32
	SignalNumber    uint32
	UserContext     uintptr
	// Iso packet descriptors follow the main struct
}

// IsochronousTransfer represents a complete isochronous transfer
type IsochronousTransfer struct {
	handle     *DeviceHandle
	endpoint   uint8
	numPackets int
	packetSize int
	buffer     []byte
	packets    []IsoPacketDescriptor
	urb        *URB
	urbBuffer  []byte // Holds URB + packet descriptors
	submitted  bool

	// Auto-reaping support
	reapErr  error
	reaped   bool
	reapCond *sync.Cond
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

	return &IsochronousTransfer{
		handle:     h,
		endpoint:   endpoint,
		numPackets: numPackets,
		packetSize: packetSize,
		buffer:     buffer,
		packets:    packets,
		urb:        urb,
		urbBuffer:  urbBuffer,
		reapCond:   sync.NewCond(&sync.Mutex{}),
	}, nil
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

	// Reset URB fields for resubmission
	t.urb.Status = 0
	t.urb.ActualLength = 0
	t.urb.ErrorCount = 0

	// Reset packet descriptors
	isoPackets := (*[1 << 16]IsoPacketDescriptor)(unsafe.Pointer(
		uintptr(unsafe.Pointer(t.urb)) + unsafe.Sizeof(URB{})))
	for i := 0; i < t.numPackets; i++ {
		isoPackets[i].ActualLength = 0
		isoPackets[i].Status = 0
		isoPackets[i].Length = uint32(t.packetSize)
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
	t.reaped = false

	// Register with centralized reaper
	t.handle.registerURBCompletion(uintptr(unsafe.Pointer(t.urb)), func(err error) {
		// Process URB completion
		t.reapCond.L.Lock()
		defer t.reapCond.L.Unlock()

		t.reapErr = err

		if err == nil {
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
		}

		// Clear submitted flag to allow resubmission
		t.submitted = false
		t.reaped = true
		t.reapCond.Broadcast()
	})

	return nil
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

// waitForReaping waits for the transfer to be reaped
func (t *IsochronousTransfer) waitForReaping() {
	t.reapCond.L.Lock()
	defer t.reapCond.L.Unlock()

	for !t.reaped {
		t.reapCond.Wait()
	}
}

// Wait waits for the transfer to complete
func (t *IsochronousTransfer) Wait() error {
	t.waitForReaping()
	return t.reapErr
}

// GetPackets returns the packet descriptors with actual transfer results
func (t *IsochronousTransfer) GetPackets() []IsoPacketDescriptor {
	t.waitForReaping()
	return t.packets
}

// GetBuffer returns the transfer buffer
func (t *IsochronousTransfer) GetBuffer() []byte {
	t.waitForReaping()
	return t.buffer
}

// GetActualLength returns the total actual bytes transferred
func (t *IsochronousTransfer) GetActualLength() int {
	t.waitForReaping()
	return int(t.urb.ActualLength)
}

// GetStatus returns the transfer status
func (t *IsochronousTransfer) GetStatus() int32 {
	t.waitForReaping()
	return t.urb.Status
}

// GetIsoPacketBuffer returns the data buffer for a specific isochronous packet.
// Similar to libusb's libusb_get_iso_packet_buffer function.
// The offset is calculated using the Length field (allocated size), but only
// ActualLength bytes are returned as valid data.
func (t *IsochronousTransfer) GetIsoPacketBuffer(packetIndex int) ([]byte, error) {
	t.waitForReaping()
	if t.reapErr != nil {
		return nil, t.reapErr
	}

	if packetIndex < 0 || packetIndex >= len(t.packets) {
		return nil, fmt.Errorf("packet index %d out of range [0, %d)", packetIndex, len(t.packets))
	}

	pkt := t.packets[packetIndex]

	// Return nil for error packets
	if pkt.Status != 0 {
		return nil, fmt.Errorf("packet %d has error status: %d", packetIndex, pkt.Status)
	}

	// Return empty slice for zero-length packets
	if pkt.ActualLength == 0 {
		return []byte{}, nil
	}

	// Calculate offset using Length (allocated size) of all previous packets
	offset := 0
	for i := 0; i < packetIndex; i++ {
		offset += int(t.packets[i].Length)
	}

	// Return slice with ActualLength bytes of valid data
	return t.buffer[offset : offset+int(pkt.ActualLength)], nil
}

// GetIsoPacketBufferSlices returns slices for all isochronous packets in a single pass.
// This is more efficient than calling GetIsoPacketBuffer repeatedly as it only
// calculates offsets once. Returns a slice for each packet, where error packets
// get nil slices and successful packets get slices into the main buffer.
func (t *IsochronousTransfer) GetIsoPacketBufferSlices() [][]byte {
	t.waitForReaping()

	slices := make([][]byte, len(t.packets))
	offset := 0

	for i, pkt := range t.packets {
		if pkt.Status != 0 || pkt.ActualLength == 0 {
			// Error packet or zero-length packet
			slices[i] = nil
		} else {
			// Valid packet with data - return ActualLength bytes
			slices[i] = t.buffer[offset : offset+int(pkt.ActualLength)]
		}

		// Always advance offset by Length (allocated size), not ActualLength
		offset += int(pkt.Length)
	}

	return slices
}
