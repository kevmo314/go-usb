package usb

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <IOKit/usb/IOUSBLib.h>
#include <CoreFoundation/CoreFoundation.h>

// Isochronous transfer support
typedef struct {
    IOUSBIsocFrame *frames;
    UInt32 numFrames;
    void *buffer;
    UInt32 bufferSize;
    void *userData;
} IsocTransferContext;

// Read isochronous data
int ReadIsocPipe(IOUSBInterfaceInterface300 **interfaceInterface,
                UInt8 pipeRef,
                void *buf,
                UInt64 frameStart,
                UInt32 numFrames,
                IOUSBIsocFrame *frameList) {
    return (*interfaceInterface)->ReadIsochPipeAsync(interfaceInterface,
                                                     pipeRef,
                                                     buf,
                                                     frameStart,
                                                     numFrames,
                                                     frameList,
                                                     NULL, // callback
                                                     NULL); // refCon
}

// Write isochronous data
int WriteIsocPipe(IOUSBInterfaceInterface300 **interfaceInterface,
                 UInt8 pipeRef,
                 void *buf,
                 UInt64 frameStart,
                 UInt32 numFrames,
                 IOUSBIsocFrame *frameList) {
    return (*interfaceInterface)->WriteIsochPipeAsync(interfaceInterface,
                                                      pipeRef,
                                                      buf,
                                                      frameStart,
                                                      numFrames,
                                                      frameList,
                                                      NULL, // callback
                                                      NULL); // refCon
}

// Get bus frame number
int GetBusFrameNumber(IOUSBInterfaceInterface300 **interfaceInterface, UInt64 *frame, AbsoluteTime *atTime) {
    return (*interfaceInterface)->GetBusFrameNumber(interfaceInterface, frame, atTime);
}

*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"
)

// IsochronousTransfer represents an isochronous USB transfer
type IsochronousTransfer struct {
	handle         *DeviceHandle
	endpoint       uint8
	packetSize     int
	numPackets     int
	buffer         []byte
	status         TransferStatus
	actualLength   int
	frameList      []C.IOUSBIsocFrame
	callback       func(*IsochronousTransfer)
	userData       interface{}
	submitted      bool
	completed      bool
	mutex          sync.Mutex
	packetLengths  []int
	packetStatuses []int
}

// NewIsochronousTransfer creates a new isochronous transfer
func NewIsochronousTransfer(handle *DeviceHandle, endpoint uint8, numPackets int, packetSize int) *IsochronousTransfer {
	totalSize := numPackets * packetSize
	frameList := make([]C.IOUSBIsocFrame, numPackets)

	// Initialize frame list
	for i := range frameList {
		frameList[i].frStatus = C.kIOReturnSuccess
		frameList[i].frReqCount = C.UInt16(packetSize)
		frameList[i].frActCount = 0
	}

	return &IsochronousTransfer{
		handle:         handle,
		endpoint:       endpoint,
		packetSize:     packetSize,
		numPackets:     numPackets,
		buffer:         make([]byte, totalSize),
		frameList:      frameList,
		packetLengths:  make([]int, numPackets),
		packetStatuses: make([]int, numPackets),
		status:         TransferError,
	}
}

// SetCallback sets the transfer callback
func (t *IsochronousTransfer) SetCallback(callback func(*IsochronousTransfer)) {
	t.callback = callback
}

// SetUserData sets user data for the transfer
func (t *IsochronousTransfer) SetUserData(data interface{}) {
	t.userData = data
}

// GetUserData gets the user data
func (t *IsochronousTransfer) GetUserData() interface{} {
	return t.userData
}

// SetPacketLength sets the length for a specific packet
func (t *IsochronousTransfer) SetPacketLength(packet int, length int) error {
	if packet < 0 || packet >= t.numPackets {
		return fmt.Errorf("packet index %d out of range", packet)
	}

	t.frameList[packet].frReqCount = C.UInt16(length)
	t.packetLengths[packet] = length
	return nil
}

// GetPacketData returns the data for a specific packet
func (t *IsochronousTransfer) GetPacketData(packet int) ([]byte, error) {
	if packet < 0 || packet >= t.numPackets {
		return nil, fmt.Errorf("packet index %d out of range", packet)
	}

	offset := packet * t.packetSize
	length := t.packetLengths[packet]
	if length == 0 {
		length = t.packetSize
	}

	end := offset + length
	if end > len(t.buffer) {
		end = len(t.buffer)
	}

	return t.buffer[offset:end], nil
}

// Submit submits the isochronous transfer
func (t *IsochronousTransfer) Submit() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.submitted {
		return fmt.Errorf("transfer already submitted")
	}

	if t.handle.closed {
		return fmt.Errorf("device is closed")
	}

	// Find the interface for this endpoint
	var intf *IOUSBInterfaceInterface
	for _, i := range t.handle.interfaces {
		intf = i
		break
	}

	if intf == nil {
		return fmt.Errorf("no interface claimed for endpoint %02x", t.endpoint)
	}

	// Get current bus frame number
	var frameNumber C.UInt64
	var atTime C.AbsoluteTime
	ret := C.GetBusFrameNumber(intf.ptr, &frameNumber, &atTime)
	if ret != kIOReturnSuccess {
		return fmt.Errorf("failed to get bus frame number: 0x%x", ret)
	}

	// Start a few frames in the future
	startFrame := frameNumber + 10

	pipeRef := t.endpoint & 0x0F

	// Submit the isochronous transfer
	if t.endpoint&0x80 != 0 {
		// IN transfer
		ret = C.ReadIsocPipe(intf.ptr,
			C.UInt8(pipeRef),
			unsafe.Pointer(&t.buffer[0]),
			C.UInt64(startFrame),
			C.UInt32(t.numPackets),
			&t.frameList[0])
	} else {
		// OUT transfer
		ret = C.WriteIsocPipe(intf.ptr,
			C.UInt8(pipeRef),
			unsafe.Pointer(&t.buffer[0]),
			C.UInt64(startFrame),
			C.UInt32(t.numPackets),
			&t.frameList[0])
	}

	if ret != kIOReturnSuccess {
		return fmt.Errorf("isochronous transfer failed: 0x%x", ret)
	}

	t.submitted = true

	// Since we're using sync API for now, mark as completed
	t.processCompletion()

	return nil
}

// processCompletion processes the completion of the transfer
func (t *IsochronousTransfer) processCompletion() {
	t.actualLength = 0
	allSuccess := true

	// Process frame results
	for i, frame := range t.frameList {
		t.packetStatuses[i] = int(frame.frStatus)
		actualCount := int(frame.frActCount)
		t.packetLengths[i] = actualCount
		t.actualLength += actualCount

		if frame.frStatus != C.kIOReturnSuccess {
			allSuccess = false
		}
	}

	if allSuccess {
		t.status = TransferCompleted
	} else {
		t.status = TransferError
	}

	t.completed = true

	if t.callback != nil {
		t.callback(t)
	}
}

// Cancel cancels the isochronous transfer
func (t *IsochronousTransfer) Cancel() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.submitted {
		return fmt.Errorf("transfer not submitted")
	}

	if t.completed {
		return nil
	}

	// Cancellation would require async API support
	t.status = TransferCancelled
	t.completed = true

	return nil
}

// Wait waits for the transfer to complete
func (t *IsochronousTransfer) Wait() error {
	// Since we're using sync API, transfer is already complete
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.submitted {
		return fmt.Errorf("transfer not submitted")
	}

	return nil
}

// Status returns the transfer status
func (t *IsochronousTransfer) Status() TransferStatus {
	return t.status
}

// ActualLength returns the total actual bytes transferred
func (t *IsochronousTransfer) ActualLength() int {
	return t.actualLength
}

// GetPacketStatus returns the status of a specific packet
func (t *IsochronousTransfer) GetPacketStatus(packet int) (int, error) {
	if packet < 0 || packet >= t.numPackets {
		return 0, fmt.Errorf("packet index %d out of range", packet)
	}
	return t.packetStatuses[packet], nil
}

// GetPacketActualLength returns the actual length transferred for a packet
func (t *IsochronousTransfer) GetPacketActualLength(packet int) (int, error) {
	if packet < 0 || packet >= t.numPackets {
		return 0, fmt.Errorf("packet index %d out of range", packet)
	}
	return t.packetLengths[packet], nil
}

// IsochronousTransferIn performs a synchronous isochronous IN transfer
func (h *DeviceHandle) IsochronousTransferIn(endpoint uint8, numPackets, packetSize int) (*IsochronousTransfer, error) {
	transfer := NewIsochronousTransfer(h, endpoint|0x80, numPackets, packetSize)
	err := transfer.Submit()
	if err != nil {
		return nil, err
	}
	return transfer, nil
}

// IsochronousTransferOut performs a synchronous isochronous OUT transfer
func (h *DeviceHandle) IsochronousTransferOut(endpoint uint8, data []byte, numPackets, packetSize int) (*IsochronousTransfer, error) {
	transfer := NewIsochronousTransfer(h, endpoint&0x7F, numPackets, packetSize)
	copy(transfer.buffer, data)
	err := transfer.Submit()
	if err != nil {
		return nil, err
	}
	return transfer, nil
}
