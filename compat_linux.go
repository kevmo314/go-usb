package usb

// Compatibility methods for Linux to match cross-platform API

// GetConfiguration gets the current device configuration
func (h *DeviceHandle) GetConfiguration() (int, error) {
	return h.Configuration()
}

// GetConfigDescriptor gets a configuration descriptor by index
func (h *DeviceHandle) GetConfigDescriptor(index uint8) (*ConfigDescriptor, error) {
	// On Linux, we use ConfigDescriptorByValue, but need to convert
	return h.ConfigDescriptorByValue(index + 1)
}

// GetActiveConfigDescriptor gets the descriptor for the active configuration
func (h *DeviceHandle) GetActiveConfigDescriptor() (*ConfigDescriptor, error) {
	config, err := h.GetConfiguration()
	if err != nil {
		return nil, err
	}
	
	if config > 0 {
		return h.ConfigDescriptorByValue(uint8(config))
	}
	
	return h.ConfigDescriptorByValue(1)
}

// GetDeviceDescriptor returns the device descriptor
func (h *DeviceHandle) GetDeviceDescriptor() (*DeviceDescriptor, error) {
	desc := h.Descriptor()
	return &desc, nil
}

// SetAltSetting sets the alternate setting for an interface
func (h *DeviceHandle) SetAltSetting(iface, altSetting uint8) error {
	return h.SetInterfaceAltSetting(iface, altSetting)
}

// KernelDriverActive checks if a kernel driver is active
func (h *DeviceHandle) KernelDriverActive(iface uint8) (bool, error) {
	// Not directly exposed in Linux implementation
	// Try to claim interface - if it fails with EBUSY, driver is active
	err := h.ClaimInterface(iface)
	if err != nil {
		if err == ErrDeviceBusy {
			return true, nil
		}
		return false, err
	}
	// Release if we successfully claimed it
	h.ReleaseInterface(iface)
	return false, nil
}

// GetBOSDescriptor gets the BOS descriptor
func (h *DeviceHandle) GetBOSDescriptor() (*BOSDescriptor, []DeviceCapabilityDescriptor, error) {
	return h.ReadBOSDescriptor()
}

// GetDeviceQualifierDescriptor gets the device qualifier descriptor
func (h *DeviceHandle) GetDeviceQualifierDescriptor() (*DeviceQualifierDescriptor, error) {
	return h.ReadDeviceQualifierDescriptor()
}

// GetCapabilities returns device capabilities
func (h *DeviceHandle) GetCapabilities() (uint32, error) {
	return h.Capabilities()
}

// GetSpeed returns the device speed
func (h *DeviceHandle) GetSpeed() (Speed, error) {
	speed, err := h.Speed()
	return Speed(speed), err
}

// GetStatus gets device/interface/endpoint status
func (h *DeviceHandle) GetStatus(recipient, index uint16) (uint16, error) {
	requestType := uint8(0x80 | (recipient & 0x1F))
	return h.Status(requestType, index)
}