//go:build windows

package usb

type devicePathTestCase struct {
	path  string
	valid bool
}

func getDevicePathTestCases() []devicePathTestCase {
	return []devicePathTestCase{
		{`\\?\usb#vid_1234&pid_5678#...`, true},
		{`\\?\USB#VID_ABCD&PID_EF01#...`, true},
		{`\\?\usb#vid_1234&pid_5678`, true},
		{`/dev/bus/usb/001/001`, false},
		{`C:\some\path`, false},
		{"", false},
	}
}
