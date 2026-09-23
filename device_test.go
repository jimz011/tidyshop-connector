package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func pad32(i *big.Int) []byte {
	b := i.Bytes()
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

// A stand-in for the phone: an EC P-256 key that signs assertions.
type fakeDevice struct {
	key *ecdsa.PrivateKey
	id  string
}

func newFakeDevice(t *testing.T) *fakeDevice {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &fakeDevice{key: key}
}

func (d *fakeDevice) jwk() map[string]string {
	return map[string]string{
		"Kty": "EC", "Crv": "P-256",
		"X": base64.RawURLEncoding.EncodeToString(pad32(d.key.X)),
		"Y": base64.RawURLEncoding.EncodeToString(pad32(d.key.Y)),
	}
}

func (d *fakeDevice) assert(t *testing.T, userID string, mutate func(map[string]any)) string {
	t.Helper()
	now := time.Now().Unix()
	head, _ := json.Marshal(map[string]any{"alg": "ES256", "typ": "JWT", "kid": d.id})
	claims := map[string]any{
		"iss": deviceIssuer, "sub": userID, "aud": deviceAudience,
		"iat": now, "exp": now + 120, "jti": fmt.Sprintf("jti-%d-%d", now, time.Now().UnixNano()),
	}
	if mutate != nil {
		mutate(claims)
	}
	body, _ := json.Marshal(claims)
	signing := base64.RawURLEncoding.EncodeToString(head) + "." + base64.RawURLEncoding.EncodeToString(body)
	sum := sha256.Sum256([]byte(signing))
	r, s, _ := ecdsa.Sign(rand.Reader, d.key, sum[:])
	return signing + "." + base64.RawURLEncoding.EncodeToString(append(pad32(r), pad32(s)...))
}

func enrol(t *testing.T, api *API, d *fakeDevice, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload["PublicKey"] = d.jwk()
	body, _ := json.Marshal(payload)
	rec := httptest.NewRecorder()
	api.enrolDevice(rec, httptest.NewRequest("POST", "/v1/devices/enrol", strings.NewReader(string(body))))
	if rec.Code == 201 {
		var out struct{ DeviceID string `json:"deviceId"` }
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		d.id = out.DeviceID
	}
	return rec
}

// A household with no identity provider at all: the whole point of the change.
func newDeviceAPI(t *testing.T) *API {
	t.Helper()
	return &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub(), pairingKey: "master-key-long-enough"}
}

func TestFirstDeviceEnrolsWithPairingKeyAndBecomesOwner(t *testing.T) {
	api := newDeviceAPI(t)
	phone := newFakeDevice(t)
	rec := enrol(t, api, phone, map[string]any{
		"PairingKey": "master-key-long-enough", "DisplayName": "Jimmy", "DeviceName": "Pixel 8",
	})
	if rec.Code != 201 {
		t.Fatalf("enrol: %d %s", rec.Code, rec.Body.String())
	}
	var out struct{ UserID string `json:"userId"` }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)

	// The assertion must now authenticate with no OIDC configured at all.
	req := httptest.NewRequest("GET", "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+phone.assert(t, out.UserID, nil))
	u, err := api.identify(req)
	if err != nil {
		t.Fatalf("device assertion rejected: %v", err)
	}
	if u.ID != out.UserID || u.FirstName != "Jimmy" {
		t.Fatalf("wrong identity: %+v", u)
	}
	for _, h := range api.s.Households {
		if h.Members[0].Role != "owner" {
			t.Fatalf("first member should own the household, got %q", h.Members[0].Role)
		}
	}
}

func TestLinkedDeviceJoinsTheSameIdentity(t *testing.T) {
	api := newDeviceAPI(t)
	phone := newFakeDevice(t)
	rec := enrol(t, api, phone, map[string]any{"PairingKey": "master-key-long-enough", "DisplayName": "Jimmy"})
	var first struct{ UserID string `json:"userId"` }
	_ = json.Unmarshal(rec.Body.Bytes(), &first)

	// Jimmy's tablet: a link code, not an invite.
	linkRec := httptest.NewRecorder()
	api.linkDevice(linkRec, httptest.NewRequest("POST", "/v1/devices/link", strings.NewReader(`{}`)), User{ID: first.UserID})
	if linkRec.Code != 201 {
		t.Fatalf("link: %d %s", linkRec.Code, linkRec.Body.String())
	}
	var link struct {
		Code string `json:"code"`
		Kind string `json:"kind"`
	}
	_ = json.Unmarshal(linkRec.Body.Bytes(), &link)
	if link.Kind != "device" {
		t.Fatalf("link code should be a device code, got %q", link.Kind)
	}

	tablet := newFakeDevice(t)
	rec2 := enrol(t, api, tablet, map[string]any{"Code": link.Code, "DeviceName": "Tab S9"})
	if rec2.Code != 201 {
		t.Fatalf("tablet enrol: %d %s", rec2.Code, rec2.Body.String())
	}
	var second struct{ UserID string `json:"userId"` }
	_ = json.Unmarshal(rec2.Body.Bytes(), &second)

	if second.UserID != first.UserID {
		t.Fatalf("a linked device must share the identity: %s vs %s", second.UserID, first.UserID)
	}
	for _, h := range api.s.Households {
		if len(h.Members) != 1 {
			t.Fatalf("linking a device must not add a member, household has %d", len(h.Members))
		}
	}
	if len(api.s.Devices) != 2 {
		t.Fatalf("expected two devices, got %d", len(api.s.Devices))
	}
}

func TestPlainInviteAddsASecondMember(t *testing.T) {
	api := newDeviceAPI(t)
	phone := newFakeDevice(t)
	rec := enrol(t, api, phone, map[string]any{"PairingKey": "master-key-long-enough", "DisplayName": "Jimmy"})
	var owner struct{ UserID string `json:"userId"` }
	_ = json.Unmarshal(rec.Body.Bytes(), &owner)

	code := mintInvite(t, api, owner.UserID)
	partner := newFakeDevice(t)
	rec2 := enrol(t, api, partner, map[string]any{"Code": code, "DisplayName": "Sam"})
	if rec2.Code != 201 {
		t.Fatalf("partner enrol: %d %s", rec2.Code, rec2.Body.String())
	}
	var second struct{ UserID string `json:"userId"` }
	_ = json.Unmarshal(rec2.Body.Bytes(), &second)
	if second.UserID == owner.UserID {
		t.Fatal("a plain invite must create a separate member, not reuse the inviter")
	}
	for _, h := range api.s.Households {
		if len(h.Members) != 2 {
			t.Fatalf("expected two members, got %d", len(h.Members))
		}
	}
}

func TestAssertionsAreRejectedWhenTheyShouldBe(t *testing.T) {
	api := newDeviceAPI(t)
	phone := newFakeDevice(t)
	rec := enrol(t, api, phone, map[string]any{"PairingKey": "master-key-long-enough", "DisplayName": "Jimmy"})
	var me struct{ UserID string `json:"userId"` }
	_ = json.Unmarshal(rec.Body.Bytes(), &me)

	check := func(name, token string) {
		t.Helper()
		req := httptest.NewRequest("GET", "/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		if _, err := api.identify(req); err == nil {
			t.Fatalf("%s: should have been rejected", name)
		}
	}

	check("expired", phone.assert(t, me.UserID, func(c map[string]any) {
		c["exp"] = time.Now().Unix() - 1
	}))
	check("lives too long", phone.assert(t, me.UserID, func(c map[string]any) {
		c["exp"] = time.Now().Unix() + 86400
	}))
	check("wrong audience", phone.assert(t, me.UserID, func(c map[string]any) {
		c["aud"] = "somebody-else"
	}))
	check("speaking for another user", phone.assert(t, me.UserID, func(c map[string]any) {
		c["sub"] = "someone-else"
	}))

	// A different key, replaying a valid device id.
	impostor := newFakeDevice(t)
	impostor.id = phone.id
	check("forged signature", impostor.assert(t, me.UserID, nil))
}

func TestAssertionCannotBeReplayed(t *testing.T) {
	api := newDeviceAPI(t)
	phone := newFakeDevice(t)
	rec := enrol(t, api, phone, map[string]any{"PairingKey": "master-key-long-enough", "DisplayName": "Jimmy"})
	var me struct{ UserID string `json:"userId"` }
	_ = json.Unmarshal(rec.Body.Bytes(), &me)

	token := phone.assert(t, me.UserID, nil)
	first := httptest.NewRequest("GET", "/v1/me", nil)
	first.Header.Set("Authorization", "Bearer "+token)
	if _, err := api.identify(first); err != nil {
		t.Fatalf("first use rejected: %v", err)
	}
	second := httptest.NewRequest("GET", "/v1/me", nil)
	second.Header.Set("Authorization", "Bearer "+token)
	if _, err := api.identify(second); err == nil {
		t.Fatal("the same assertion was accepted twice")
	}
}

func TestRevokedDeviceStopsWorkingImmediately(t *testing.T) {
	api := newDeviceAPI(t)
	phone := newFakeDevice(t)
	rec := enrol(t, api, phone, map[string]any{"PairingKey": "master-key-long-enough", "DisplayName": "Jimmy"})
	var me struct{ UserID string `json:"userId"` }
	_ = json.Unmarshal(rec.Body.Bytes(), &me)

	// Someone else must not be able to revoke it.
	other := httptest.NewRecorder()
	api.revokeDevice(other, httptest.NewRequest("DELETE", "/v1/devices/"+phone.id, nil), User{ID: "stranger"})
	if other.Code != 403 {
		t.Fatalf("stranger revoke returned %d, want 403", other.Code)
	}

	del := httptest.NewRecorder()
	api.revokeDevice(del, httptest.NewRequest("DELETE", "/v1/devices/"+phone.id, nil), User{ID: me.UserID})
	if del.Code != 200 {
		t.Fatalf("revoke failed: %d %s", del.Code, del.Body.String())
	}

	req := httptest.NewRequest("GET", "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+phone.assert(t, me.UserID, nil))
	if _, err := api.identify(req); err == nil {
		t.Fatal("a revoked device still authenticated")
	}
}

func TestEnrolNeedsAValidCodeAndAKey(t *testing.T) {
	api := newDeviceAPI(t)
	phone := newFakeDevice(t)

	if rec := enrol(t, api, phone, map[string]any{"PairingKey": "wrong"}); rec.Code != 403 {
		t.Fatalf("bad pairing key returned %d, want 403", rec.Code)
	}
	if rec := enrol(t, api, phone, map[string]any{"Code": "ZZZZ9999"}); rec.Code != 403 {
		t.Fatalf("unknown code returned %d, want 403", rec.Code)
	}

	body, _ := json.Marshal(map[string]any{
		"PairingKey": "master-key-long-enough",
		"PublicKey":  map[string]string{"Kty": "RSA"},
	})
	rec := httptest.NewRecorder()
	api.enrolDevice(rec, httptest.NewRequest("POST", "/v1/devices/enrol", strings.NewReader(string(body))))
	if rec.Code != 400 {
		t.Fatalf("non-EC key returned %d, want 400", rec.Code)
	}
}

// With no provider configured, a provider token must not be silently accepted.
func TestProviderTokenRejectedWhenSsoIsOff(t *testing.T) {
	api := newDeviceAPI(t)
	req := httptest.NewRequest("GET", "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+jwt(t, "RS256", "k", "https://issuer.example.com", "tidyshop-android",
		func(b []byte) []byte { return []byte("not-a-real-signature") }))
	if _, err := api.identify(req); err == nil {
		t.Fatal("a provider token was accepted with no provider configured")
	}
}
