package usb

import (
	"fmt"
)

// Version returns the version of the go-usb library
func Version() string {
	return "1.0.0"
}

// Error types
var (
	ErrDeviceNotFound   = fmt.Errorf("device not found")
	ErrPermissionDenied = fmt.Errorf("permission denied")
	ErrDeviceBusy       = fmt.Errorf("device busy")
	ErrEAGAIN           = fmt.Errorf("resource temporarily unavailable")
	ErrInvalidParameter = fmt.Errorf("invalid parameter")
	ErrNotSupported     = fmt.Errorf("operation not supported")
	ErrIO               = fmt.Errorf("I/O error")
	ErrNoDevice         = fmt.Errorf("no device")
	ErrNotFound         = fmt.Errorf("not found")
	ErrBusy             = fmt.Errorf("busy")
	ErrTimeout          = fmt.Errorf("timeout")
	ErrOverflow         = fmt.Errorf("overflow")
	ErrPipe             = fmt.Errorf("pipe error")
	ErrInterrupted      = fmt.Errorf("interrupted")
	ErrNoMem            = fmt.Errorf("no memory")
	ErrOther            = fmt.Errorf("other error")
)

// Speed types
type Speed int

const (
	SpeedUnknown Speed = iota
	SpeedLow
	SpeedFull
	SpeedHigh
	SpeedSuper
	SpeedSuperPlus
)

// Endpoint direction
type EndpointDirection uint8

const (
	EndpointDirectionOut EndpointDirection = 0
	EndpointDirectionIn  EndpointDirection = 0x80
)
