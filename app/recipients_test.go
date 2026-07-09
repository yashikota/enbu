package app

import (
	"context"
	"strings"
	"testing"
)

func TestListRecipients(t *testing.T) {
	reg := newMemRegistry()
	a := &App{
		Registry:      reg,
		TokenProvider: &staticTokenProvider{token: "tok", username: "alice"},
		RepoDetector:  &staticRepoDetector{owner: "alice", repo: "myrepo"},
	}

	ref := "ghcr.io/alice/myrepo-enbu"
	pubKey1 := "age1qyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqysqqp"
	pubKey2 := "age1qyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqysqqa"
	tag1 := "recipient-alice-aabbccdd"
	tag2 := "recipient-bob-11223344"
	if err := reg.Push(context.Background(), ref+":"+tag1, "application/vnd.enbu.recipient.age.v1", []byte(pubKey1), "tok", nil); err != nil {
		t.Fatal(err)
	}
	if err := reg.Push(context.Background(), ref+":"+tag2, "application/vnd.enbu.recipient.age.v1", []byte(pubKey2), "tok", nil); err != nil {
		t.Fatal(err)
	}

	recipients, err := a.ListRecipients(context.Background())
	if err != nil {
		t.Fatalf("ListRecipients: %v", err)
	}
	if len(recipients) != 2 {
		t.Fatalf("got %d recipients, want 2", len(recipients))
	}

	var aliceFound bool
	for _, r := range recipients {
		if r.Username == "alice" {
			aliceFound = true
			if r.Fingerprint != "aabbccdd" {
				t.Fatalf("alice fingerprint = %q, want aabbccdd", r.Fingerprint)
			}
			if !strings.Contains(r.PublicKey, "age1") {
				t.Fatalf("alice public key looks wrong: %q", r.PublicKey)
			}
		}
	}
	if !aliceFound {
		t.Fatal("alice not found in recipients")
	}
}
