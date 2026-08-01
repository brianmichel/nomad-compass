package repo

import (
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/brianmichel/nomad-compass/internal/storage"
)

func TestAuthMethodForCredentialHandlesSupportedAndInvalidCredentials(t *testing.T) {
	method, err := authMethodForCredential(nil, nil)
	if err != nil || method != nil {
		t.Fatalf("nil credential = %#v, %v; want nil, nil", method, err)
	}

	method, err = authMethodForCredential(&storage.Credential{Type: storage.CredentialTypeHTTPToken}, &storage.CredentialPayload{Token: "token"})
	if err != nil {
		t.Fatalf("HTTP token auth: %v", err)
	}
	httpAuth, ok := method.(*githttp.BasicAuth)
	if !ok || httpAuth.Username != "token" || httpAuth.Password != "token" {
		t.Fatalf("default HTTP auth = %#v", method)
	}

	method, err = authMethodForCredential(&storage.Credential{Type: storage.CredentialTypeHTTPToken}, &storage.CredentialPayload{Username: "alice", Token: "token"})
	if err != nil {
		t.Fatalf("named HTTP token auth: %v", err)
	}
	httpAuth = method.(*githttp.BasicAuth)
	if httpAuth.Username != "alice" {
		t.Fatalf("named HTTP username = %q", httpAuth.Username)
	}

	if _, err := authMethodForCredential(&storage.Credential{Type: "unsupported"}, &storage.CredentialPayload{}); err == nil {
		t.Fatal("expected unsupported credential type error")
	}
	if _, err := authMethodForCredential(&storage.Credential{Type: storage.CredentialTypeSSHKey}, &storage.CredentialPayload{PrivateKey: "not a key"}); err == nil {
		t.Fatal("expected invalid SSH key error")
	}
}

func TestParseSSHKeyRejectsMalformedAndWrongPassphrase(t *testing.T) {
	if _, err := parseSSHKey("not a private key", ""); err == nil {
		t.Fatal("expected malformed key error")
	}
	if _, err := parseSSHKey("not a private key", "passphrase"); err == nil {
		t.Fatal("expected malformed encrypted key error")
	}
}
