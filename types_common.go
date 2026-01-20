package usb

// This file contains common type definitions and constants used across platforms.
// Platform-specific implementations are in *_linux.go files.

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

// Transfer types
type TransferType int

const (
	TransferTypeControl TransferType = iota
	TransferTypeIsochronous
	TransferTypeBulk
	TransferTypeInterrupt
	TransferTypeStream
)

// Transfer status
type TransferStatus int

const (
	TransferCompleted TransferStatus = iota
	TransferError
	TransferTimedOut
	TransferCancelled
	TransferStall
	TransferNoDevice
	TransferOverflow
	TransferInProgress
)

// DeviceDescriptor represents a USB device descriptor
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

// RawConfigDescriptor represents a raw USB configuration descriptor
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

// InterfaceDescriptor represents a USB interface descriptor
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

// EndpointDescriptor represents a USB endpoint descriptor
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
