package main

import "testing"

func TestValidateFilename(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		wantOK bool
	}{
		{"normal file", "vacation.jpg", true},
		{"with spaces and dashes", "my report-final v2.pdf", true},
		{"empty is rejected", "", false},
		{"path traversal is rejected", "../secret.txt", false},
		{"leading slash is rejected", "/etc/passwd", false},
		{"weird characters rejected", "rm -rf ~; echo $HOME.txt", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFilename(tc.input)
			gotOK := err == nil
			if gotOK != tc.wantOK {
				t.Errorf("validateFilename(%q): got allowed=%v, wanted allowed=%v (error was: %v)",
					tc.input, gotOK, tc.wantOK, err)
			}
		})
	}
}
