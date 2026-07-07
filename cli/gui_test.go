package cli

import (
	"net"
	"testing"
)

func TestListenGUIWithPortZero(t *testing.T) {
	ln, err := listenGUI(0)
	if err != nil {
		t.Fatalf("listenGUI(0): %v", err)
	}
	defer func() { _ = ln.Close() }()

	if ln.Addr().(*net.TCPAddr).IP.String() != "127.0.0.1" {
		t.Fatalf("listen address = %s, want loopback", ln.Addr())
	}
}

func TestListenGUIFallsBackWhenDefaultPortIsBusy(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:3939")
	if err != nil {
		t.Skipf("default GUI port is already busy: %v", err)
	}
	defer func() { _ = busy.Close() }()

	ln, err := listenGUI(defaultPort)
	if err != nil {
		t.Fatalf("listenGUI(defaultPort): %v", err)
	}
	defer func() { _ = ln.Close() }()

	if ln.Addr().(*net.TCPAddr).Port == defaultPort {
		t.Fatalf("listenGUI used busy default port")
	}
}
