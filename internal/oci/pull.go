package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func Pull(ctx context.Context, ref string, token string) ([]byte, error) {
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return nil, fmt.Errorf("parsing reference %q: %w", ref, err)
	}

	repo.Client = &auth.Client{
		Credential: auth.StaticCredential(repo.Reference.Registry, auth.Credential{
			Username: getUsername(),
			Password: token,
		}),
	}

	store := memory.New()
	tag := repo.Reference.Reference

	desc, err := oras.Copy(ctx, repo, tag, store, tag, oras.DefaultCopyOptions)
	if err != nil {
		return nil, fmt.Errorf("pulling from %s: %w", ref, err)
	}

	rc, err := store.Fetch(ctx, desc)
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	manifestBytes, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	if len(manifest.Layers) == 0 {
		return nil, fmt.Errorf("no layers in manifest")
	}

	layerRC, err := store.Fetch(ctx, manifest.Layers[0])
	if err != nil {
		return nil, fmt.Errorf("fetching layer: %w", err)
	}
	defer layerRC.Close()

	data, err := io.ReadAll(layerRC)
	if err != nil {
		return nil, fmt.Errorf("reading layer: %w", err)
	}

	return data, nil
}

func ListTags(ctx context.Context, ref string, token string) ([]string, error) {
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return nil, fmt.Errorf("parsing reference %q: %w", ref, err)
	}

	repo.Client = &auth.Client{
		Credential: auth.StaticCredential(repo.Reference.Registry, auth.Credential{
			Username: getUsername(),
			Password: token,
		}),
	}

	var tags []string
	err = repo.Tags(ctx, "", func(t []string) error {
		tags = append(tags, t...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}

	return tags, nil
}

func GetDigest(ctx context.Context, ref string, token string) (string, error) {
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return "", fmt.Errorf("parsing reference %q: %w", ref, err)
	}

	repo.Client = &auth.Client{
		Credential: auth.StaticCredential(repo.Reference.Registry, auth.Credential{
			Username: getUsername(),
			Password: token,
		}),
	}

	tag := repo.Reference.Reference
	desc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", ref, err)
	}

	return string(desc.Digest), nil
}

func getUsername() string {
	if actor := os.Getenv("GITHUB_ACTOR"); actor != "" {
		return actor
	}
	return "enbu"
}
