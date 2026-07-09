package app

import (
	"context"
	"strings"
)

type RecipientInfo struct {
	Username    string
	Fingerprint string
	PublicKey   string
}

func (a *App) ListRecipients(ctx context.Context) ([]RecipientInfo, error) {
	accessToken, _, err := a.TokenProvider.LoadToken()
	if err != nil {
		return nil, err
	}
	owner, repo, err := a.RepoDetector.LoadRepo()
	if err != nil {
		return nil, err
	}

	ref := a.registryRef(owner, repo)
	tags, err := a.Registry.ListTags(ctx, ref, accessToken)
	if err != nil {
		return nil, err
	}

	var results []RecipientInfo
	for _, tag := range tags {
		if !IsUserRecipientTag(tag) {
			continue
		}
		tagRef := ref + ":" + tag
		data, err := a.Registry.Pull(ctx, tagRef, accessToken)
		if err != nil {
			continue
		}
		// tag format: recipient-{username}-{fingerprint}
		without := strings.TrimPrefix(tag, RecipientTagPrefix())
		lastDash := strings.LastIndex(without, "-")
		if lastDash < 0 {
			continue
		}
		results = append(results, RecipientInfo{
			Username:    without[:lastDash],
			Fingerprint: without[lastDash+1:],
			PublicKey:   strings.TrimSpace(string(data)),
		})
	}
	return results, nil
}
