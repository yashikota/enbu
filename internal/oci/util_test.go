package oci

import (
	"io"
	"testing"
)

func TestDigestOf(t *testing.T) {
	data := []byte("hello world")
	digest := digestOf(data)

	expected := "sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if string(digest) != expected {
		t.Errorf("digest = %q, want %q", digest, expected)
	}
}

func TestDigestOfEmpty(t *testing.T) {
	data := []byte("")
	digest := digestOf(data)

	expected := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if string(digest) != expected {
		t.Errorf("digest = %q, want %q", digest, expected)
	}
}

func TestBytesReader(t *testing.T) {
	data := []byte("test content")
	r := bytesReader(data)

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("got %q, want %q", got, data)
	}
}
