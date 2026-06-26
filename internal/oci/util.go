package oci

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/opencontainers/go-digest"
)

func digestOf(data []byte) digest.Digest {
	h := sha256.Sum256(data)
	return digest.Digest(fmt.Sprintf("sha256:%x", h))
}

func bytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}
