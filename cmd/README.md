# USB Configuration Descriptor Examples

This directory contains example programs demonstrating how to use the go-usb library to fetch and parse USB configuration descriptors.

## Examples

### listconfigs

Lists all USB devices and their configuration descriptors in a hierarchical format.

```bash
# Build
go build ./cmd/listconfigs

# List all USB devices and their configurations
./listconfigs

# List configurations for a specific device (by VID/PID)
./listconfigs -vid 0x1234 -pid 0x5678

# Verbose output (includes endpoints and extra descriptors)
./listconfigs -v
```

Output shows:
- Device identification (VID, PID, manufacturer, product)
- Configuration descriptors with attributes and power requirements
- Interface descriptors with class information
- Endpoint descriptors with transfer types and directions
- SuperSpeed companion descriptors when present

### capabilities

Demonstrates USB 3.0+ capability descriptors including USB 2.0 extensions and SuperSpeed capabilities.

```bash
# Build
go build ./cmd/capabilities

# List USB 3.0+ device capabilities
./capabilities

# Check specific device
./capabilities -vid 0x1234 -pid 0x5678
```

Output shows:
- BOS (Binary Object Store) descriptors
- USB 2.0 Extension capabilities (LPM support, BESL parameters)
- SuperSpeed USB capabilities (supported speeds, latency values)
- SuperSpeed endpoint companion descriptors with burst and stream information

## Key Features Demonstrated

1. **Configuration Descriptor Parsing**
   - Hierarchical structure: Config → Interface → Alt Setting → Endpoint
   - Class-specific descriptors stored in Extra fields
   - Interface Association Descriptors (IAD)

2. **SuperSpeed Support**
   - SuperSpeed endpoint companion descriptors
   - MaxBurst and stream capabilities
   - Isochronous mult values

3. **USB 2.0 Extensions**
   - Link Power Management (LPM) capabilities
   - BESL (Best Effort Service Latency) parameters

4. **Helper Methods**
   - `GetConfigDescriptorByValue()` - Get parsed configuration
   - `GetSSEndpointCompanionDescriptor()` - Get SuperSpeed companion
   - `GetSSUSBDeviceCapabilityDescriptor()` - Get SuperSpeed capabilities
   - `GetUSB20ExtensionDescriptor()` - Get USB 2.0 extensions

## Requirements

- Root/administrator privileges may be required to access USB devices
- USB 3.0+ devices and ports for SuperSpeed features
- Linux operating system (the library uses Linux usbfs)

## Notes

- The library automatically parses the hierarchical descriptor structure
- Extra/class-specific descriptors are preserved in the Extra field
- The Unmarshal method follows Go conventions for parsing binary data