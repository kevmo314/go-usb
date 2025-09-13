package usb

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const (
	USBDEVFS_CONTROL          = 0xc0185500
	USBDEVFS_BULK             = 0xc0185502
	USBDEVFS_CLAIMINTERFACE   = 0x8004550f
	USBDEVFS_RELEASEINTERFACE = 0x80045510
	USBDEVFS_SETINTERFACE     = 0x80085504
	USBDEVFS_CLEAR_HALT       = 0x80045515
	USBDEVFS_RESETEP          = 0x80045503
	USBDEVFS_SETCONFIGURATION = 0x80045505
	USBDEVFS_GETDRIVER        = 0x41045508
	USBDEVFS_SUBMITURB        = 0x8038550a
	USBDEVFS_DISCARDURB       = 0x0000550b
	USBDEVFS_REAPURB          = 0x4008550c
	USBDEVFS_REAPURBNDELAY    = 0x4008550d
	USBDEVFS_DISCONNECT       = 0x00005516
	USBDEVFS_CONNECT          = 0x00005517
	USBDEVFS_DISCONNECT_CLAIM = 0x8108551b
	USBDEVFS_IOCTL            = 0xc0105512
	USBDEVFS_GET_CAPABILITIES = 0x8004551a
	USBDEVFS_ALLOC_STREAMS    = 0x8008551c
	USBDEVFS_FREE_STREAMS     = 0x8008551d
	USBDEVFS_GET_SPEED        = 0x8004551f
)

// USB descriptor types
const (
	USB_DT_DEVICE                       = 0x01
	USB_DT_CONFIG                       = 0x02
	USB_DT_STRING                       = 0x03
	USB_DT_INTERFACE                    = 0x04
	USB_DT_ENDPOINT                     = 0x05
	USB_DT_DEVICE_QUALIFIER             = 0x06
	USB_DT_OTHER_SPEED_CONFIG           = 0x07
	USB_DT_INTERFACE_POWER              = 0x08
	USB_DT_OTG                          = 0x09
	USB_DT_DEBUG                        = 0x0a
	USB_DT_INTERFACE_ASSOCIATION        = 0x0b
	USB_DT_BOS                          = 0x0f
	USB_DT_DEVICE_CAPABILITY            = 0x10
	USB_DT_SS_ENDPOINT_COMPANION        = 0x30
	USB_DT_SUPERSPEEDPLUS_ISOCH_EP_COMP = 0x31
)

// USB feature selectors
const (
	USB_DEVICE_SELF_POWERED      = 0
	USB_DEVICE_REMOTE_WAKEUP     = 1
	USB_DEVICE_TEST_MODE         = 2
	USB_DEVICE_BATTERY           = 2
	USB_DEVICE_B_HNP_ENABLE      = 3
	USB_DEVICE_WUSB_DEVICE       = 3
	USB_DEVICE_A_HNP_SUPPORT     = 4
	USB_DEVICE_A_ALT_HNP_SUPPORT = 5
	USB_DEVICE_DEBUG_MODE        = 6
)

// USB standard requests
const (
	USB_REQ_GET_STATUS        = 0x00
	USB_REQ_CLEAR_FEATURE     = 0x01
	USB_REQ_SET_FEATURE       = 0x03
	USB_REQ_SET_ADDRESS       = 0x05
	USB_REQ_GET_DESCRIPTOR    = 0x06
	USB_REQ_SET_DESCRIPTOR    = 0x07
	USB_REQ_GET_CONFIGURATION = 0x08
	USB_REQ_SET_CONFIGURATION = 0x09
	USB_REQ_GET_INTERFACE     = 0x0A
	USB_REQ_SET_INTERFACE     = 0x0B
	USB_REQ_SYNCH_FRAME       = 0x0C
)

type DeviceDescriptor struct {
	Length            uint8
	DescriptorType    uint8
	USBVersion        uint16
	DeviceClass       uint8
	DeviceSubClass    uint8
	DeviceProtocol    uint8
	MaxPacketSize0    uint8
	VendorID          uint16
	ProductID         uint16
	DeviceVersion     uint16
	ManufacturerIndex uint8
	ProductIndex      uint8
	SerialNumberIndex uint8
	NumConfigurations uint8
}

type RawConfigDescriptor struct {
	Length             uint8
	DescriptorType     uint8
	TotalLength        uint16
	NumInterfaces      uint8
	ConfigurationValue uint8
	ConfigurationIndex uint8
	Attributes         uint8
	MaxPower           uint8
}

type InterfaceDescriptor struct {
	Length            uint8
	DescriptorType    uint8
	InterfaceNumber   uint8
	AlternateSetting  uint8
	NumEndpoints      uint8
	InterfaceClass    uint8
	InterfaceSubClass uint8
	InterfaceProtocol uint8
	InterfaceIndex    uint8
}

type EndpointDescriptor struct {
	Length         uint8
	DescriptorType uint8
	EndpointAddr   uint8
	Attributes     uint8
	MaxPacketSize  uint16
	Interval       uint8
}

// USB 3.0+ SuperSpeed Endpoint Companion Descriptor
type SuperSpeedEndpointCompanionDescriptor struct {
	Length           uint8
	DescriptorType   uint8 // USB_DT_SS_ENDPOINT_COMP
	MaxBurst         uint8
	Attributes       uint8
	BytesPerInterval uint16
}

// Interface Association Descriptor (IAD)
type InterfaceAssocDescriptor struct {
	Length           uint8
	DescriptorType   uint8 // USB_DT_INTERFACE_ASSOC
	FirstInterface   uint8
	InterfaceCount   uint8
	FunctionClass    uint8
	FunctionSubClass uint8
	FunctionProtocol uint8
	Function         uint8
}

// Binary Object Store (BOS) Descriptor
type BOSDescriptor struct {
	Length         uint8
	DescriptorType uint8 // USB_DT_BOS
	TotalLength    uint16
	NumDeviceCaps  uint8
}

// Device Capability Descriptor (part of BOS)
type DeviceCapabilityDescriptor struct {
	Length            uint8
	DescriptorType    uint8 // USB_DT_DEVICE_CAPABILITY
	DevCapabilityType uint8
	// Capability-specific data follows
}

// USB 2.0 Extension Capability
type USB2ExtensionCapability struct {
	Length            uint8
	DescriptorType    uint8
	DevCapabilityType uint8 // 0x02
	Attributes        uint32
}

// SuperSpeed USB Capability
type SuperSpeedUSBCapability struct {
	Length                 uint8
	DescriptorType         uint8
	DevCapabilityType      uint8 // 0x03
	Attributes             uint8
	SpeedsSupported        uint16
	FunctionalitySupported uint8
	U1DevExitLat           uint8
	U2DevExitLat           uint16
}

// OTG Descriptor
type OTGDescriptor struct {
	Length         uint8
	DescriptorType uint8 // USB_DT_OTG
	Attributes     uint8
}

// Device Qualifier Descriptor
type DeviceQualifierDescriptor struct {
	Length            uint8
	DescriptorType    uint8 // USB_DT_DEVICE_QUALIFIER
	USBVersion        uint16
	DeviceClass       uint8
	DeviceSubClass    uint8
	DeviceProtocol    uint8
	MaxPacketSize0    uint8
	NumConfigurations uint8
	Reserved          uint8
}

type Device struct {
	Path         string
	Bus          uint8
	Address      uint8
	Descriptor   DeviceDescriptor
	Configs      []RawConfigDescriptor
	sysfsStrings *SysfsStrings

	handle *DeviceHandle
	mu     sync.RWMutex
}

// SysfsStrings holds cached sysfs string descriptors
type SysfsStrings struct {
	Manufacturer string
	Product      string
	Serial       string
}

type DeviceHandle struct {
	device        *Device
	fd            int
	claimedIfaces map[uint8]bool
	mu            sync.RWMutex
	closed        bool

	// Reaper state for isochronous transfers
	reapMutex sync.Mutex
	reapMap   map[uintptr]func(error) // URB ptr -> completion callback
	reaping   bool                    // Is reaper running?
}

func (d *Device) Open() (*DeviceHandle, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.handle != nil && !d.handle.closed {
		return nil, ErrDeviceBusy
	}

	fd, err := syscall.Open(d.Path, syscall.O_RDWR, 0)
	if err != nil {
		if err == syscall.EACCES {
			return nil, ErrPermissionDenied
		}
		return nil, fmt.Errorf("failed to open device: %w", err)
	}

	handle := &DeviceHandle{
		device:        d,
		fd:            fd,
		claimedIfaces: make(map[uint8]bool),
		closed:        false,
		reapMap:       make(map[uintptr]func(error)),
	}

	d.handle = handle
	return handle, nil
}

func (h *DeviceHandle) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}

	for iface := range h.claimedIfaces {
		h.releaseInterfaceInternal(iface)
	}

	err := syscall.Close(h.fd)
	h.closed = true
	h.device.handle = nil

	return err
}

// registerURBCompletion registers a URB for completion notification
func (h *DeviceHandle) registerURBCompletion(urbPtr uintptr, callback func(error)) {
	h.reapMutex.Lock()
	defer h.reapMutex.Unlock()

	// Register the callback
	h.reapMap[urbPtr] = callback

	// Start reaper if not already running
	if !h.reaping {
		h.reaping = true
		go h.reapLoop()
	}
}

// reapLoop continuously reaps completed URBs and notifies waiting transfers
func (h *DeviceHandle) reapLoop() {
	for {
		// Check if handle is closed
		h.mu.RLock()
		closed := h.closed
		h.mu.RUnlock()

		if closed {
			// Notify all pending transfers that we're closing
			h.reapMutex.Lock()
			for _, callback := range h.reapMap {
				callback(ErrDeviceNotFound)
			}
			h.reapMap = nil
			h.reaping = false
			h.reapMutex.Unlock()
			return
		}

		// Wait for URB completion using REAPURB ioctl
		var reapedURB *URB

		_, _, errno := syscall.Syscall(
			syscall.SYS_IOCTL,
			uintptr(h.fd),
			USBDEVFS_REAPURB,
			uintptr(unsafe.Pointer(&reapedURB)),
		)
		if errno == syscall.EINTR || errno == syscall.EAGAIN {
			continue
		} else if errno != 0 {
			h.reapMutex.Lock()
			for _, callback := range h.reapMap {
				callback(fmt.Errorf("reaper failed: %v", errno))
			}
			h.reapMap = make(map[uintptr]func(error))
			h.reaping = false
			h.reapMutex.Unlock()
			return
		}

		// Find the callback for this URB
		h.reapMutex.Lock()
		callback, ok := h.reapMap[uintptr(unsafe.Pointer(reapedURB))]
		if !ok {
			panic("reapLoop: no callback for reaped URB")
		}
		delete(h.reapMap, uintptr(unsafe.Pointer(reapedURB)))
		h.reapMutex.Unlock()

		// Call the callback with the URB status
		var err error
		if reapedURB.Status != 0 {
			err = fmt.Errorf("URB completed with status: %d", reapedURB.Status)
		}
		callback(err)
	}
}

func (h *DeviceHandle) GetDescriptor() DeviceDescriptor {
	return h.device.Descriptor
}

func (h *DeviceHandle) GetConfiguration() (int, error) {
	buf := make([]byte, 1)

	ctrl := usbCtrlRequest{
		RequestType: 0x80,
		Request:     0x08,
		Value:       0,
		Index:       0,
		Length:      uint16(len(buf)),
		Data:        unsafe.Pointer(&buf[0]),
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return 0, errno
	}

	return int(buf[0]), nil
}

func (h *DeviceHandle) SetConfiguration(config int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	cfg := uint32(config)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_SETCONFIGURATION, uintptr(unsafe.Pointer(&cfg)))
	if errno != 0 {
		return errno
	}

	return nil
}

// GetConfigDescriptorByValue gets the parsed configuration descriptor by index
// This is equivalent to libusb_get_config_descriptor
func (h *DeviceHandle) GetConfigDescriptorByValue(index uint8) (*ConfigDescriptor, error) {
	data, err := h.GetRawConfigDescriptor(index)
	if err != nil {
		return nil, err
	}

	config := &ConfigDescriptor{}
	err = config.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return config, nil
}

// GetRawConfigDescriptor gets the raw configuration descriptor data by index
func (h *DeviceHandle) GetRawConfigDescriptor(index uint8) ([]byte, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, ErrDeviceNotFound
	}

	// First get the config descriptor header to know the total length
	buf := make([]byte, 9)

	ctrl := usbCtrlRequest{
		RequestType: 0x80, // Device to host
		Request:     USB_REQ_GET_DESCRIPTOR,
		Value:       (USB_DT_CONFIG << 8) | uint16(index),
		Index:       0,
		Length:      9,
		Data:        unsafe.Pointer(&buf[0]),
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return nil, fmt.Errorf("failed to get config descriptor header: %w", errno)
	}

	// Parse total length from the header
	totalLength := binary.LittleEndian.Uint16(buf[2:4])

	// Now get the full descriptor
	fullBuf := make([]byte, totalLength)
	ctrl.Length = totalLength
	ctrl.Data = unsafe.Pointer(&fullBuf[0])

	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return nil, fmt.Errorf("failed to get full config descriptor: %w", errno)
	}

	return fullBuf, nil
}

func (h *DeviceHandle) ClaimInterface(iface uint8) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	if h.claimedIfaces[iface] {
		return nil
	}

	ifaceNum := uint32(iface)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CLAIMINTERFACE, uintptr(unsafe.Pointer(&ifaceNum)))
	if errno != 0 {
		return errno
	}

	h.claimedIfaces[iface] = true
	return nil
}

func (h *DeviceHandle) ReleaseInterface(iface uint8) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	return h.releaseInterfaceInternal(iface)
}

func (h *DeviceHandle) releaseInterfaceInternal(iface uint8) error {
	if !h.claimedIfaces[iface] {
		return nil
	}

	ifaceNum := uint32(iface)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_RELEASEINTERFACE, uintptr(unsafe.Pointer(&ifaceNum)))
	if errno != 0 {
		return errno
	}

	delete(h.claimedIfaces, iface)
	return nil
}

func (h *DeviceHandle) SetInterfaceAltSetting(iface uint8, altSetting uint8) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	if !h.claimedIfaces[iface] {
		return fmt.Errorf("interface %d not claimed", iface)
	}

	setIface := struct {
		Interface  uint32
		AltSetting uint32
	}{
		Interface:  uint32(iface),
		AltSetting: uint32(altSetting),
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_SETINTERFACE, uintptr(unsafe.Pointer(&setIface)))
	if errno != 0 {
		return errno
	}

	return nil
}

func (h *DeviceHandle) ClearHalt(endpoint uint8) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	ep := uint32(endpoint)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CLEAR_HALT, uintptr(unsafe.Pointer(&ep)))
	if errno != 0 {
		return errno
	}

	return nil
}

func (h *DeviceHandle) DetachKernelDriver(iface uint8) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	// First try simple USBDEVFS_DISCONNECT
	disconnectIface := struct {
		Interface uint32
		Flags     uint32
		Driver    [256]int8
	}{
		Interface: uint32(iface),
		Flags:     0x01, // USBDEVFS_DISCONNECT_CLAIM_IF_DRIVER - disconnect and claim
	}

	// Try DISCONNECT_CLAIM first (newer, more reliable)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_DISCONNECT_CLAIM, uintptr(unsafe.Pointer(&disconnectIface)))
	if errno == 0 {
		return nil // Success
	}
	// If DISCONNECT_CLAIM fails with ENOTTY, the kernel doesn't support it
	// If it fails with EINVAL, the structure might be wrong
	// If it fails with ENOENT, no driver is attached

	// Fallback to simple DISCONNECT (older method)
	ifaceNum := uint32(iface)
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_DISCONNECT, uintptr(unsafe.Pointer(&ifaceNum)))
	if errno != 0 {
		// ENODATA means no driver was attached (not an error)
		// EINVAL means the interface doesn't exist
		// ENOTTY means the device doesn't support this ioctl
		if errno == syscall.ENODATA || errno == syscall.ENOENT {
			return nil
		}
		if errno == syscall.ENOTTY {
			// Device doesn't support disconnect - might not have a kernel driver
			return fmt.Errorf("device doesn't support driver detachment")
		}
		return errno
	}

	return nil
}

func (h *DeviceHandle) AttachKernelDriver(iface uint8) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	// Use USBDEVFS_CONNECT to re-attach kernel driver
	ifaceNum := uint32(iface)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONNECT, uintptr(unsafe.Pointer(&ifaceNum)))
	if errno != 0 {
		// ENODATA means driver was not previously bound
		// EBUSY means driver is already attached
		// Both are acceptable outcomes
		if errno == syscall.ENODATA || errno == syscall.EBUSY {
			return nil
		}
		return errno
	}

	return nil
}

// GetStatus gets device, interface, or endpoint status
func (h *DeviceHandle) GetStatus(requestType uint8, index uint16) (uint16, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, ErrDeviceNotFound
	}

	buf := make([]byte, 2)

	ctrl := usbCtrlRequest{
		RequestType: requestType, // 0x80 for device, 0x81 for interface, 0x82 for endpoint
		Request:     USB_REQ_GET_STATUS,
		Value:       0,
		Index:       index,
		Length:      2,
		Data:        unsafe.Pointer(&buf[0]),
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return 0, errno
	}

	return binary.LittleEndian.Uint16(buf), nil
}

// ClearFeature clears a feature on device, interface, or endpoint
func (h *DeviceHandle) ClearFeature(requestType uint8, feature uint16, index uint16) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	ctrl := usbCtrlRequest{
		RequestType: requestType, // 0x00 for device, 0x01 for interface, 0x02 for endpoint
		Request:     USB_REQ_CLEAR_FEATURE,
		Value:       feature,
		Index:       index,
		Length:      0,
		Data:        nil,
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return errno
	}

	return nil
}

// SetFeature sets a feature on device, interface, or endpoint
func (h *DeviceHandle) SetFeature(requestType uint8, feature uint16, index uint16) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	ctrl := usbCtrlRequest{
		RequestType: requestType, // 0x00 for device, 0x01 for interface, 0x02 for endpoint
		Request:     USB_REQ_SET_FEATURE,
		Value:       feature,
		Index:       index,
		Length:      0,
		Data:        nil,
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return errno
	}

	return nil
}

// GetInterface gets the alternate setting of an interface
func (h *DeviceHandle) GetInterface(iface uint8) (uint8, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, ErrDeviceNotFound
	}

	buf := make([]byte, 1)

	ctrl := usbCtrlRequest{
		RequestType: 0x81, // Interface recipient
		Request:     USB_REQ_GET_INTERFACE,
		Value:       0,
		Index:       uint16(iface),
		Length:      1,
		Data:        unsafe.Pointer(&buf[0]),
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return 0, errno
	}

	return buf[0], nil
}

// GetRawDescriptor gets any descriptor by type and index
func (h *DeviceHandle) GetRawDescriptor(descType uint8, descIndex uint8, langID uint16, data []byte) (int, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, ErrDeviceNotFound
	}

	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}

	ctrl := usbCtrlRequest{
		RequestType: 0x80, // Device-to-host
		Request:     USB_REQ_GET_DESCRIPTOR,
		Value:       (uint16(descType) << 8) | uint16(descIndex),
		Index:       langID,
		Length:      uint16(len(data)),
		Data:        dataPtr,
	}

	ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return 0, errno
	}

	return int(ret), nil
}

// SetDescriptor sets a descriptor (rarely used)
func (h *DeviceHandle) SetDescriptor(descType uint8, descIndex uint8, langID uint16, data []byte) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	var dataPtr unsafe.Pointer
	if len(data) > 0 {
		dataPtr = unsafe.Pointer(&data[0])
	}

	ctrl := usbCtrlRequest{
		RequestType: 0x00, // Host-to-device
		Request:     USB_REQ_SET_DESCRIPTOR,
		Value:       (uint16(descType) << 8) | uint16(descIndex),
		Index:       langID,
		Length:      uint16(len(data)),
		Data:        dataPtr,
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return errno
	}

	return nil
}

// SynchFrame synchronizes isochronous transfers (USB 1.1/2.0)
func (h *DeviceHandle) SynchFrame(endpoint uint8) (uint16, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, ErrDeviceNotFound
	}

	buf := make([]byte, 2)

	ctrl := usbCtrlRequest{
		RequestType: 0x82, // Endpoint recipient
		Request:     USB_REQ_SYNCH_FRAME,
		Value:       0,
		Index:       uint16(endpoint),
		Length:      2,
		Data:        unsafe.Pointer(&buf[0]),
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return 0, errno
	}

	return binary.LittleEndian.Uint16(buf), nil
}

// GetCapabilities gets usbfs capabilities (Linux 3.15+)
func (h *DeviceHandle) GetCapabilities() (uint32, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, ErrDeviceNotFound
	}

	var caps uint32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_GET_CAPABILITIES, uintptr(unsafe.Pointer(&caps)))
	if errno != 0 {
		return 0, errno
	}

	return caps, nil
}

// GetSpeed gets the device speed
func (h *DeviceHandle) GetSpeed() (uint8, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return 0, ErrDeviceNotFound
	}

	var speed uint32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_GET_SPEED, uintptr(unsafe.Pointer(&speed)))
	if errno != 0 {
		return 0, errno
	}

	return uint8(speed), nil
}

// AllocStreams allocates bulk streams (USB 3.0+)
func (h *DeviceHandle) AllocStreams(numStreams uint32, endpoints []uint8) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	streams := struct {
		NumStreams uint32
		NumEps     uint32
		Eps        [30]uint8 // Maximum endpoints per interface
	}{
		NumStreams: numStreams,
		NumEps:     uint32(len(endpoints)),
	}

	copy(streams.Eps[:], endpoints)

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_ALLOC_STREAMS, uintptr(unsafe.Pointer(&streams)))
	if errno != 0 {
		return errno
	}

	return nil
}

// FreeStreams frees bulk streams (USB 3.0+)
func (h *DeviceHandle) FreeStreams(endpoints []uint8) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return ErrDeviceNotFound
	}

	streams := struct {
		NumEps uint32
		Eps    [30]uint8
	}{
		NumEps: uint32(len(endpoints)),
	}

	copy(streams.Eps[:], endpoints)

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_FREE_STREAMS, uintptr(unsafe.Pointer(&streams)))
	if errno != 0 {
		return errno
	}

	return nil
}

// GetSSEndpointCompanionDescriptor gets the SuperSpeed endpoint companion descriptor for a given endpoint
// This is equivalent to libusb_get_ss_endpoint_companion_descriptor
func (h *DeviceHandle) GetSSEndpointCompanionDescriptor(configIndex uint8, interfaceNumber uint8, altSetting uint8, endpointAddress uint8) (*SuperSpeedEndpointCompanionDescriptor, error) {
	config, err := h.GetConfigDescriptorByValue(configIndex)
	if err != nil {
		return nil, err
	}

	altSettingDesc := config.GetInterfaceAltSetting(interfaceNumber, altSetting)
	if altSettingDesc == nil {
		return nil, fmt.Errorf("interface %d alt setting %d not found", interfaceNumber, altSetting)
	}

	// Find the endpoint and return its companion descriptor if present
	for i := range altSettingDesc.Endpoints {
		if altSettingDesc.Endpoints[i].EndpointAddr == endpointAddress {
			if altSettingDesc.Endpoints[i].SSCompanion != nil {
				return altSettingDesc.Endpoints[i].SSCompanion, nil
			}
			// Search through extra data if companion wasn't parsed
			extra := altSettingDesc.Endpoints[i].Extra
			pos := 0
			for pos+2 <= len(extra) {
				length := int(extra[pos])
				descType := extra[pos+1]

				if length < 2 || pos+length > len(extra) {
					break
				}

				if descType == USB_DT_SS_ENDPOINT_COMPANION {
					if length < 6 {
						return nil, fmt.Errorf("invalid SS endpoint companion descriptor length: %d", length)
					}
					return &SuperSpeedEndpointCompanionDescriptor{
						Length:           extra[pos],
						DescriptorType:   extra[pos+1],
						MaxBurst:         extra[pos+2],
						Attributes:       extra[pos+3],
						BytesPerInterval: binary.LittleEndian.Uint16(extra[pos+4 : pos+6]),
					}, nil
				}

				pos += length
			}
			return nil, fmt.Errorf("SS endpoint companion descriptor not found for endpoint %02x", endpointAddress)
		}
	}

	return nil, fmt.Errorf("endpoint %02x not found", endpointAddress)
}

// GetSSUSBDeviceCapabilityDescriptor gets the SuperSpeed USB device capability descriptor
// This is equivalent to libusb_get_ss_usb_device_capability_descriptor
func (h *DeviceHandle) GetSSUSBDeviceCapabilityDescriptor() (*SuperSpeedUSBCapability, error) {
	// Read the full BOS descriptor once
	buf := make([]byte, 1024) // Start with reasonable size
	n, err := h.GetRawDescriptor(USB_DT_BOS, 0, 0, buf)
	if err != nil || n < 5 {
		return nil, fmt.Errorf("failed to read BOS descriptor: %w", err)
	}

	// Resize buffer to actual data read
	buf = buf[:n]

	// Parse BOS header
	if buf[1] != USB_DT_BOS {
		return nil, fmt.Errorf("not a BOS descriptor")
	}

	totalLength := binary.LittleEndian.Uint16(buf[2:4])
	if int(totalLength) > n {
		// Need to read more data
		buf = make([]byte, totalLength)
		n, err = h.GetRawDescriptor(USB_DT_BOS, 0, 0, buf)
		if err != nil || n < int(totalLength) {
			return nil, fmt.Errorf("failed to read full BOS descriptor: %w", err)
		}
		buf = buf[:n]
	}

	numDevCaps := buf[4]
	pos := 5 // Start after BOS header

	// Look for SuperSpeed USB capability (type 0x03)
	for i := 0; i < int(numDevCaps) && pos < len(buf); i++ {
		if pos+3 > len(buf) {
			break
		}

		length := int(buf[pos])
		descType := buf[pos+1]
		devCapType := buf[pos+2]

		if length < 3 || pos+length > len(buf) {
			break
		}

		if descType == USB_DT_DEVICE_CAPABILITY && devCapType == 0x03 {
			// Found SuperSpeed USB capability
			if length < 10 {
				return nil, fmt.Errorf("invalid SuperSpeed USB capability length: %d", length)
			}

			return &SuperSpeedUSBCapability{
				Length:                 buf[pos],
				DescriptorType:         buf[pos+1],
				DevCapabilityType:      buf[pos+2],
				Attributes:             buf[pos+3],
				SpeedsSupported:        binary.LittleEndian.Uint16(buf[pos+4 : pos+6]),
				FunctionalitySupported: buf[pos+6],
				U1DevExitLat:           buf[pos+7],
				U2DevExitLat:           binary.LittleEndian.Uint16(buf[pos+8 : pos+10]),
			}, nil
		}

		pos += length
	}

	return nil, fmt.Errorf("SuperSpeed USB capability not found")
}

// GetUSB20ExtensionDescriptor gets the USB 2.0 extension descriptor
// This is equivalent to libusb_get_usb_2_0_extension_descriptor
func (h *DeviceHandle) GetUSB20ExtensionDescriptor() (*USB2ExtensionCapability, error) {
	// Read the full BOS descriptor once
	buf := make([]byte, 1024) // Start with reasonable size
	n, err := h.GetRawDescriptor(USB_DT_BOS, 0, 0, buf)
	if err != nil || n < 5 {
		return nil, fmt.Errorf("failed to read BOS descriptor: %w", err)
	}

	// Resize buffer to actual data read
	buf = buf[:n]

	// Parse BOS header
	if buf[1] != USB_DT_BOS {
		return nil, fmt.Errorf("not a BOS descriptor")
	}

	totalLength := binary.LittleEndian.Uint16(buf[2:4])
	if int(totalLength) > n {
		// Need to read more data
		buf = make([]byte, totalLength)
		n, err = h.GetRawDescriptor(USB_DT_BOS, 0, 0, buf)
		if err != nil || n < int(totalLength) {
			return nil, fmt.Errorf("failed to read full BOS descriptor: %w", err)
		}
		buf = buf[:n]
	}

	numDevCaps := buf[4]
	pos := 5 // Start after BOS header

	// Look for USB 2.0 extension capability (type 0x02)
	for i := 0; i < int(numDevCaps) && pos < len(buf); i++ {
		if pos+3 > len(buf) {
			break
		}

		length := int(buf[pos])
		descType := buf[pos+1]
		devCapType := buf[pos+2]

		if length < 3 || pos+length > len(buf) {
			break
		}

		if descType == USB_DT_DEVICE_CAPABILITY && devCapType == 0x02 {
			// Found USB 2.0 extension capability
			if length < 7 {
				return nil, fmt.Errorf("invalid USB 2.0 extension capability length: %d", length)
			}

			return &USB2ExtensionCapability{
				Length:            buf[pos],
				DescriptorType:    buf[pos+1],
				DevCapabilityType: buf[pos+2],
				Attributes:        binary.LittleEndian.Uint32(buf[pos+3 : pos+7]),
			}, nil
		}

		pos += length
	}

	return nil, fmt.Errorf("USB 2.0 extension capability not found")
}

// ReadBOSDescriptor reads the Binary Object Store descriptor (USB 3.0+)
func (h *DeviceHandle) ReadBOSDescriptor() (*BOSDescriptor, []DeviceCapabilityDescriptor, error) {
	// First, get the BOS descriptor header
	buf := make([]byte, 5) // BOS descriptor header is 5 bytes

	n, err := h.GetRawDescriptor(USB_DT_BOS, 0, 0, buf)
	if err != nil || n < 5 {
		return nil, nil, fmt.Errorf("failed to read BOS descriptor: %w", err)
	}

	// Validate descriptor type
	if buf[1] != USB_DT_BOS {
		return nil, nil, fmt.Errorf("not a BOS descriptor (type: 0x%02x)", buf[1])
	}

	bos := &BOSDescriptor{
		Length:         buf[0],
		DescriptorType: buf[1],
		TotalLength:    binary.LittleEndian.Uint16(buf[2:4]),
		NumDeviceCaps:  buf[4],
	}

	// Validate total length is reasonable (not too small)
	if bos.TotalLength < 5 {
		return nil, nil, fmt.Errorf("invalid BOS total length: %d", bos.TotalLength)
	}
	// Note: TotalLength is uint16, so max value is 65535

	// Now read the full BOS descriptor with all capabilities
	fullBuf := make([]byte, bos.TotalLength)
	n, err = h.GetRawDescriptor(USB_DT_BOS, 0, 0, fullBuf)
	if err != nil || n < int(bos.TotalLength) {
		return nil, nil, fmt.Errorf("failed to read full BOS descriptor: %w", err)
	}

	// Parse device capabilities
	caps := make([]DeviceCapabilityDescriptor, 0, bos.NumDeviceCaps)
	pos := 5 // Start after BOS header

	for i := 0; i < int(bos.NumDeviceCaps) && pos < len(fullBuf); i++ {
		if pos+3 > len(fullBuf) {
			break
		}

		length := int(fullBuf[pos])

		// Validate descriptor length
		if length < 3 {
			break // Invalid descriptor length
		}
		if pos+length > len(fullBuf) {
			break // Descriptor extends beyond buffer
		}

		cap := DeviceCapabilityDescriptor{
			Length:            fullBuf[pos],
			DescriptorType:    fullBuf[pos+1],
			DevCapabilityType: fullBuf[pos+2],
		}

		caps = append(caps, cap)
		pos += length
	}

	return bos, caps, nil
}

// ReadDeviceQualifierDescriptor reads device qualifier (USB 2.0+)
func (h *DeviceHandle) ReadDeviceQualifierDescriptor() (*DeviceQualifierDescriptor, error) {
	buf := make([]byte, 10)

	n, err := h.GetRawDescriptor(USB_DT_DEVICE_QUALIFIER, 0, 0, buf)
	if err != nil || n < 10 {
		return nil, fmt.Errorf("failed to read device qualifier: %w", err)
	}

	qual := &DeviceQualifierDescriptor{
		Length:            buf[0],
		DescriptorType:    buf[1],
		USBVersion:        binary.LittleEndian.Uint16(buf[2:4]),
		DeviceClass:       buf[4],
		DeviceSubClass:    buf[5],
		DeviceProtocol:    buf[6],
		MaxPacketSize0:    buf[7],
		NumConfigurations: buf[8],
		Reserved:          buf[9],
	}

	return qual, nil
}

// SetTestMode sets USB test mode (for compliance testing)
func (h *DeviceHandle) SetTestMode(testMode uint8) error {
	return h.SetFeature(0x00, USB_DEVICE_TEST_MODE, uint16(testMode)<<8)
}

func (h *DeviceHandle) GetDevice() *Device {
	return h.device
}

func (h *DeviceHandle) GetStringDescriptor(index uint8) (string, error) {
	if index == 0 {
		return "", nil
	}

	buf := make([]byte, 256)

	ctrl := usbCtrlRequest{
		RequestType: 0x80,
		Request:     0x06,
		Value:       (0x03 << 8) | uint16(index),
		Index:       0x0409,
		Length:      uint16(len(buf)),
		Data:        unsafe.Pointer(&buf[0]),
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(h.fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return "", errno
	}

	if buf[0] < 2 {
		return "", fmt.Errorf("invalid string descriptor")
	}

	length := int(buf[0])
	if length > len(buf) {
		length = len(buf)
	}

	result := make([]uint16, 0, (length-2)/2)
	for i := 2; i < length; i += 2 {
		if i+1 < length {
			result = append(result, binary.LittleEndian.Uint16(buf[i:i+2]))
		}
	}

	return string(utf16ToRunes(result)), nil
}

func utf16ToRunes(u16 []uint16) []rune {
	runes := make([]rune, 0, len(u16))
	for _, v := range u16 {
		if v == 0 {
			break
		}
		runes = append(runes, rune(v))
	}
	return runes
}

type usbCtrlRequest struct {
	RequestType uint8
	Request     uint8
	Value       uint16
	Index       uint16
	Length      uint16
	Timeout     uint32
	Data        unsafe.Pointer
}

// WrapSysDevice creates a Device and DeviceHandle from an existing file descriptor.
// This is equivalent to libusb_wrap_sys_device(). The file descriptor must be
// an open USB device file descriptor.
// The DeviceHandle takes ownership of the fd and will close it when Close() is called.
func WrapSysDevice(fd int) (*DeviceHandle, error) {
	if fd < 0 {
		return nil, fmt.Errorf("invalid file descriptor: %d", fd)
	}

	// Verify the fd is valid by attempting a simple ioctl
	var caps uint32
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), USBDEVFS_GET_CAPABILITIES, uintptr(unsafe.Pointer(&caps)))
	if errno != 0 && errno != syscall.ENOTTY {
		// ENOTTY is acceptable for older kernels without capability support
		return nil, fmt.Errorf("file descriptor does not appear to be a USB device: %w", errno)
	}

	// Create a minimal Device structure
	// Path, Bus, and Address may be unknown when created from fd
	device := &Device{
		Path:    fmt.Sprintf("<fd:%d>", fd), // Placeholder path indicating fd origin
		Bus:     0,                          // Unknown bus
		Address: 0,                          // Unknown address
	}

	// Read device descriptor using control transfer
	buf := make([]byte, 18)
	ctrl := usbCtrlRequest{
		RequestType: 0x80,
		Request:     USB_REQ_GET_DESCRIPTOR,
		Value:       USB_DT_DEVICE << 8,
		Index:       0,
		Length:      18,
		Data:        unsafe.Pointer(&buf[0]),
	}

	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), USBDEVFS_CONTROL, uintptr(unsafe.Pointer(&ctrl)))
	if errno != 0 {
		return nil, fmt.Errorf("failed to read device descriptor: %w", errno)
	}

	// Parse device descriptor
	device.Descriptor = DeviceDescriptor{
		Length:            buf[0],
		DescriptorType:    buf[1],
		USBVersion:        binary.LittleEndian.Uint16(buf[2:4]),
		DeviceClass:       buf[4],
		DeviceSubClass:    buf[5],
		DeviceProtocol:    buf[6],
		MaxPacketSize0:    buf[7],
		VendorID:          binary.LittleEndian.Uint16(buf[8:10]),
		ProductID:         binary.LittleEndian.Uint16(buf[10:12]),
		DeviceVersion:     binary.LittleEndian.Uint16(buf[12:14]),
		ManufacturerIndex: buf[14],
		ProductIndex:      buf[15],
		SerialNumberIndex: buf[16],
		NumConfigurations: buf[17],
	}

	// Try to determine bus and address from /proc/self/fd if possible
	// This is optional and failure is not an error
	fdPath := fmt.Sprintf("/proc/self/fd/%d", fd)
	if devicePath, err := os.Readlink(fdPath); err == nil {
		if strings.HasPrefix(devicePath, "/dev/bus/usb/") {
			parts := strings.Split(devicePath, "/")
			if len(parts) >= 6 {
				if busNum, err := strconv.Atoi(parts[len(parts)-2]); err == nil && busNum >= 0 && busNum <= 255 {
					device.Bus = uint8(busNum)
				}
				if addrNum, err := strconv.Atoi(parts[len(parts)-1]); err == nil && addrNum >= 0 && addrNum <= 255 {
					device.Address = uint8(addrNum)
				}
				device.Path = devicePath // Use real path if available
			}
		}
	}

	// Create DeviceHandle with the provided fd
	handle := &DeviceHandle{
		device:        device,
		fd:            fd,
		claimedIfaces: make(map[uint8]bool),
		closed:        false,
		reapMap:       make(map[uintptr]func(error)),
	}

	device.handle = handle

	return handle, nil
}
