package usb

import (
	"encoding/hex"
	"testing"
)

func TestConfigDescriptorUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		data     string // hex encoded
		wantErr  bool
		validate func(t *testing.T, c *ConfigDescriptor)
	}{
		{
			name: "simple_config_with_one_interface",
			// Config descriptor (9 bytes) + Interface descriptor (9 bytes) + 2 Endpoint descriptors (7 bytes each)
			data: "09022000010100c032" + // Config: 32 bytes total, 1 interface, config value 1, self-powered, 100mA
				"0904000002ff010000" + // Interface: 0, alt 0, 2 endpoints, vendor specific, iface string index 0
				"0705810240000a" + // Endpoint: 0x81 (IN), bulk, 64 bytes, interval 10
				"0705020240000a", // Endpoint: 0x02 (OUT), bulk, 64 bytes, interval 10
			validate: func(t *testing.T, c *ConfigDescriptor) {
				if c.NumInterfaces != 1 {
					t.Errorf("NumInterfaces = %d, want 1", c.NumInterfaces)
				}
				if c.ConfigurationValue != 1 {
					t.Errorf("ConfigurationValue = %d, want 1", c.ConfigurationValue)
				}
				if c.MaxPower != 0x32 {
					t.Errorf("MaxPower = %d, want 50 (100mA)", c.MaxPower)
				}
				if len(c.Interfaces) != 1 {
					t.Errorf("len(Interfaces) = %d, want 1", len(c.Interfaces))
				}
				if len(c.Interfaces[0].AltSettings) != 1 {
					t.Errorf("len(AltSettings) = %d, want 1", len(c.Interfaces[0].AltSettings))
				}
				alt := c.Interfaces[0].AltSettings[0]
				if alt.NumEndpoints != 2 {
					t.Errorf("NumEndpoints = %d, want 2", alt.NumEndpoints)
				}
				if len(alt.Endpoints) != 2 {
					t.Errorf("len(Endpoints) = %d, want 2", len(alt.Endpoints))
				}
				// Check endpoints
				ep1 := alt.Endpoints[0]
				if ep1.EndpointAddr != 0x81 {
					t.Errorf("Endpoint[0].EndpointAddr = %02x, want 0x81", ep1.EndpointAddr)
				}
				if !ep1.IsInput() {
					t.Error("Endpoint[0] should be IN endpoint")
				}
				if ep1.TransferType() != 2 { // Bulk transfer type
					t.Errorf("Endpoint[0] transfer type = %d, want bulk", ep1.TransferType())
				}
				ep2 := alt.Endpoints[1]
				if ep2.EndpointAddr != 0x02 {
					t.Errorf("Endpoint[1].EndpointAddr = %02x, want 0x02", ep2.EndpointAddr)
				}
				if !ep2.IsOutput() {
					t.Error("Endpoint[1] should be OUT endpoint")
				}
			},
		},
		{
			name: "config_with_multiple_alt_settings",
			data: "09023b00020100c032" + // Config: 59 bytes total, 2 interfaces
				"09040000010e010000" + // Interface 0, alt 0, 1 endpoint, video control, 9 bytes
				"0705830308000a" + // Endpoint: 0x83 (IN), interrupt, 8 bytes, 7 bytes total
				"09040100000e020000" + // Interface 1, alt 0, 0 endpoints, video streaming, 9 bytes
				"09040101010e020000" + // Interface 1, alt 1, 1 endpoint, video streaming, 9 bytes
				"0705810500020001", // Endpoint: 0x81 (IN), isochronous, 512 bytes, 7 bytes total
			validate: func(t *testing.T, c *ConfigDescriptor) {
				if c.NumInterfaces != 2 {
					t.Errorf("NumInterfaces = %d, want 2", c.NumInterfaces)
				}
				if len(c.Interfaces) != 2 {
					t.Errorf("len(Interfaces) = %d, want 2", len(c.Interfaces))
				}
				// Check interface 0
				if len(c.Interfaces[0].AltSettings) != 1 {
					t.Errorf("Interface[0] AltSettings = %d, want 1", len(c.Interfaces[0].AltSettings))
				}
				// Check interface 1
				if len(c.Interfaces[1].AltSettings) != 2 {
					t.Errorf("Interface[1] AltSettings = %d, want 2", len(c.Interfaces[1].AltSettings))
				}
				// Check alt setting 0 has no endpoints
				if len(c.Interfaces[1].AltSettings[0].Endpoints) != 0 {
					t.Errorf("Interface[1].AltSettings[0] endpoints = %d, want 0",
						len(c.Interfaces[1].AltSettings[0].Endpoints))
				}
				// Check alt setting 1 has 1 endpoint
				if len(c.Interfaces[1].AltSettings[1].Endpoints) != 1 {
					t.Errorf("Interface[1].AltSettings[1] endpoints = %d, want 1",
						len(c.Interfaces[1].AltSettings[1].Endpoints))
				}
				// Check isochronous endpoint
				ep := c.Interfaces[1].AltSettings[1].Endpoints[0]
				if ep.TransferType() != 1 { // Isochronous transfer type
					t.Errorf("Endpoint transfer type = %d, want isochronous", ep.TransferType())
				}
			},
		},
		{
			name: "config_with_class_specific_descriptors",
			data: "09024300020100c032" + // Config: 67 bytes total
				"0904000001030100" + "00" + // Interface 0: HID class, 9 bytes
				"0921110100012234" + // HID descriptor (class-specific), 9 bytes
				"0705810340000a" + // Endpoint, 7 bytes
				"0904010002080650" + "00" + // Interface 1: Mass Storage, 9 bytes
				"0705820240000a" + // Endpoint OUT, 7 bytes
				"0705830240000a", // Endpoint IN, 7 bytes
			validate: func(t *testing.T, c *ConfigDescriptor) {
				// Check that class-specific descriptor is in Extra
				if len(c.Interfaces[0].AltSettings[0].Extra) == 0 {
					t.Error("Expected class-specific descriptor in Extra")
				}
				// HID descriptor should be 9 bytes starting with 0x09, 0x21
				extra := c.Interfaces[0].AltSettings[0].Extra
				if len(extra) < 9 || extra[0] != 0x09 || extra[1] != 0x21 {
					t.Errorf("Invalid HID descriptor in Extra: %x", extra)
				}
			},
		},
		{
			name: "config_with_interface_association",
			data: "09024b00030100c032" + // Config: 75 bytes total
				"080b00020e030000" + // Interface Association Descriptor
				"0904000001ff0100" + // Interface 0
				"0705810308000a" + // Endpoint
				"0904010000ff0200" + // Interface 1
				"0904020001030100" + // Interface 2
				"0705820308000a", // Endpoint
			validate: func(t *testing.T, c *ConfigDescriptor) {
				// IAD should be in config's Extra since it comes before interfaces
				if len(c.Extra) < 8 {
					t.Error("Expected IAD in config Extra")
				}
				if c.Extra[0] != 0x08 || c.Extra[1] != 0x0b {
					t.Errorf("Invalid IAD in Extra: %x", c.Extra)
				}
			},
		},
		{
			name: "config_with_superspeed_companion",
			data: "09022e00010100c032" + // Config, 46 bytes total
				"0904000002ff010000" + // Interface, 9 bytes
				"0705810240000a" + // Endpoint, 7 bytes
				"063000000000" + // SuperSpeed Endpoint Companion, 6 bytes
				"0705020240000a", // Another endpoint, 7 bytes
			validate: func(t *testing.T, c *ConfigDescriptor) {
				ep := c.Interfaces[0].AltSettings[0].Endpoints[0]
				if ep.SSCompanion == nil {
					t.Error("Expected SuperSpeed companion descriptor")
				}
				if ep.SSCompanion.DescriptorType != USB_DT_SS_ENDPOINT_COMPANION {
					t.Errorf("Wrong companion descriptor type: %02x", ep.SSCompanion.DescriptorType)
				}
			},
		},
		{
			name:    "config_too_short",
			data:    "090220",
			wantErr: true,
		},
		{
			name:    "interface_descriptor_too_short",
			data:    "09022000010100c032" + "07040000000ff0",
			wantErr: true,
		},
		{
			name:    "endpoint_descriptor_too_short",
			data:    "09022000010100c032" + "0904000001ff010000" + "05058102ff",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := hex.DecodeString(tt.data)
			if err != nil {
				t.Fatalf("Failed to decode hex: %v", err)
			}

			c := &ConfigDescriptor{}
			err = c.Unmarshal(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, c)
			}
		})
	}
}

func TestConfigDescriptorHelpers(t *testing.T) {
	// Create a test config descriptor
	data, _ := hex.DecodeString(
		"09023b00020100c032" + // Config
			"09040000010e010000" + // Interface 0, alt 0
			"0705830308000a" + // Endpoint 0x83
			"09040100000e020000" + // Interface 1, alt 0
			"09040101010e020000" + // Interface 1, alt 1
			"0705810500020001") // Endpoint 0x81

	c := &ConfigDescriptor{}
	if err := c.Unmarshal(data); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	t.Run("GetInterface", func(t *testing.T) {
		iface := c.Interface(0)
		if iface == nil {
			t.Error("GetInterface(0) returned nil")
		}
		iface = c.Interface(1)
		if iface == nil {
			t.Error("GetInterface(1) returned nil")
		}
		iface = c.Interface(2)
		if iface != nil {
			t.Error("GetInterface(2) should return nil")
		}
	})

	t.Run("GetInterfaceAltSetting", func(t *testing.T) {
		alt := c.InterfaceAltSetting(1, 0)
		if alt == nil {
			t.Error("GetInterfaceAltSetting(1, 0) returned nil")
		} else if alt.AlternateSetting != 0 {
			t.Errorf("Wrong alt setting: %d", alt.AlternateSetting)
		}

		alt = c.InterfaceAltSetting(1, 1)
		if alt == nil {
			t.Error("GetInterfaceAltSetting(1, 1) returned nil")
		} else if alt.AlternateSetting != 1 {
			t.Errorf("Wrong alt setting: %d", alt.AlternateSetting)
		}

		alt = c.InterfaceAltSetting(1, 2)
		if alt != nil {
			t.Error("GetInterfaceAltSetting(1, 2) should return nil")
		}
	})

	t.Run("FindEndpoint", func(t *testing.T) {
		ep := c.FindEndpoint(0x83)
		if ep == nil {
			t.Error("FindEndpoint(0x83) returned nil")
		} else if ep.EndpointAddr != 0x83 {
			t.Errorf("Wrong endpoint: %02x", ep.EndpointAddr)
		}

		ep = c.FindEndpoint(0x81)
		if ep == nil {
			t.Error("FindEndpoint(0x81) returned nil")
		}

		ep = c.FindEndpoint(0x99)
		if ep != nil {
			t.Error("FindEndpoint(0x99) should return nil")
		}
	})
}

func TestEndpointHelpers(t *testing.T) {
	tests := []struct {
		name     string
		endpoint Endpoint
		wantIn   bool
		wantOut  bool
		wantNum  uint8
		wantType uint8
	}{
		{
			name: "bulk_in_ep1",
			endpoint: Endpoint{
				EndpointAddr: 0x81,
				Attributes:   0x02,
			},
			wantIn:   true,
			wantOut:  false,
			wantNum:  1,
			wantType: 2, // Bulk
		},
		{
			name: "bulk_out_ep2",
			endpoint: Endpoint{
				EndpointAddr: 0x02,
				Attributes:   0x02,
			},
			wantIn:   false,
			wantOut:  true,
			wantNum:  2,
			wantType: 2, // Bulk
		},
		{
			name: "interrupt_in_ep3",
			endpoint: Endpoint{
				EndpointAddr: 0x83,
				Attributes:   0x03,
			},
			wantIn:   true,
			wantOut:  false,
			wantNum:  3,
			wantType: 3, // Interrupt
		},
		{
			name: "isochronous_out_ep4",
			endpoint: Endpoint{
				EndpointAddr: 0x04,
				Attributes:   0x01,
			},
			wantIn:   false,
			wantOut:  true,
			wantNum:  4,
			wantType: 1, // Isochronous
		},
		{
			name: "control_ep0",
			endpoint: Endpoint{
				EndpointAddr: 0x00,
				Attributes:   0x00,
			},
			wantIn:   false,
			wantOut:  true,
			wantNum:  0,
			wantType: 0, // Control
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.endpoint.IsInput(); got != tt.wantIn {
				t.Errorf("IsInput() = %v, want %v", got, tt.wantIn)
			}
			if got := tt.endpoint.IsOutput(); got != tt.wantOut {
				t.Errorf("IsOutput() = %v, want %v", got, tt.wantOut)
			}
			if got := tt.endpoint.EndpointNumber(); got != tt.wantNum {
				t.Errorf("GetEndpointNumber() = %d, want %d", got, tt.wantNum)
			}
			if got := tt.endpoint.TransferType(); got != TransferType(tt.wantType) {
				t.Errorf("TransferType() = %d, want %d", got, tt.wantType)
			}
		})
	}
}
