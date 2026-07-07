package main

import "testing"

func TestShouldLaunchGUIByDefault(t *testing.T) {
	tests := []struct {
		name string
		goos string
		args []string
		want bool
	}{
		{name: "windows no args", goos: "windows", args: []string{"enbu.exe"}, want: true},
		{name: "windows with command", goos: "windows", args: []string{"enbu.exe", "pull"}, want: false},
		{name: "linux no args", goos: "linux", args: []string{"enbu"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldLaunchGUIByDefault(tt.goos, tt.args); got != tt.want {
				t.Fatalf("shouldLaunchGUIByDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}
