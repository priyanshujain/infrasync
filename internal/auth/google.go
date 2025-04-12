package auth

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleAuthOptions struct {
	CredentialsJSON   []byte
	CredentialsFile   string
	CredentialsEnvVar string
}

func NewGoogleClient(ctx context.Context, opts GoogleAuthOptions) (*http.Client, error) {
	var credsJSON []byte
	var err error

	if len(opts.CredentialsJSON) > 0 {
		credsJSON = opts.CredentialsJSON
	} else if opts.CredentialsFile != "" {
		credsJSON, err = ioutil.ReadFile(opts.CredentialsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read credentials file: %w", err)
		}
	} else if opts.CredentialsEnvVar != "" {
		envPath := os.Getenv(opts.CredentialsEnvVar)
		if envPath != "" {
			credsJSON, err = ioutil.ReadFile(envPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read credentials from env var path: %w", err)
			}
		}
	} else {
		return nil, fmt.Errorf("no credentials provided")
	}

	creds, err := google.CredentialsFromJSON(ctx, credsJSON,
		"https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	client := oauth2.NewClient(ctx, creds.TokenSource)
	return client, nil
}
