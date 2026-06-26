package oci

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"

	"github.com/opencontainers/go-digest"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func digestOf(data []byte) digest.Digest {
	h := sha256.Sum256(data)
	return digest.Digest(fmt.Sprintf("sha256:%x", h))
}

func bytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}

func newRepository(ref string, token string) (*remote.Repository, error) {
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return nil, fmt.Errorf("parsing reference %q: %w", ref, err)
	}

	registry := repo.Reference.Registry
	if strings.HasPrefix(registry, "localhost:") || registry == "localhost" {
		repo.PlainHTTP = true
	}

	repo.Client = &auth.Client{
		Credential: auth.StaticCredential(registry, auth.Credential{
			Username: getUsername(),
			Password: token,
		}),
	}

	return repo, nil
}
