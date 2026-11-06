package aiconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coolerks/Fealdhord/internal/database"
)

type testCredentials map[string]string

func (c testCredentials) Get(ref string) (string, error) {
	value, ok := c[ref]
	if !ok {
		return "", errors.New("missing")
	}
	return value, nil
}

func (c testCredentials) Set(ref, key string) error {
	if key == "reject" {
		return errors.New("system rejected write")
	}
	c[ref] = key
	return nil
}

func (c testCredentials) Delete(ref string) error {
	delete(c, ref)
	return nil
}

func TestProviderLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	credentials := testCredentials{}
	service := New(db.AIProviderQueries(), credentials)
	key := "secret-first"
	input := ProviderInput{Provider: Provider{Name: "Local", Protocol: "responses", BaseURL: "http://localhost:8080", Enabled: true}, APIKey: &key}
	p, err := service.SaveProvider(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if p.BaseURL != "http://localhost:8080/v1" || !p.HasCredential || len(credentials) != 1 {
		t.Fatalf("provider = %+v", p)
	}
	model, err := service.SaveModel(ctx, Model{ProviderID: p.ID, Name: "example", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Assign(ctx, "chat", model.ID); err != nil {
		t.Fatal(err)
	}
	_, adapter, err := service.Resolve(ctx, "chat")
	if err != nil || adapter == nil {
		t.Fatalf("resolve = %v", err)
	}
	input = ProviderInput{Provider: p}
	input.BaseURL = "https://different.example/v1"
	if _, err := service.SaveProvider(ctx, input); err == nil || err.Error() != "CredentialRequired" {
		t.Fatalf("changed origin: %v", err)
	}
	input.Provider = p
	rejected := "reject"
	input.APIKey = &rejected
	if _, err := service.SaveProvider(ctx, input); err == nil || err.Error() != "CredentialError" {
		t.Fatalf("rejected key: %v", err)
	}
	second := "secret-second"
	input.APIKey = &second
	input.Enabled = false
	updated, err := service.SaveProvider(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range credentials {
		if value != second {
			t.Fatal("old credential retained")
		}
	}
	if len(credentials) != 1 {
		t.Fatal("replacement credential missing")
	}
	if _, err := service.SaveProvider(ctx, input); err == nil || err.Error() != "Conflict" {
		t.Fatalf("stale write = %v", err)
	}
	if _, _, err := service.Resolve(ctx, "chat"); err == nil || err.Error() != "ConfigurationError" {
		t.Fatalf("disabled resolution = %v", err)
	}
	snapshot, err := service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for ref := range credentials {
		if strings.Contains(string(wire), ref) || strings.Contains(string(wire), second) {
			t.Fatal("credential exposed")
		}
	}
	if err := service.DeleteProvider(ctx, p.ID, updated.Revision); err != nil {
		t.Fatal(err)
	}
	snapshot, err = service.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Providers)+len(snapshot.Models)+len(snapshot.Assignments)+len(credentials) != 0 {
		t.Fatal("provider deletion left dependent configuration")
	}
}

func TestConnectionUsesConfiguredModel(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("unexpected connection request: %s", r.URL.Path)
		}
		var body struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "example" || body.MaxTokens != 32 {
			t.Errorf("unexpected payload: %+v, %v", body, err)
		}
		fmt.Fprint(w, `{"type":"message","role":"assistant","content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn"}`)
	}))
	defer server.Close()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := New(db.AIProviderQueries(), testCredentials{})
	key := "test-key"
	p, err := service.SaveProvider(ctx, ProviderInput{Provider: Provider{Name: "test", Protocol: "messages", Enabled: true, BaseURL: server.URL}, APIKey: &key})
	if err != nil {
		t.Fatal(err)
	}
	m, err := service.SaveModel(ctx, Model{ProviderID: p.ID, Name: "example", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.TestConnection(ctx, m.ID); err != nil {
		t.Fatal(err)
	}
}
