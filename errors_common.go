package usb

import "fmt"

// Define the actual error values
var (
	errDeviceNotFound   = fmt.Errorf("device not found")
	errPermissionDenied = fmt.Errorf("permission denied")
	errDeviceBusy       = fmt.Errorf("device busy")
	errEAGAIN           = fmt.Errorf("resource temporarily unavailable")
	errInvalidParameter = fmt.Errorf("invalid parameter")
	errIO               = fmt.Errorf("I/O error")
	errNoDevice         = fmt.Errorf("no device")
	errNotFound         = fmt.Errorf("not found")
	errBusy             = fmt.Errorf("busy")
	errTimeout          = fmt.Errorf("timeout")
	errOverflow         = fmt.Errorf("overflow")
	errPipe             = fmt.Errorf("pipe error")
	errInterrupted      = fmt.Errorf("interrupted")
	errNoMem            = fmt.Errorf("no memory")
	errNotSupported     = fmt.Errorf("not supported")
	errOther            = fmt.Errorf("other error")
)
