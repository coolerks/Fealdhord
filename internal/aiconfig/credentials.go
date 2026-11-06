package aiconfig

import "github.com/zalando/go-keyring"

type CredentialStore interface {
	Get(string) (string, error)
	Set(string, string) error
	Delete(string) error
}

type SystemCredentials struct{}

const credentialService = "Fealdhord AI"

func (SystemCredentials) Get(ref string) (string, error) {
	return keyring.Get(credentialService, ref)
}

func (SystemCredentials) Set(ref, key string) error {
	return keyring.Set(credentialService, ref, key)
}

func (SystemCredentials) Delete(ref string) error {
	return keyring.Delete(credentialService, ref)
}
