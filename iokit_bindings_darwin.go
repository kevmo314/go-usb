package usb

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <IOKit/usb/IOUSBLib.h>
#include <IOKit/IOCFPlugIn.h>
#include <CoreFoundation/CoreFoundation.h>
#include <mach/mach.h>

// USB device and interface IDs - use the ones from IOKit headers

// Helper function to get USB device interface
IOUSBDeviceInterface320** GetUSBDeviceInterface(io_service_t usbDevice) {
    IOCFPlugInInterface **plugInInterface = NULL;
    IOUSBDeviceInterface320 **deviceInterface = NULL;
    SInt32 score;
    kern_return_t kr;
    
    kr = IOCreatePlugInInterfaceForService(usbDevice,
                                          kIOUSBDeviceUserClientTypeID,
                                          kIOCFPlugInInterfaceID,
                                          &plugInInterface,
                                          &score);
    
    if (kr != 0 || !plugInInterface) {
        return NULL;
    }
    
    HRESULT result = (*plugInInterface)->QueryInterface(plugInInterface,
                                                       CFUUIDGetUUIDBytes(kIOUSBDeviceInterfaceID320),
                                                       (LPVOID *)&deviceInterface);
    
    (*plugInInterface)->Release(plugInInterface);
    
    if (result || !deviceInterface) {
        return NULL;
    }
    
    return deviceInterface;
}

// Helper function to get USB interface interface  
IOUSBInterfaceInterface300** GetUSBInterfaceInterface(io_service_t usbInterface) {
    IOCFPlugInInterface **plugInInterface = NULL;
    IOUSBInterfaceInterface300 **interfaceInterface = NULL;
    SInt32 score;
    kern_return_t kr;
    
    kr = IOCreatePlugInInterfaceForService(usbInterface,
                                          kIOUSBInterfaceUserClientTypeID,
                                          kIOCFPlugInInterfaceID,
                                          &plugInInterface,
                                          &score);
    
    if (kr != 0 || !plugInInterface) {
        return NULL;
    }
    
    HRESULT result = (*plugInInterface)->QueryInterface(plugInInterface,
                                                       CFUUIDGetUUIDBytes(kIOUSBInterfaceInterfaceID300),
                                                       (LPVOID *)&interfaceInterface);
    
    (*plugInInterface)->Release(plugInInterface);
    
    if (result || !interfaceInterface) {
        return NULL;
    }
    
    return interfaceInterface;
}

// Release device interface using COM Release
void ReleaseDeviceInterface(IOUSBDeviceInterface320 **deviceInterface) {
    if (deviceInterface && *deviceInterface) {
        (*deviceInterface)->Release(deviceInterface);
    }
}

// Release interface interface using COM Release
void ReleaseInterfaceInterface(IOUSBInterfaceInterface300 **interfaceInterface) {
    if (interfaceInterface && *interfaceInterface) {
        (*interfaceInterface)->Release(interfaceInterface);
    }
}

// Get location ID for a USB device
uint32_t GetLocationID(IOUSBDeviceInterface320 **deviceInterface) {
    UInt32 locationID = 0;
    (*deviceInterface)->GetLocationID(deviceInterface, &locationID);
    return locationID;
}

// Get device descriptor via control transfer
int GetDeviceDescriptor(IOUSBDeviceInterface320 **deviceInterface, IOUSBDeviceDescriptor *desc) {
    IOUSBDevRequest request;
    request.bmRequestType = 0x80; // Device-to-host, standard, device
    request.bRequest = 0x06;      // GET_DESCRIPTOR
    request.wValue = (0x01 << 8); // Device descriptor
    request.wIndex = 0;
    request.wLength = sizeof(IOUSBDeviceDescriptor);
    request.pData = desc;
    
    return (*deviceInterface)->DeviceRequest(deviceInterface, &request);
}

// Open device
int OpenDevice(IOUSBDeviceInterface320 **deviceInterface) {
    return (*deviceInterface)->USBDeviceOpen(deviceInterface);
}

// Close device
int CloseDevice(IOUSBDeviceInterface320 **deviceInterface) {
    return (*deviceInterface)->USBDeviceClose(deviceInterface);
}

// Set configuration
int SetConfiguration(IOUSBDeviceInterface320 **deviceInterface, UInt8 config) {
    return (*deviceInterface)->SetConfiguration(deviceInterface, config);
}

// Get configuration
int GetConfiguration(IOUSBDeviceInterface320 **deviceInterface, UInt8 *config) {
    return (*deviceInterface)->GetConfiguration(deviceInterface, config);
}

// Control transfer helper
int ControlTransfer(IOUSBDeviceInterface320 **deviceInterface, 
                   UInt8 bmRequestType,
                   UInt8 bRequest, 
                   UInt16 wValue,
                   UInt16 wIndex,
                   void *data,
                   UInt16 wLength,
                   UInt32 timeout) {
    IOUSBDevRequestTO request;
    request.bmRequestType = bmRequestType;
    request.bRequest = bRequest;
    request.wValue = wValue;
    request.wIndex = wIndex;
    request.wLength = wLength;
    request.pData = data;
    request.noDataTimeout = timeout;
    request.completionTimeout = timeout;
    
    return (*deviceInterface)->DeviceRequestTO(deviceInterface, &request);
}

// Reset device
int ResetDevice(IOUSBDeviceInterface320 **deviceInterface) {
    return (*deviceInterface)->ResetDevice(deviceInterface);
}

// String descriptor helper
int GetStringDescriptor(IOUSBDeviceInterface320 **deviceInterface,
                       UInt8 index,
                       UInt16 langID,
                       void *buf,
                       UInt16 maxLen,
                       UInt32 timeout) {
    IOUSBDevRequestTO request;
    request.bmRequestType = 0x80; // Device-to-host, standard, device
    request.bRequest = 0x06;      // GET_DESCRIPTOR
    request.wValue = (0x03 << 8) | index; // String descriptor type
    request.wIndex = langID;
    request.wLength = maxLen;
    request.pData = buf;
    request.noDataTimeout = timeout;
    request.completionTimeout = timeout;
    
    return (*deviceInterface)->DeviceRequestTO(deviceInterface, &request);
}

// Interface operations
int OpenInterface(IOUSBInterfaceInterface300 **interfaceInterface) {
    return (*interfaceInterface)->USBInterfaceOpen(interfaceInterface);
}

int CloseInterface(IOUSBInterfaceInterface300 **interfaceInterface) {
    return (*interfaceInterface)->USBInterfaceClose(interfaceInterface);
}

int GetInterfaceNumber(IOUSBInterfaceInterface300 **interfaceInterface, UInt8 *intfNum) {
    return (*interfaceInterface)->GetInterfaceNumber(interfaceInterface, intfNum);
}

int GetAlternateSetting(IOUSBInterfaceInterface300 **interfaceInterface, UInt8 *alternate) {
    return (*interfaceInterface)->GetAlternateSetting(interfaceInterface, alternate);
}

int SetAlternateSetting(IOUSBInterfaceInterface300 **interfaceInterface, UInt8 alternate) {
    return (*interfaceInterface)->SetAlternateInterface(interfaceInterface, alternate);
}

int GetNumEndpoints(IOUSBInterfaceInterface300 **interfaceInterface, UInt8 *numEndpoints) {
    return (*interfaceInterface)->GetNumEndpoints(interfaceInterface, numEndpoints);
}

// Bulk transfer
int BulkTransfer(IOUSBInterfaceInterface300 **interfaceInterface,
                UInt8 pipeRef,
                void *buf,
                UInt32 *size,
                UInt32 timeout) {
    if (timeout == 0) {
        return (*interfaceInterface)->WritePipe(interfaceInterface, pipeRef, buf, *size);
    } else {
        return (*interfaceInterface)->WritePipeTO(interfaceInterface, pipeRef, buf, *size, timeout, timeout);
    }
}

int BulkTransferRead(IOUSBInterfaceInterface300 **interfaceInterface,
                    UInt8 pipeRef,
                    void *buf,
                    UInt32 *size,
                    UInt32 timeout) {
    if (timeout == 0) {
        return (*interfaceInterface)->ReadPipe(interfaceInterface, pipeRef, buf, size);
    } else {
        return (*interfaceInterface)->ReadPipeTO(interfaceInterface, pipeRef, buf, size, timeout, timeout);
    }
}

// Async transfer support
typedef struct {
    void *buffer;
    UInt32 size;
    IOReturn status;
    void *userData;
    void (*callback)(void *userData, IOReturn result, void *arg0);
} AsyncTransferContext;

void AsyncCallback(void *refCon, IOReturn result, void *arg0) {
    AsyncTransferContext *ctx = (AsyncTransferContext *)refCon;
    ctx->status = result;
    if (ctx->callback) {
        ctx->callback(ctx->userData, result, arg0);
    }
}

// Async bulk transfer
int BulkTransferAsync(IOUSBInterfaceInterface300 **interfaceInterface,
                     UInt8 pipeRef,
                     void *buf,
                     UInt32 size,
                     void *context) {
    AsyncTransferContext *ctx = (AsyncTransferContext *)context;
    return (*interfaceInterface)->WritePipeAsync(interfaceInterface, pipeRef, buf, size, 
                                                 AsyncCallback, context);
}

int BulkTransferReadAsync(IOUSBInterfaceInterface300 **interfaceInterface,
                         UInt8 pipeRef,
                         void *buf,
                         UInt32 size,
                         void *context) {
    AsyncTransferContext *ctx = (AsyncTransferContext *)context;
    return (*interfaceInterface)->ReadPipeAsync(interfaceInterface, pipeRef, buf, size,
                                                AsyncCallback, context);
}

// Create run loop source for interface
CFRunLoopSourceRef CreateInterfaceAsyncEventSource(IOUSBInterfaceInterface300 **interfaceInterface) {
    CFRunLoopSourceRef source = NULL;
    (*interfaceInterface)->CreateInterfaceAsyncEventSource(interfaceInterface, &source);
    return source;
}

// Create run loop source for device  
CFRunLoopSourceRef CreateDeviceAsyncEventSource(IOUSBDeviceInterface320 **deviceInterface) {
    CFRunLoopSourceRef source = NULL;
    (*deviceInterface)->CreateDeviceAsyncEventSource(deviceInterface, &source);
    return source;
}

// Clear endpoint halt
int ClearPipeStall(IOUSBInterfaceInterface300 **interfaceInterface, UInt8 pipeRef) {
    return (*interfaceInterface)->ClearPipeStall(interfaceInterface, pipeRef);
}

// Get pipe properties
int GetPipeProperties(IOUSBInterfaceInterface300 **interfaceInterface,
                      UInt8 pipeRef,
                      UInt8 *direction,
                      UInt8 *number,
                      UInt8 *transferType,
                      UInt16 *maxPacketSize,
                      UInt8 *interval) {
    return (*interfaceInterface)->GetPipeProperties(interfaceInterface, pipeRef,
                                                   direction, number, transferType,
                                                   maxPacketSize, interval);
}

*/
import "C"

import (
	"fmt"
	"unsafe"
)

// IOKit constants
const (
	kIOReturnSuccess = 0
	kIOUSBPipeStalled = int32(-536870897)
	kIOReturnNotResponding = int32(-536870906)
	kIOReturnNoDevice = int32(-536870208)
	kIOReturnExclusiveAccess = int32(-536870203)
	kIOUSBTransactionTimeout = int32(-536870899)
)

// IOUSBDeviceInterface wraps the C IOUSBDeviceInterface320
type IOUSBDeviceInterface struct {
	ptr **C.IOUSBDeviceInterface320
}

// IOUSBInterfaceInterface wraps the C IOUSBInterfaceInterface300  
type IOUSBInterfaceInterface struct {
	ptr **C.IOUSBInterfaceInterface300
}

// GetUSBDeviceInterface creates a device interface from an io_service_t
func GetUSBDeviceInterface(device C.io_service_t) (*IOUSBDeviceInterface, error) {
	devInterface := C.GetUSBDeviceInterface(device)
	if devInterface == nil {
		return nil, fmt.Errorf("failed to get USB device interface")
	}
	return &IOUSBDeviceInterface{ptr: devInterface}, nil
}

// GetUSBInterfaceInterface creates an interface interface from an io_service_t
func GetUSBInterfaceInterface(intf C.io_service_t) (*IOUSBInterfaceInterface, error) {
	intfInterface := C.GetUSBInterfaceInterface(intf)
	if intfInterface == nil {
		return nil, fmt.Errorf("failed to get USB interface interface")
	}
	return &IOUSBInterfaceInterface{ptr: intfInterface}, nil
}

// Release releases the device interface
func (d *IOUSBDeviceInterface) Release() {
	if d.ptr != nil && *d.ptr != nil {
		C.ReleaseDeviceInterface(d.ptr)
		d.ptr = nil
	}
}

// Release releases the interface interface
func (i *IOUSBInterfaceInterface) Release() {
	if i.ptr != nil && *i.ptr != nil {
		C.ReleaseInterfaceInterface(i.ptr)
		i.ptr = nil
	}
}

// Open opens the device
func (d *IOUSBDeviceInterface) Open() error {
	ret := C.OpenDevice(d.ptr)
	if ret != kIOReturnSuccess {
		return fmt.Errorf("failed to open device: 0x%x", ret)
	}
	return nil
}

// Close closes the device
func (d *IOUSBDeviceInterface) Close() error {
	ret := C.CloseDevice(d.ptr)
	if ret != kIOReturnSuccess {
		return fmt.Errorf("failed to close device: 0x%x", ret)
	}
	return nil
}

// SetConfiguration sets the device configuration
func (d *IOUSBDeviceInterface) SetConfiguration(config uint8) error {
	ret := C.SetConfiguration(d.ptr, C.UInt8(config))
	if ret != kIOReturnSuccess {
		return fmt.Errorf("failed to set configuration: 0x%x", ret)
	}
	return nil
}

// GetConfiguration gets the current device configuration
func (d *IOUSBDeviceInterface) GetConfiguration() (uint8, error) {
	var config C.UInt8
	ret := C.GetConfiguration(d.ptr, &config)
	if ret != kIOReturnSuccess {
		return 0, fmt.Errorf("failed to get configuration: 0x%x", ret)
	}
	return uint8(config), nil
}

// GetLocationID returns the device's location ID
func (d *IOUSBDeviceInterface) GetLocationID() uint32 {
	return uint32(C.GetLocationID(d.ptr))
}

// GetDeviceDescriptor retrieves the device descriptor
func (d *IOUSBDeviceInterface) GetDeviceDescriptor() (*DeviceDescriptor, error) {
	var desc C.IOUSBDeviceDescriptor
	ret := C.GetDeviceDescriptor(d.ptr, &desc)
	if ret != kIOReturnSuccess {
		return nil, fmt.Errorf("failed to get device descriptor: 0x%x", ret)
	}
	
	return &DeviceDescriptor{
		Length:            uint8(desc.bLength),
		DescriptorType:    uint8(desc.bDescriptorType),
		USBVersion:        uint16(desc.bcdUSB),
		DeviceClass:       uint8(desc.bDeviceClass),
		DeviceSubClass:    uint8(desc.bDeviceSubClass),
		DeviceProtocol:    uint8(desc.bDeviceProtocol),
		MaxPacketSize0:    uint8(desc.bMaxPacketSize0),
		VendorID:          uint16(desc.idVendor),
		ProductID:         uint16(desc.idProduct),
		DeviceVersion:     uint16(desc.bcdDevice),
		ManufacturerIndex: uint8(desc.iManufacturer),
		ProductIndex:      uint8(desc.iProduct),
		SerialNumberIndex: uint8(desc.iSerialNumber),
		NumConfigurations: uint8(desc.bNumConfigurations),
	}, nil
}

// ControlTransfer performs a control transfer
func (d *IOUSBDeviceInterface) ControlTransfer(bmRequestType, bRequest uint8, wValue, wIndex uint16, data []byte, timeout uint32) (int, error) {
	var ptr unsafe.Pointer
	if len(data) > 0 {
		ptr = unsafe.Pointer(&data[0])
	}
	
	ret := C.ControlTransfer(d.ptr,
		C.UInt8(bmRequestType),
		C.UInt8(bRequest),
		C.UInt16(wValue),
		C.UInt16(wIndex),
		ptr,
		C.UInt16(len(data)),
		C.UInt32(timeout))
	
	if ret != kIOReturnSuccess {
		return 0, fmt.Errorf("control transfer failed: 0x%x", ret)
	}
	
	return len(data), nil
}

// ResetDevice resets the USB device
func (d *IOUSBDeviceInterface) ResetDevice() error {
	ret := C.ResetDevice(d.ptr)
	if ret != kIOReturnSuccess {
		return fmt.Errorf("failed to reset device: 0x%x", ret)
	}
	return nil
}

// GetStringDescriptor retrieves a string descriptor
func (d *IOUSBDeviceInterface) GetStringDescriptor(index uint8, langID uint16) (string, error) {
	buf := make([]byte, 256)
	ret := C.GetStringDescriptor(d.ptr, C.UInt8(index), C.UInt16(langID), 
		unsafe.Pointer(&buf[0]), C.UInt16(len(buf)), C.UInt32(5000))
	
	if ret != kIOReturnSuccess {
		return "", fmt.Errorf("failed to get string descriptor: 0x%x", ret)
	}
	
	// Parse USB string descriptor format
	if len(buf) < 2 {
		return "", fmt.Errorf("invalid string descriptor")
	}
	
	length := int(buf[0])
	if length < 2 || length > len(buf) {
		return "", fmt.Errorf("invalid string descriptor length")
	}
	
	// USB strings are UTF-16LE, starting at byte 2
	if length <= 2 {
		return "", nil
	}
	
	// Convert UTF-16LE to string
	strBytes := buf[2:length]
	runes := make([]rune, 0, len(strBytes)/2)
	for i := 0; i < len(strBytes)-1; i += 2 {
		r := rune(strBytes[i]) | (rune(strBytes[i+1]) << 8)
		if r != 0 {
			runes = append(runes, r)
		}
	}
	
	return string(runes), nil
}

// Interface operations

// Open opens the interface
func (i *IOUSBInterfaceInterface) Open() error {
	ret := C.OpenInterface(i.ptr)
	if ret != kIOReturnSuccess {
		return fmt.Errorf("failed to open interface: 0x%x", ret)
	}
	return nil
}

// Close closes the interface
func (i *IOUSBInterfaceInterface) Close() error {
	ret := C.CloseInterface(i.ptr)
	if ret != kIOReturnSuccess {
		return fmt.Errorf("failed to close interface: 0x%x", ret)
	}
	return nil
}

// SetAlternateSetting sets the alternate setting for the interface
func (i *IOUSBInterfaceInterface) SetAlternateSetting(altSetting uint8) error {
	ret := C.SetAlternateSetting(i.ptr, C.UInt8(altSetting))
	if ret != kIOReturnSuccess {
		return fmt.Errorf("failed to set alternate setting: 0x%x", ret)
	}
	return nil
}

// ClearPipeStall clears a stall condition on an endpoint
func (i *IOUSBInterfaceInterface) ClearPipeStall(pipeRef uint8) error {
	ret := C.ClearPipeStall(i.ptr, C.UInt8(pipeRef))
	if ret != kIOReturnSuccess {
		return fmt.Errorf("failed to clear pipe stall: 0x%x", ret)
	}
	return nil
}

// BulkTransferOut performs a bulk OUT transfer
func (i *IOUSBInterfaceInterface) BulkTransferOut(pipeRef uint8, data []byte, timeout uint32) (int, error) {
	size := C.UInt32(len(data))
	ret := C.BulkTransfer(i.ptr, C.UInt8(pipeRef), unsafe.Pointer(&data[0]), &size, C.UInt32(timeout))
	
	if ret != kIOReturnSuccess {
		if int32(ret) == kIOUSBPipeStalled {
			return int(size), fmt.Errorf("pipe stalled")
		}
		if int32(ret) == kIOUSBTransactionTimeout {
			return int(size), ErrTimeout
		}
		return int(size), fmt.Errorf("bulk transfer failed: 0x%x", ret)
	}
	
	return int(size), nil
}

// BulkTransferIn performs a bulk IN transfer
func (i *IOUSBInterfaceInterface) BulkTransferIn(pipeRef uint8, data []byte, timeout uint32) (int, error) {
	size := C.UInt32(len(data))
	ret := C.BulkTransferRead(i.ptr, C.UInt8(pipeRef), unsafe.Pointer(&data[0]), &size, C.UInt32(timeout))
	
	if ret != kIOReturnSuccess {
		if int32(ret) == kIOUSBPipeStalled {
			return int(size), fmt.Errorf("pipe stalled")
		}
		if int32(ret) == kIOUSBTransactionTimeout {
			return int(size), ErrTimeout
		}
		return int(size), fmt.Errorf("bulk transfer failed: 0x%x", ret)
	}
	
	return int(size), nil
}

// AsyncTransferContext wraps the C async transfer context
type AsyncTransferContext struct {
	Buffer   []byte
	Size     uint32
	Status   int32
	Callback func(result int32, bytesTransferred uint32)
	cContext *C.AsyncTransferContext
}

// BulkTransferOutAsync performs an async bulk OUT transfer
func (i *IOUSBInterfaceInterface) BulkTransferOutAsync(pipeRef uint8, data []byte, callback func(result int32, bytesTransferred uint32)) error {
	ctx := &AsyncTransferContext{
		Buffer:   data,
		Size:     uint32(len(data)),
		Callback: callback,
	}
	
	// Allocate C context
	ctx.cContext = (*C.AsyncTransferContext)(C.malloc(C.sizeof_AsyncTransferContext))
	ctx.cContext.buffer = unsafe.Pointer(&data[0])
	ctx.cContext.size = C.UInt32(len(data))
	ctx.cContext.userData = unsafe.Pointer(ctx)
	
	ret := C.BulkTransferAsync(i.ptr, C.UInt8(pipeRef), unsafe.Pointer(&data[0]), 
		C.UInt32(len(data)), unsafe.Pointer(ctx.cContext))
	
	if ret != kIOReturnSuccess {
		C.free(unsafe.Pointer(ctx.cContext))
		return fmt.Errorf("async bulk transfer failed: 0x%x", ret)
	}
	
	return nil
}

// BulkTransferInAsync performs an async bulk IN transfer  
func (i *IOUSBInterfaceInterface) BulkTransferInAsync(pipeRef uint8, data []byte, callback func(result int32, bytesTransferred uint32)) error {
	ctx := &AsyncTransferContext{
		Buffer:   data,
		Size:     uint32(len(data)),
		Callback: callback,
	}
	
	// Allocate C context
	ctx.cContext = (*C.AsyncTransferContext)(C.malloc(C.sizeof_AsyncTransferContext))
	ctx.cContext.buffer = unsafe.Pointer(&data[0])
	ctx.cContext.size = C.UInt32(len(data))
	ctx.cContext.userData = unsafe.Pointer(ctx)
	
	ret := C.BulkTransferReadAsync(i.ptr, C.UInt8(pipeRef), unsafe.Pointer(&data[0]),
		C.UInt32(len(data)), unsafe.Pointer(ctx.cContext))
	
	if ret != kIOReturnSuccess {
		C.free(unsafe.Pointer(ctx.cContext))
		return fmt.Errorf("async bulk transfer failed: 0x%x", ret)
	}
	
	return nil
}

// CreateAsyncEventSource creates a run loop source for async events
func (i *IOUSBInterfaceInterface) CreateAsyncEventSource() (C.CFRunLoopSourceRef, error) {
	source := C.CreateInterfaceAsyncEventSource(i.ptr)
	if source == 0 {
		return 0, fmt.Errorf("failed to create async event source")
	}
	return source, nil
}

// CreateDeviceAsyncEventSource creates a run loop source for device async events
func (d *IOUSBDeviceInterface) CreateAsyncEventSource() (C.CFRunLoopSourceRef, error) {
	source := C.CreateDeviceAsyncEventSource(d.ptr)
	if source == 0 {
		return 0, fmt.Errorf("failed to create device async event source")
	}
	return source, nil
}