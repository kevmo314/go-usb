# github.com/kevmo314/go-usb

A pure Go library for USB device communication, providing a libusb-like interface without C dependencies. This library enables direct USB device access through the Linux kernel's usbfs interface.

## Features

- Pure Go implementation (no CGO or libusb dependency)
- Device enumeration and management
- Control, bulk, interrupt, and isochronous transfers
- Synchronous and asynchronous transfer operations
- Thread-safe device operations
- Comprehensive error handling
- String descriptor support
- Configuration and interface management

## Installation

```bash
go get github.com/kevmo314/go-usb
```

## Requirements

- Linux operating system (uses usbfs)
- Go 1.18 or higher
- Appropriate permissions to access USB devices (typically requires root or udev rules)

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    usb "github.com/kevmo314/go-usb"
)

func main() {
    // Get list of USB devices (no context needed!)
    devices, err := usb.DeviceList()
    if err != nil {
        log.Fatal(err)
    }

    // Print all devices
    for _, dev := range devices {
        fmt.Printf("Device: Bus %03d Address %03d VID:PID %04x:%04x\n",
            dev.Bus, dev.Address,
            dev.Descriptor.VendorID, dev.Descriptor.ProductID)
    }

    // Open a specific device by VID/PID
    handle, err := usb.OpenDevice(0x1234, 0x5678)
    if err != nil {
        log.Fatal(err)
    }
    defer handle.Close()

    // Perform operations with the device...
}
```

## Usage Examples

### Enumerate Devices

```go
devices, _ := usb.DeviceList()
for _, dev := range devices {
    desc := dev.Descriptor
    fmt.Printf("VID: %04x, PID: %04x\n", desc.VendorID, desc.ProductID)
}
```

### Open Device and Read String Descriptors

```go
// Open device by VID/PID
handle, _ := usb.OpenDevice(vendorID, productID)
defer handle.Close()

// Or open a specific device from the list
devices, _ := usb.DeviceList()
handle, _ := devices[0].Open()
defer handle.Close()

// Get manufacturer string
manufacturer, _ := handle.StringDescriptor(desc.ManufacturerIndex)
fmt.Printf("Manufacturer: %s\n", manufacturer)

// Get product string
product, _ := handle.StringDescriptor(desc.ProductIndex)
fmt.Printf("Product: %s\n", product)
```

### Control Transfer

```go
// Read device descriptor
buf := make([]byte, 18)
n, err := handle.ControlTransfer(
    0x80,                    // bmRequestType (device-to-host)
    0x06,                    // bRequest (GET_DESCRIPTOR)
    0x0100,                  // wValue (DEVICE descriptor)
    0x0000,                  // wIndex
    buf,                     // data buffer
    5 * time.Second,         // timeout
)
```

### Bulk Transfer

```go
// Claim interface first
err := handle.ClaimInterface(0)
if err != nil {
    log.Fatal(err)
}
defer handle.ReleaseInterface(0)

// Write data to bulk endpoint
data := []byte("Hello USB!")
n, err := handle.BulkTransfer(
    0x02,                    // endpoint address (OUT endpoint 2)
    data,                    // data to send
    5 * time.Second,         // timeout
)

// Read data from bulk endpoint
buf := make([]byte, 512)
n, err = handle.BulkTransfer(
    0x82,                    // endpoint address (IN endpoint 2)
    buf,                     // buffer to receive data
    5 * time.Second,         // timeout
)
```

### Interrupt Transfer

```go
// Read from interrupt endpoint
buf := make([]byte, 64)
n, err := handle.InterruptTransfer(
    0x81,                    // endpoint address (IN endpoint 1)
    buf,                     // buffer to receive data
    100 * time.Millisecond,  // timeout
)
```

### Asynchronous Transfer

```go
// Create a transfer object
transfer := usb.NewTransfer(handle, 0x81, usb.TransferTypeInterrupt, 64)

// Set callback
transfer.SetCallback(func(t *usb.Transfer) {
    if t.Status() == usb.TransferCompleted {
        data := t.Buffer()
        fmt.Printf("Received %d bytes\n", t.ActualLength())
    }
})

// Submit transfer
err := handle.SubmitTransfer(transfer)

// Reap completed transfers
completedTransfer, err := handle.ReapTransfer(time.Second)
```

## Permissions

USB device access typically requires elevated privileges. You have several options:

### Run as root
```bash
sudo go run main.go
```

### Create udev rules
Create a file `/etc/udev/rules.d/99-usb.rules`:
```
# Allow access to specific device
SUBSYSTEM=="usb", ATTRS{idVendor}=="1234", ATTRS{idProduct}=="5678", MODE="0666"

# Allow access to all USB devices (less secure)
SUBSYSTEM=="usb", MODE="0666"
```

Then reload udev:
```bash
sudo udevadm control --reload-rules
sudo udevadm trigger
```

## Included Tools

The repository includes several command-line tools and examples in the `cmd/` directory:

- **lsusb**: List USB devices (similar to the system lsusb command)
  ```bash
  go run cmd/lsusb/main.go       # List all devices
  go run cmd/lsusb/main.go -v     # Verbose output
  go run cmd/lsusb/main.go -t     # Tree view
  go run cmd/lsusb/main.go -s :6  # Show device 6 on any bus
  go run cmd/lsusb/main.go -s 1:6 # Show device 6 on bus 1
  ```

- **browse-msc**: Browse USB Mass Storage devices
- **browse-uvc**: Browse USB Video Class devices
- **verify-transfers**: Test and verify USB transfer operations

## Limitations

- Linux only (uses usbfs interface)
- Requires appropriate permissions for USB device access
- No hotplug support (can be implemented with udev monitoring)

## Resources

- [USB 2.0 Specification](https://www.usb.org/document-library/usb-20-specification)
- [Linux usbfs Documentation](https://www.kernel.org/doc/html/latest/driver-api/usb/index.html)
- [libusb Documentation](https://libusb.info/)
