package utils

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func installationTestKey(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func testInstallationTransport(t *testing.T, app, installation int64, key []byte, url string) *cachedInstallationTransport {
	t.Helper()
	transport, err := getInstallationTransport(app, installation, key)
	if err != nil {
		t.Fatal(err)
	}
	transport.BaseURL = url
	t.Cleanup(func() { installationTransports.CompareAndDelete(transport.key, transport) })
	return transport
}

func TestInstallationTokenReuseConcurrent(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != "POST" || r.URL.Path != "/app/installations/42/access_tokens" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"token": "test-token", "expires_at": time.Now().Add(time.Hour)})
	}))
	defer server.Close()
	key := installationTestKey(t)
	t.Setenv("GITHUB_APP_PRIVATE_KEY_BASE64", base64.StdEncoding.EncodeToString(key))
	testInstallationTransport(t, 1, 42, key, server.URL)
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, token, err := (DiggerGithubRealClientProvider{}).Get(1, 42)
			if err != nil {
				t.Error(err)
				return
			}
			if token == nil || *token != "test-token" {
				t.Error("unexpected installation token")
			}
		}()
	}
	workers.Wait()
	if got := requests.Load(); got != 1 {
		t.Fatalf("token requests = %d, want 1", got)
	}
}

func TestInstallationTokenIdentityIsolation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		json.NewEncoder(w).Encode(map[string]interface{}{"token": "test-token", "expires_at": time.Now().Add(time.Hour)})
	}))
	defer server.Close()
	key, rotatedKey := installationTestKey(t), installationTestKey(t)
	cases := []struct {
		app, installation int64
		key               []byte
	}{{1, 42, key}, {2, 42, key}, {1, 43, key}, {1, 42, rotatedKey}}
	for _, c := range cases {
		transport := testInstallationTransport(t, c.app, c.installation, c.key, server.URL)
		if _, err := transport.Token(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := requests.Load(); got != 4 {
		t.Fatalf("token requests = %d, want 4", got)
	}
}

func TestInstallationTokenRefreshAndRevocation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/resource" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		expiry := time.Now().Add(time.Hour)
		if requests.Add(1) == 1 {
			expiry = time.Now().Add(30 * time.Second)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"token": "test-token", "expires_at": expiry})
	}))
	defer server.Close()
	key := installationTestKey(t)
	transport := testInstallationTransport(t, 1, 42, key, server.URL)
	for i := 0; i < 2; i++ {
		if _, err := transport.Token(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("token requests after expiry = %d, want 2", got)
	}
	request, err := http.NewRequest(http.MethodGet, server.URL+"/resource", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.StatusCode)
	}
	replacement := testInstallationTransport(t, 1, 42, key, server.URL)
	if replacement == transport {
		t.Fatal("revoked transport was reused")
	}
	if _, err := replacement.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 3 {
		t.Fatalf("token requests after revocation = %d, want 3", got)
	}
	response, err = transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	current, err := getInstallationTransport(1, 42, key)
	if err != nil {
		t.Fatal(err)
	}
	if current != replacement {
		t.Fatal("a late 401 from the old transport evicted its replacement")
	}
}
