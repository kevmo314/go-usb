package usb

import (
	"encoding/binary"
	"fmt"
)

// ConfigDescriptor represents a parsed USB configuration descriptor
// Similar to libusb_config_descriptor
type ConfigDescriptor struct {
	// Standard config descriptor fields
	Length             uint8
	DescriptorType     uint8
	TotalLength        uint16
	NumInterfaces      uint8
	ConfigurationValue uint8
	ConfigurationIndex uint8
	Attributes         uint8
	MaxPower           uint8

	// Parsed interfaces
	Interfaces []Interface

	// Extra descriptors not parsed into the structure
	Extra []byte
}

// Interface represents a USB interface with all its alternate settings
// Similar to libusb_interface
type Interface struct {
	// Array of alternate settings for this interface
	AltSettings []InterfaceAltSetting
}

// InterfaceAltSetting represents an interface descriptor with its endpoints
// Similar to libusb_interface_descriptor
type InterfaceAltSetting struct {
	// Standard interface descriptor fields
	Length            uint8
	DescriptorType    uint8
	InterfaceNumber   uint8
	AlternateSetting  uint8
	NumEndpoints      uint8
	InterfaceClass    uint8
	InterfaceSubClass uint8
	InterfaceProtocol uint8
	InterfaceIndex    uint8

	// Parsed endpoints
	Endpoints []Endpoint

	// Extra descriptors (e.g., class-specific descriptors)
	Extra []byte
}

// Endpoint represents a parsed endpoint descriptor
// Similar to libusb_endpoint_descriptor
type Endpoint struct {
	// Standard endpoint descriptor fields
	Length         uint8
	DescriptorType uint8
	EndpointAddr   uint8
	Attributes     uint8
	MaxPacketSize  uint16
	Interval       uint8

	// For SuperSpeed devices, companion descriptor if present
	SSCompanion *SuperSpeedEndpointCompanionDescriptor

	// Extra descriptors
	Extra []byte
}

// Unmarshal parses raw configuration descriptor data into this ConfigDescriptor
func (c *ConfigDescriptor) Unmarshal(data []byte) error {
	if len(data) < 9 {
		return fmt.Errorf("config descriptor too short: %d bytes", len(data))
	}

	// Parse the configuration descriptor header
	c.Length = data[0]
	c.DescriptorType = data[1]
	c.TotalLength = binary.LittleEndian.Uint16(data[2:4])
	c.NumInterfaces = data[4]
	c.ConfigurationValue = data[5]
	c.ConfigurationIndex = data[6]
	c.Attributes = data[7]
	c.MaxPower = data[8]

	// Map to track interfaces by number
	interfaceMap := make(map[uint8]*Interface)

	// Current parsing state
	var currentInterface *InterfaceAltSetting
	var currentEndpoints []Endpoint
	var extraBuffer []byte

	// Parse the rest of the descriptors
	pos := 9
	for pos < len(data) {
		if pos+2 > len(data) {
			break
		}

		length := int(data[pos])
		descType := data[pos+1]

		if length == 0 || pos+length > len(data) {
			break
		}

		switch descType {
		case USB_DT_INTERFACE: // 0x04
			// Save previous interface if exists
			if currentInterface != nil {
				currentInterface.Endpoints = currentEndpoints
				currentInterface.Extra = extraBuffer

				// Add or update interface in map
				if _, exists := interfaceMap[currentInterface.InterfaceNumber]; !exists {
					interfaceMap[currentInterface.InterfaceNumber] = &Interface{
						AltSettings: []InterfaceAltSetting{},
					}
				}
				interfaceMap[currentInterface.InterfaceNumber].AltSettings = append(
					interfaceMap[currentInterface.InterfaceNumber].AltSettings, *currentInterface)

				extraBuffer = nil
				currentEndpoints = nil
			}

			if length < 9 {
				return fmt.Errorf("interface descriptor too short: %d bytes", length)
			}

			// Parse interface descriptor
			iface := InterfaceAltSetting{
				Length:            data[pos],
				DescriptorType:    data[pos+1],
				InterfaceNumber:   data[pos+2],
				AlternateSetting:  data[pos+3],
				NumEndpoints:      data[pos+4],
				InterfaceClass:    data[pos+5],
				InterfaceSubClass: data[pos+6],
				InterfaceProtocol: data[pos+7],
				InterfaceIndex:    data[pos+8],
			}

			currentInterface = &iface
			currentEndpoints = make([]Endpoint, 0, iface.NumEndpoints)

		case USB_DT_ENDPOINT: // 0x05
			if currentInterface == nil {
				// Endpoint without interface, add to config extra
				c.Extra = append(c.Extra, data[pos:pos+length]...)
			} else {
				if length < 7 {
					return fmt.Errorf("endpoint descriptor too short: %d bytes", length)
				}

				endpoint := Endpoint{
					Length:         data[pos],
					DescriptorType: data[pos+1],
					EndpointAddr:   data[pos+2],
					Attributes:     data[pos+3],
					MaxPacketSize:  binary.LittleEndian.Uint16(data[pos+4 : pos+6]),
					Interval:       data[pos+6],
				}

				// Check if next descriptor is SuperSpeed companion
				nextPos := pos + length
				if nextPos+2 <= len(data) && data[nextPos+1] == USB_DT_SS_ENDPOINT_COMPANION {
					companionLen := int(data[nextPos])
					if nextPos+companionLen <= len(data) && companionLen >= 6 {
						endpoint.SSCompanion = &SuperSpeedEndpointCompanionDescriptor{
							Length:           data[nextPos],
							DescriptorType:   data[nextPos+1],
							MaxBurst:         data[nextPos+2],
							Attributes:       data[nextPos+3],
							BytesPerInterval: binary.LittleEndian.Uint16(data[nextPos+4 : nextPos+6]),
						}
						// Skip the companion descriptor
						pos = nextPos
						length = companionLen
					}
				}

				currentEndpoints = append(currentEndpoints, endpoint)
			}

		case USB_DT_INTERFACE_ASSOCIATION: // 0x0b
			// Interface Association Descriptor
			if currentInterface != nil {
				extraBuffer = append(extraBuffer, data[pos:pos+length]...)
			} else {
				c.Extra = append(c.Extra, data[pos:pos+length]...)
			}

		default:
			// Unknown or class-specific descriptor
			if currentInterface != nil {
				// Add to current interface's extra
				extraBuffer = append(extraBuffer, data[pos:pos+length]...)
			} else {
				// Add to config's extra
				c.Extra = append(c.Extra, data[pos:pos+length]...)
			}
		}

		pos += length
	}

	// Save last interface if exists
	if currentInterface != nil {
		currentInterface.Endpoints = currentEndpoints
		currentInterface.Extra = extraBuffer

		// Add or update interface in map
		if _, exists := interfaceMap[currentInterface.InterfaceNumber]; !exists {
			interfaceMap[currentInterface.InterfaceNumber] = &Interface{
				AltSettings: []InterfaceAltSetting{},
			}
		}
		interfaceMap[currentInterface.InterfaceNumber].AltSettings = append(
			interfaceMap[currentInterface.InterfaceNumber].AltSettings, *currentInterface)
	}

	// Convert map to sorted slice
	c.Interfaces = make([]Interface, 0, len(interfaceMap))
	for i := range uint8(255) {
		if iface, exists := interfaceMap[i]; exists {
			c.Interfaces = append(c.Interfaces, *iface)
		}
	}

	return nil
}

// Helper methods for ConfigDescriptor

// GetInterface returns the interface with the given number, or nil if not found
func (c *ConfigDescriptor) GetInterface(interfaceNumber uint8) *Interface {
	for i := range c.Interfaces {
		if len(c.Interfaces[i].AltSettings) > 0 &&
			c.Interfaces[i].AltSettings[0].InterfaceNumber == interfaceNumber {
			return &c.Interfaces[i]
		}
	}
	return nil
}

// GetInterfaceAltSetting returns a specific alternate setting for an interface
func (c *ConfigDescriptor) GetInterfaceAltSetting(interfaceNumber, altSetting uint8) *InterfaceAltSetting {
	iface := c.GetInterface(interfaceNumber)
	if iface == nil {
		return nil
	}

	for i := range iface.AltSettings {
		if iface.AltSettings[i].AlternateSetting == altSetting {
			return &iface.AltSettings[i]
		}
	}
	return nil
}

// FindEndpoint finds an endpoint by address across all interfaces and alt settings
func (c *ConfigDescriptor) FindEndpoint(endpointAddress uint8) *Endpoint {
	for _, iface := range c.Interfaces {
		for _, altSetting := range iface.AltSettings {
			for i := range altSetting.Endpoints {
				if altSetting.Endpoints[i].EndpointAddr == endpointAddress {
					return &altSetting.Endpoints[i]
				}
			}
		}
	}
	return nil
}

// IsInput returns true if this is an IN endpoint
func (e *Endpoint) IsInput() bool {
	return (e.EndpointAddr & 0x80) != 0
}

// IsOutput returns true if this is an OUT endpoint
func (e *Endpoint) IsOutput() bool {
	return (e.EndpointAddr & 0x80) == 0
}

// GetEndpointNumber returns the endpoint number (without direction bit)
func (e *Endpoint) GetEndpointNumber() uint8 {
	return e.EndpointAddr & 0x0F
}

// GetTransferType returns the transfer type (Control, Isochronous, Bulk, or Interrupt)
func (e *Endpoint) GetTransferType() uint8 {
	return e.Attributes & 0x03
}
