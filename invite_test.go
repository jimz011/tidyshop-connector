package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newAPIWithHousehold(t *testing.T) *API {
	t.Helper()
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub(), pairingKey: "master-key-long-enough"}
	api.s.Households["H1"] = Household{ID: "H1", Name: "Family",
		Members: []Member{{User: User{ID: "alice"}, Role: "owner"}}}
	api.s.KnownUsers["alice"] = User{ID: "alice"}
	return api
}

func mintInvite(t *testing.T, api *API, by string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	api.createInvite(rec, httptest.NewRequest("POST", "/v1/invites", strings.NewReader(`{}`)), User{ID: by})
	if rec.Code != 201 {
		t.Fatalf("createInvite: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Code      string `json:"code"`
		ExpiresAt int64  `json:"expiresAt"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Code) != 8 {
		t.Fatalf("code %q should be 8 characters", out.Code)
	}
	if out.ExpiresAt <= time.Now().UnixMilli() {
		t.Fatal("invite should expire in the future")
	}
	return out.Code
}

func TestInviteAdmitsOneMemberThenIsSpent(t *testing.T) {
	api := newAPIWithHousehold(t)
	code := mintInvite(t, api, "alice")

	// Bob redeems it and lands in Alice's household without the master key.
	rec := httptest.NewRecorder()
	api.pair(rec, httptest.NewRequest("POST", "/v1/pair",
		strings.NewReader(`{"InviteCode":"`+code+`"}`)), User{ID: "bob", Email: "bob@example.com"})
	if rec.Code != 200 {
		t.Fatalf("bob could not redeem: %d %s", rec.Code, rec.Body.String())
	}
	if !member(api.s.Households["H1"], "bob") {
		t.Fatal("bob is not in the household")
	}
	if len(api.s.Households) != 1 {
		t.Fatalf("an invite must join the named household, not create one: %+v", api.s.Households)
	}

	// The same code must not work twice.
	rec2 := httptest.NewRecorder()
	api.pair(rec2, httptest.NewRequest("POST", "/v1/pair",
		strings.NewReader(`{"InviteCode":"`+code+`"}`)), User{ID: "mallory"})
	if rec2.Code != 403 {
		t.Fatalf("reused invite should be rejected, got %d", rec2.Code)
	}
	if member(api.s.Households["H1"], "mallory") {
		t.Fatal("mallory got in with a spent invite")
	}
}

func TestInviteCodeIsCaseAndSeparatorInsensitive(t *testing.T) {
	api := newAPIWithHousehold(t)
	code := mintInvite(t, api, "alice")
	messy := strings.ToLower(code[:4] + "-" + code[4:])

	rec := httptest.NewRecorder()
	api.pair(rec, httptest.NewRequest("POST", "/v1/pair",
		strings.NewReader(`{"InviteCode":"`+messy+`"}`)), User{ID: "bob"})
	if rec.Code != 200 {
		t.Fatalf("typed-in code %q rejected: %d %s", messy, rec.Code, rec.Body.String())
	}
}

func TestExpiredInviteRejected(t *testing.T) {
	api := newAPIWithHousehold(t)
	code := mintInvite(t, api, "alice")
	inv := api.s.Invites[code]
	inv.ExpiresAt = time.Now().UnixMilli() - 1
	api.s.Invites[code] = inv

	rec := httptest.NewRecorder()
	api.pair(rec, httptest.NewRequest("POST", "/v1/pair",
		strings.NewReader(`{"InviteCode":"`+code+`"}`)), User{ID: "bob"})
	if rec.Code != 403 {
		t.Fatalf("expired invite should be rejected, got %d", rec.Code)
	}
}

func TestRevokedInviteRejectedAndOnlyOwnHouseholdCanRevoke(t *testing.T) {
	api := newAPIWithHousehold(t)
	api.s.Households["H2"] = Household{ID: "H2", Members: []Member{{User: User{ID: "carol"}}}}
	api.s.KnownUsers["carol"] = User{ID: "carol"}
	code := mintInvite(t, api, "alice")

	// Carol is in a different household and must not be able to revoke it.
	rec := httptest.NewRecorder()
	api.revokeInvite(rec, httptest.NewRequest("DELETE", "/v1/invites/"+code, nil), User{ID: "carol"})
	if rec.Code != 403 {
		t.Fatalf("outsider revoke should be 403, got %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	api.revokeInvite(rec2, httptest.NewRequest("DELETE", "/v1/invites/"+code, nil), User{ID: "alice"})
	if rec2.Code != 200 {
		t.Fatalf("owner revoke failed: %d %s", rec2.Code, rec2.Body.String())
	}

	rec3 := httptest.NewRecorder()
	api.pair(rec3, httptest.NewRequest("POST", "/v1/pair",
		strings.NewReader(`{"InviteCode":"`+code+`"}`)), User{ID: "bob"})
	if rec3.Code != 403 {
		t.Fatalf("revoked invite should be rejected, got %d", rec3.Code)
	}
}

// The master key stays as a way back in if invites are unusable.
func TestMasterPairingKeyStillWorks(t *testing.T) {
	api := newAPIWithHousehold(t)
	rec := httptest.NewRecorder()
	api.pair(rec, httptest.NewRequest("POST", "/v1/pair",
		strings.NewReader(`{"PairingKey":"master-key-long-enough"}`)), User{ID: "bob"})
	if rec.Code != 200 {
		t.Fatalf("master key pairing broke: %d %s", rec.Code, rec.Body.String())
	}
	if !member(api.s.Households["H1"], "bob") {
		t.Fatal("bob not enrolled via master key")
	}

	bad := httptest.NewRecorder()
	api.pair(bad, httptest.NewRequest("POST", "/v1/pair",
		strings.NewReader(`{"PairingKey":"wrong"}`)), User{ID: "mallory"})
	if bad.Code != 403 {
		t.Fatalf("wrong master key should be rejected, got %d", bad.Code)
	}
}

func TestCannotInviteWithoutAHousehold(t *testing.T) {
	api := newAPIWithHousehold(t)
	rec := httptest.NewRecorder()
	api.createInvite(rec, httptest.NewRequest("POST", "/v1/invites", strings.NewReader(`{}`)), User{ID: "nobody"})
	if rec.Code != 403 {
		t.Fatalf("stranger should not mint invites, got %d", rec.Code)
	}
}
