package age

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"filippo.io/age"
	"filippo.io/age/agessh"
)

func Encrypt(plaintext []byte, recipients ...age.Recipient) ([]byte, error) {
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipients...)
	if err != nil {
		return nil, fmt.Errorf("creating age writer: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("writing plaintext: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing age writer: %w", err)
	}
	return buf.Bytes(), nil
}

func EncryptForPublicKeys(plaintext []byte, publicKeys []string) ([]byte, error) {
	recipients := make([]age.Recipient, 0, len(publicKeys))
	for _, pk := range publicKeys {
		var r age.Recipient
		var err error
		if strings.HasPrefix(pk, "age1") {
			r, err = age.ParseX25519Recipient(pk)
		} else if strings.HasPrefix(pk, "ssh-") {
			r, err = agessh.ParseRecipient(pk)
		} else {
			return nil, fmt.Errorf("unsupported public key format: %q", pk)
		}
		if err != nil {
			return nil, fmt.Errorf("parsing public key %q: %w", pk, err)
		}
		recipients = append(recipients, r)
	}
	return Encrypt(plaintext, recipients...)
}

func Decrypt(ciphertext []byte, identities ...age.Identity) ([]byte, error) {
	r, err := age.Decrypt(bytes.NewReader(ciphertext), identities...)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}
	return io.ReadAll(r)
}
