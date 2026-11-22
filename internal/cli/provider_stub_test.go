package cli

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// discoveryStub is a local OpenID Connect provider. It publishes a discovery
// document, signing keys, and a token endpoint, so tests exercise sign-in
// without contacting a real provider.
type discoveryStub struct {
	server   *httptest.Server
	metadata map[string]any
	requests atomic.Int32
	key      *rsa.PrivateKey

	mu     sync.Mutex
	nonce  string
	claims map[string]any
}

func newDiscoveryStub(t *testing.T) *discoveryStub {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating provider key: %v", err)
	}

	stub := &discoveryStub{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", stub.discovery)
	mux.HandleFunc("/keys", stub.keys)
	mux.HandleFunc("/token", stub.token)
	stub.server = httptest.NewServer(mux)
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *discoveryStub) discovery(w http.ResponseWriter, r *http.Request) {
	s.requests.Add(1)

	metadata := map[string]any{
		"issuer":                 s.server.URL,
		"authorization_endpoint": s.server.URL + "/authorize",
		"token_endpoint":         s.server.URL + "/token",
		"jwks_uri":               s.server.URL + "/keys",
	}
	for key, value := range s.metadata {
		if value == "" {
			delete(metadata, key)
			continue
		}
		metadata[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metadata); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *discoveryStub) keys(w http.ResponseWriter, r *http.Request) {
	exponent := big.NewInt(int64(s.key.PublicKey.E))
	response := map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA",
			"kid": "test",
			"use": "sig",
			"alg": "RS256",
			"n":   base64.RawURLEncoding.EncodeToString(s.key.PublicKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(exponent.Bytes()),
		}},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *discoveryStub) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	nonce := s.nonce
	claims := map[string]any{
		"iss":   s.server.URL,
		"aud":   "invoicer",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"nonce": nonce,
	}
	for name, value := range s.claims {
		claims[name] = value
	}
	s.mu.Unlock()

	response := map[string]any{
		"access_token": "access",
		"token_type":   "Bearer",
		"id_token":     s.sign(claims),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *discoveryStub) sign(claims map[string]any) string {
	header := encodeStubJSON(map[string]any{"alg": "RS256", "kid": "test", "typ": "JWT"})
	payload := encodeStubJSON(claims)
	signingInput := header + "." + payload

	digest := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, digest[:])
	if err != nil {
		panic(err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (s *discoveryStub) prepareNonce(nonce string, claims map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonce = nonce
	s.claims = claims
}

func encodeStubJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
