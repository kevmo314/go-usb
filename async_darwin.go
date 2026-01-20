package usb

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>

void RunLoopRunWithTimeout(double seconds) {
    CFRunLoopRunInMode(kCFRunLoopDefaultMode, seconds, true);
}

void AddSourceToRunLoop(CFRunLoopSourceRef source) {
    CFRunLoopAddSource(CFRunLoopGetCurrent(), source, kCFRunLoopDefaultMode);
}

void RemoveSourceFromRunLoop(CFRunLoopSourceRef source) {
    CFRunLoopRemoveSource(CFRunLoopGetCurrent(), source, kCFRunLoopDefaultMode);
}
*/
import "C"

import (
	"fmt"
	"sync"
	"time"
)

// AsyncTransfer represents an asynchronous USB transfer on macOS
type AsyncTransfer struct {
	*Transfer
	handle    *DeviceHandle
	submitted bool
	completed bool
	mutex     sync.Mutex
}

// NewAsyncTransfer creates a new async transfer
func NewAsyncTransfer(handle *DeviceHandle, endpoint uint8, transferType TransferType, bufferSize int) *AsyncTransfer {
	return &AsyncTransfer{
		Transfer: &Transfer{
			handle:       handle,
			endpoint:     endpoint,
			transferType: transferType,
			buffer:       make([]byte, bufferSize),
			status:       TransferError,
		},
		handle: handle,
	}
}

// Submit submits the async transfer
func (t *AsyncTransfer) Submit() error {
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

	// Create async event source if needed
	if t.handle.asyncSource == 0 {
		source, err := intf.CreateAsyncEventSource()
		if err != nil {
			return err
		}
		t.handle.asyncSource = source
		C.AddSourceToRunLoop(source)
	}

	// Submit the async transfer
	callback := func(result int32, bytesTransferred uint32) {
		t.mutex.Lock()
		defer t.mutex.Unlock()

		t.actualLength = int(bytesTransferred)
		if result == kIOReturnSuccess {
			t.status = TransferCompleted
		} else if result == int32(kIOUSBTransactionTimeout) {
			t.status = TransferTimedOut
		} else {
			t.status = TransferError
		}
		t.completed = true

		if t.callback != nil {
			t.callback(t.Transfer)
		}
	}

	var err error
	pipeRef := t.endpoint & 0x0F

	if t.endpoint&0x80 != 0 {
		// IN transfer
		err = intf.BulkTransferInAsync(pipeRef, t.buffer, callback)
	} else {
		// OUT transfer
		err = intf.BulkTransferOutAsync(pipeRef, t.buffer, callback)
	}

	if err != nil {
		return err
	}

	t.submitted = true
	return nil
}

// Wait waits for the transfer to complete
func (t *AsyncTransfer) Wait(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		t.mutex.Lock()
		if t.completed {
			t.mutex.Unlock()
			return nil
		}
		t.mutex.Unlock()

		if time.Now().After(deadline) {
			return ErrTimeout
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// Cancel cancels the async transfer
func (t *AsyncTransfer) Cancel() error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.submitted {
		return fmt.Errorf("transfer not submitted")
	}

	if t.completed {
		return nil
	}

	// Note: Proper cancellation would require IOKit async API support
	t.status = TransferCancelled
	t.completed = true

	return nil
}

// IsCompleted checks if the transfer is completed
func (t *AsyncTransfer) IsCompleted() bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	return t.completed
}

// AsyncBulkTransfer performs an asynchronous bulk transfer
func (h *DeviceHandle) AsyncBulkTransfer(endpoint uint8, data []byte, callback func(*Transfer)) error {
	transfer := NewAsyncTransfer(h, endpoint, TransferTypeBulk, len(data))
	copy(transfer.buffer, data)
	transfer.SetCallback(callback)

	return transfer.Submit()
}

// AsyncInterruptTransfer performs an asynchronous interrupt transfer
func (h *DeviceHandle) AsyncInterruptTransfer(endpoint uint8, data []byte, callback func(*Transfer)) error {
	transfer := NewAsyncTransfer(h, endpoint, TransferTypeInterrupt, len(data))
	copy(transfer.buffer, data)
	transfer.SetCallback(callback)

	return transfer.Submit()
}

// HandleEvents processes pending USB events
func HandleEvents(timeout time.Duration) error {
	// Run the CFRunLoop to process async events
	seconds := timeout.Seconds()
	if seconds <= 0 {
		seconds = 0.001 // Minimum timeout
	}

	C.RunLoopRunWithTimeout(C.double(seconds))
	return nil
}

// RunEventLoop runs the event loop in a separate goroutine
func RunEventLoop(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
			HandleEvents(10 * time.Millisecond)
		}
	}
}

// Note: A full implementation would require:
// 1. Integration with CFRunLoop for proper async event handling
// 2. Use of IOUSBInterfaceInterface's async methods (ReadPipeAsync, WritePipeAsync)
// 3. Proper callback registration with IOKit
// 4. Thread-safe transfer queue management
