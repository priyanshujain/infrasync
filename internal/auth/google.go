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

// GoogleAuthOptions contains options for authenticating with Google Cloud
type GoogleAuthOptions struct {
	// Direct JSON credentials
	CredentialsJSON []byte
	// Path to credentials file
	CredentialsFile string
	// Environment variable name (e.g. GOOGLE_APPLICATION_CREDENTIALS)
	CredentialsEnvVar string
}

// NewGoogleClient creates a new Google Cloud client with explicit credentials
func NewGoogleClient(ctx context.Context, opts GoogleAuthOptions) (*http.Client, error) {
	var credsJSON []byte
	var err error

	// Priority order: Direct JSON > File > Environment var
	if len(opts.CredentialsJSON) > 0 {
		credsJSON = opts.CredentialsJSON
	} else if opts.CredentialsFile != "" {
		// Read from file
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

	// Create client from credentials JSON
	creds, err := google.CredentialsFromJSON(ctx, credsJSON,
		"https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	// Create client using the token source
	client := oauth2.NewClient(ctx, creds.TokenSource)
	return client, nil
}