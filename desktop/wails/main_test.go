package main

import (
	"context"
	"reflect"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestShowWindow(t *testing.T) {
	var calls []string
	actions := windowActions{
		show:       func(context.Context) { calls = append(calls, "show") },
		unminimise: func(context.Context) { calls = append(calls, "unminimise") },
		center:     func(context.Context) { calls = append(calls, "center") },
	}

	showWindow(nil, actions) //nolint:staticcheck // The nil guard is intentional and requires direct coverage.
	if len(calls) != 0 {
		t.Fatalf("showWindow(nil) called actions: %v", calls)
	}

	showWindow(context.Background(), actions)
	want := []string{"show", "unminimise", "center"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("showWindow() calls = %v, want %v", calls, want)
	}
}

func TestActivationHandlers(t *testing.T) {
	ctx := context.Background()
	var activated []context.Context
	onSecondInstanceLaunch, onURLOpen := activationHandlers(
		func() context.Context { return ctx },
		func(got context.Context) { activated = append(activated, got) },
	)

	onSecondInstanceLaunch(options.SecondInstanceData{Args: []string{"enbu://auth/complete"}})
	onURLOpen("enbu://auth/complete")

	if len(activated) != 2 || activated[0] != ctx || activated[1] != ctx {
		t.Fatalf("activation contexts = %v, want the provided context twice", activated)
	}
}
