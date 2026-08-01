package cli

import "testing"

func TestValidateCredentialInputRejectsMissingAndUnsupportedValues(t *testing.T) {
	for _, tc := range []struct {
		name           string
		credentialType string
		token          string
		privateKey     string
		wantError      bool
	}{
		{"valid HTTPS token", "https-token", "token", "", false},
		{"blank HTTPS token", "https-token", "  \n", "", true},
		{"valid SSH key", "ssh-key", "", "private-key", false},
		{"blank SSH key", "ssh-key", "", " \t", true},
		{"unsupported type", "oauth", "token", "key", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCredentialInput(tc.credentialType, tc.token, tc.privateKey)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, wantError = %v", err, tc.wantError)
			}
		})
	}
}
