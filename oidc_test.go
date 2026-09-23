package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func jwt(t *testing.T, alg, kid, issuer, aud string, sign func([]byte) []byte) string {
	t.Helper()
	head, _ := json.Marshal(map[string]any{"alg": alg, "kid": kid, "typ": "JWT"})
	body, _ := json.Marshal(map[string]any{
		"sub": "user-1", "email": "a@example.com", "given_name": "Ada",
		"iss": issuer, "aud": aud, "exp": time.Now().Add(time.Hour).Unix(),
	})
	signing := b64(head) + "." + b64(body)
	return signing + "." + b64(sign([]byte(signing)))
}

// A provider that is deliberately NOT Authentik: its issuer has no trailing slash
// and its JWKS lives somewhere other than <issuer>/jwks/.
func fakeProvider(t *testing.T, jwks map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":   srv.URL, // no trailing slash
			"jwks_uri": srv.URL + "/some/other/keys",
		})
	})
	mux.HandleFunc("/some/other/keys", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	})
	return srv
}

func TestAcceptsRS256FromNonAuthentikProvider(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := fakeProvider(t, map[string]any{"keys": []map[string]any{{
		"kid": "rsa-1", "kty": "RSA",
		"n": b64(key.N.Bytes()),
		"e": b64(big.NewInt(int64(key.E)).Bytes()),
	}}})
	defer srv.Close()

	a := newAuth(srv.URL, "tidyshop-android")
	token := jwt(t, "RS256", "rsa-1", srv.URL, "tidyshop-android", func(b []byte) []byte {
		sum := sha256.Sum256(b)
		sig, _ := rsa.SignPKCS1v15(rand.Reader, key, cryptoHashSHA256, sum[:])
		return sig
	})

	req := httptest.NewRequest("GET", "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	c, err := a.claims(req)
	if err != nil {
		t.Fatalf("token rejected: %v", err)
	}
	if c.Sub != "user-1" || c.Email != "a@example.com" {
		t.Fatalf("claims wrong: %+v", c)
	}
	// The JWKS must have been found via discovery, not by guessing /jwks/.
	if a.jwksURL != srv.URL+"/some/other/keys" {
		t.Fatalf("jwks_uri not taken from discovery: %q", a.jwksURL)
	}
	if a.issuer != srv.URL {
		t.Fatalf("issuer not taken from discovery: %q", a.issuer)
	}
}

func TestAcceptsES256(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pad := func(i *big.Int) []byte {
		b := i.Bytes()
		out := make([]byte, 32)
		copy(out[32-len(b):], b)
		return out
	}
	srv := fakeProvider(t, map[string]any{"keys": []map[string]any{{
		"kid": "ec-1", "kty": "EC", "crv": "P-256",
		"x": b64(pad(key.X)), "y": b64(pad(key.Y)),
	}}})
	defer srv.Close()

	a := newAuth(srv.URL, "tidyshop-android")
	token := jwt(t, "ES256", "ec-1", srv.URL, "tidyshop-android", func(b []byte) []byte {
		sum := sha256.Sum256(b)
		r, ss, _ := ecdsa.Sign(rand.Reader, key, sum[:])
		return append(pad(r), pad(ss)...)
	})

	req := httptest.NewRequest("GET", "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if _, err := a.claims(req); err != nil {
		t.Fatalf("ES256 token rejected: %v", err)
	}
}

func TestRejectsWrongIssuerAndAudience(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	srv := fakeProvider(t, map[string]any{"keys": []map[string]any{{
		"kid": "rsa-1", "kty": "RSA",
		"n": b64(key.N.Bytes()), "e": b64(big.NewInt(int64(key.E)).Bytes()),
	}}})
	defer srv.Close()
	sign := func(b []byte) []byte {
		sum := sha256.Sum256(b)
		sig, _ := rsa.SignPKCS1v15(rand.Reader, key, cryptoHashSHA256, sum[:])
		return sig
	}

	for _, tc := range []struct{ name, iss, aud string }{
		{"someone else's issuer", "https://evil.example.com", "tidyshop-android"},
		{"another app's audience", "", "some-other-client"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			iss := tc.iss
			if iss == "" {
				iss = srv.URL
			}
			a := newAuth(srv.URL, "tidyshop-android")
			req := httptest.NewRequest("GET", "/v1/me", nil)
			req.Header.Set("Authorization", "Bearer "+jwt(t, "RS256", "rsa-1", iss, tc.aud, sign))
			if _, err := a.claims(req); err == nil {
				t.Fatal("token should have been rejected")
			}
		})
	}
}
