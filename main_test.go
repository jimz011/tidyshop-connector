package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func subCount(h *Hub) int { h.Lock(); defer h.Unlock(); return len(h.subs) }

func TestSSESnapshotAndFanout(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Lists["L1"] = List{ID: "L1", OwnerID: "alice", Title: "Groceries",
		Permissions: map[string]string{"bob": "edit"}}
	api.s.Lists["L9"] = List{ID: "L9", OwnerID: "carol", Title: "Private"}

	rec := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/v1/events", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() { api.events(rec, req, User{ID: "bob"}); close(done) }()

	// wait for the subscriber to register, then push a change
	for i := 0; i < 200 && subCount(api.hub) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	l := api.s.Lists["L1"]
	l.Title = "Groceries (updated)"
	api.emit("list.updated", recipients(l), map[string]any{"origin": "phone-2", "list": publicList(l)})
	time.Sleep(80 * time.Millisecond)
	cancel()
	<-done

	body := rec.Body.String()
	t.Logf("stream:\n%s", body)

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Error("missing X-Accel-Buffering: no (nginx would buffer the stream)")
	}
	if !strings.HasPrefix(body, "event: sync\ndata: ") {
		t.Error("expected a snapshot frame first")
	}
	if !strings.Contains(body, "event: list.updated\ndata: ") {
		t.Error("expected the pushed change to arrive")
	}
	if !strings.Contains(body, "Groceries (updated)") {
		t.Error("pushed list body missing")
	}
	if strings.Contains(body, "Private") {
		t.Error("leaked a list bob has no permission on")
	}
	if !strings.Contains(body, `"origin":"phone-2"`) {
		t.Error("origin missing; client cannot suppress its own echo")
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Error("frames must be terminated by a blank line")
	}
	if subCount(api.hub) != 0 {
		t.Error("subscriber leaked after disconnect")
	}
}

// PUT on an unknown id creates the list under the id the device chose, so the app
// never has to renumber a list it may already have open.
func TestPutUpsertsWithClientOwnedID(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{{User: User{ID: "alice"}, Role: "owner"}}}

	body := `{"HouseholdID":"H1","Title":"Shop","Items":[{"ID":"i1","Name":"Milk"}]}`
	req := httptest.NewRequest("PUT", "/v1/lists/local-uuid-1234", strings.NewReader(body))
	rec := httptest.NewRecorder()
	api.updateList(rec, req, User{ID: "alice"})

	if rec.Code != 201 {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body.String())
	}
	got, ok := api.s.Lists["local-uuid-1234"]
	if !ok {
		t.Fatal("list not stored under the client-supplied id")
	}
	if got.OwnerID != "alice" || got.Title != "Shop" || len(got.Items) != 1 {
		t.Fatalf("stored wrong: %+v", got)
	}

	// second PUT is a plain update, not a duplicate
	req2 := httptest.NewRequest("PUT", "/v1/lists/local-uuid-1234",
		strings.NewReader(`{"HouseholdID":"H1","Title":"Shop 2","Items":[{"ID":"i1","Name":"Milk"}]}`))
	rec2 := httptest.NewRecorder()
	api.updateList(rec2, req2, User{ID: "alice"})
	if rec2.Code != 200 {
		t.Fatalf("update: status %d, body %s", rec2.Code, rec2.Body.String())
	}
	if len(api.s.Lists) != 1 || api.s.Lists["local-uuid-1234"].Title != "Shop 2" {
		t.Fatalf("expected one updated list, got %+v", api.s.Lists)
	}

	// a non-member cannot create into someone else's household
	rec3 := httptest.NewRecorder()
	api.updateList(rec3, httptest.NewRequest("PUT", "/v1/lists/other-id-9999",
		strings.NewReader(`{"HouseholdID":"H1","Title":"Nope"}`)), User{ID: "mallory"})
	if rec3.Code != 403 {
		t.Fatalf("non-member create: status %d, want 403", rec3.Code)
	}
}

func TestOwnerControlsInvitationDelegation(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "owner"}, Role: "owner"},
		{User: User{ID: "bob"}, Role: "member"},
		{User: User{ID: "carol"}, Role: "member"},
	}}

	// Ordinary members cannot grant invitation access to themselves or anyone else.
	denied := httptest.NewRecorder()
	api.updateInvitePermission(denied,
		httptest.NewRequest("PUT", "/v1/members/bob/invite-permission", strings.NewReader(`{"Allow":true}`)),
		User{ID: "bob"})
	if denied.Code != 403 {
		t.Fatalf("member delegation status %d, want 403", denied.Code)
	}

	granted := httptest.NewRecorder()
	api.updateInvitePermission(granted,
		httptest.NewRequest("PUT", "/v1/members/bob/invite-permission", strings.NewReader(`{"Allow":true}`)),
		User{ID: "owner"})
	if granted.Code != 200 || !mayInvite(api.s.Households["H1"], "bob") {
		t.Fatalf("owner did not grant invitation access: status %d body %s", granted.Code, granted.Body.String())
	}

	invite := httptest.NewRecorder()
	api.createInvite(invite,
		httptest.NewRequest("POST", "/v1/invites", strings.NewReader(`{"ExpiresInHours":24}`)),
		User{ID: "bob"})
	if invite.Code != 201 {
		t.Fatalf("delegated inviter status %d, body %s", invite.Code, invite.Body.String())
	}

	stillDenied := httptest.NewRecorder()
	api.createInvite(stillDenied,
		httptest.NewRequest("POST", "/v1/invites", strings.NewReader(`{"ExpiresInHours":24}`)),
		User{ID: "carol"})
	if stillDenied.Code != 403 {
		t.Fatalf("non-delegated inviter status %d, want 403", stillDenied.Code)
	}
}
func TestHouseholdsUsePublicJSONShape(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Households["H1"] = Household{ID: "H1", Name: "Family", Members: []Member{
		{User: User{ID: "alice", FirstName: "Alice"}, Role: "owner"},
	}}

	rec := httptest.NewRecorder()
	api.households(rec, User{ID: "alice"})
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("households status %d, body %s", rec.Code, body)
	}
	for _, expected := range []string{`"id":"H1"`, `"name":"Family"`, `"userId":"alice"`, `"canInvite":true`} {
		if !strings.Contains(body, expected) {
			t.Errorf("public household response missing %s: %s", expected, body)
		}
	}
	if strings.Contains(body, `"ID"`) || strings.Contains(body, `"Members"`) {
		t.Fatalf("leaked internal Go field names: %s", body)
	}
}
func TestMemberCanClaimAdministrationWithMasterPairingKey(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub(), pairingKey: "correct-master-key"}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "owner"}, Role: "owner"},
		{User: User{ID: "member"}, Role: "member"},
	}}

	denied := httptest.NewRecorder()
	api.claimHouseholdAdmin(denied,
		httptest.NewRequest("PUT", "/v1/households/admin", strings.NewReader(`{"PairingKey":"wrong-master-key"}`)),
		User{ID: "member"})
	if denied.Code != 403 {
		t.Fatalf("wrong key status %d, want 403", denied.Code)
	}

	granted := httptest.NewRecorder()
	api.claimHouseholdAdmin(granted,
		httptest.NewRequest("PUT", "/v1/households/admin", strings.NewReader(`{"PairingKey":"correct-master-key"}`)),
		User{ID: "member"})
	if granted.Code != 200 || !isOwner(api.s.Households["H1"], "member") {
		t.Fatalf("member was not promoted: status %d body %s", granted.Code, granted.Body.String())
	}
}
func TestDevicePolicyBlocksSecondDeviceLink(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "owner"}, Role: "owner"},
		{User: User{ID: "member"}, Role: "member", AllowMultipleDevices: false},
	}}
	api.s.Devices["D1"] = Device{ID: "D1", UserID: "member"}

	blocked := httptest.NewRecorder()
	api.linkDevice(blocked,
		httptest.NewRequest("POST", "/v1/devices/link", strings.NewReader(`{"ExpiresInHours":1}`)),
		User{ID: "member"})
	if blocked.Code != 403 {
		t.Fatalf("second device status %d, want 403", blocked.Code)
	}

	h := api.s.Households["H1"]
	h.Members[1].AllowMultipleDevices = true
	api.s.Households["H1"] = h
	allowed := httptest.NewRecorder()
	api.linkDevice(allowed,
		httptest.NewRequest("POST", "/v1/devices/link", strings.NewReader(`{"ExpiresInHours":1}`)),
		User{ID: "member"})
	if allowed.Code != 201 {
		t.Fatalf("allowed device link status %d, body %s", allowed.Code, allowed.Body.String())
	}
}

func TestAdminRemovalRevokesMemberAndTransfersLists(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.KnownUsers["admin"] = User{ID: "admin"}
	api.s.KnownUsers["member"] = User{ID: "member"}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "admin"}, Role: "owner"},
		{User: User{ID: "member"}, Role: "member"},
	}}
	api.s.Devices["D1"] = Device{ID: "D1", UserID: "member"}
	api.s.Lists["L1"] = List{ID: "L1", HouseholdID: "H1", OwnerID: "member", Permissions: map[string]string{"admin": "edit"}}

	rec := httptest.NewRecorder()
	api.removeMember(rec, httptest.NewRequest("DELETE", "/v1/members/member", nil), User{ID: "admin"})
	if rec.Code != 200 {
		t.Fatalf("remove member status %d, body %s", rec.Code, rec.Body.String())
	}
	if member(api.s.Households["H1"], "member") {
		t.Fatal("removed user is still a household member")
	}
	if _, exists := api.s.Devices["D1"]; exists {
		t.Fatal("removed member device was not revoked")
	}
	if _, exists := api.s.KnownUsers["member"]; exists {
		t.Fatal("removed member can still authenticate as a known user")
	}
	if got := api.s.Lists["L1"].OwnerID; got != "admin" {
		t.Fatalf("owned list transferred to %q, want admin", got)
	}
}

func TestAdminManagesListMetadataAndPermanentDeletion(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.KnownUsers["admin"] = User{ID: "admin", FirstName: "Alice"}
	api.s.KnownUsers["member"] = User{ID: "member", FirstName: "Bob"}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: api.s.KnownUsers["admin"], Role: "owner"},
		{User: api.s.KnownUsers["member"], Role: "member"},
	}}
	listID := "stale-list-123"
	api.s.Lists[listID] = List{
		ID: listID, HouseholdID: "H1", OwnerID: "member", Title: "Old groceries", Icon: "store:de:sephora", StoreCountryCode: "DE", StoreCountryName: "Germany",
		Items: []Item{{ID: "secret-item", Name: "Private shopping item"}},
		Permissions: map[string]string{"admin": "edit"}, ItemHistory: map[string]int{"private shopping item": 4}, UpdatedAt: 1234,
	}

	metadata := httptest.NewRecorder()
	api.adminLists(metadata, User{ID: "admin"})
	body := metadata.Body.String()
	if metadata.Code != 200 || !strings.Contains(body, `"title":"Old groceries"`) || !strings.Contains(body, `"ownerName":"Bob"`) || !strings.Contains(body, `"countryName":"Germany"`) {
		t.Fatalf("admin metadata status %d, body %s", metadata.Code, body)
	}
	if strings.Contains(body, "Private shopping item") || strings.Contains(body, "secret-item") || strings.Contains(body, "itemHistory") {
		t.Fatalf("admin metadata leaked list contents: %s", body)
	}

	denied := httptest.NewRecorder()
	api.adminLists(denied, User{ID: "member"})
	if denied.Code != 403 { t.Fatalf("member metadata status %d, want 403", denied.Code) }

	deleted := httptest.NewRecorder()
	api.deleteAdminList(deleted, httptest.NewRequest("DELETE", "/v1/admin/lists/"+listID, nil), User{ID: "admin"})
	if deleted.Code != 200 { t.Fatalf("admin delete status %d, body %s", deleted.Code, deleted.Body.String()) }
	if _, exists := api.s.Lists[listID]; exists { t.Fatal("deleted list remains stored") }
	if _, tombstoned := api.s.DeletedLists[listID]; !tombstoned { t.Fatal("permanent deletion tombstone missing") }

	resurrect := httptest.NewRecorder()
	api.updateList(resurrect, httptest.NewRequest("PUT", "/v1/lists/"+listID,
		strings.NewReader(`{"HouseholdID":"H1","Title":"Old groceries"}`)), User{ID: "member"})
	if resurrect.Code != 410 { t.Fatalf("offline resurrection status %d, want 410; body %s", resurrect.Code, resurrect.Body.String()) }
}


// The enrolment handler only checks the shape of the key, so the tests below use a stand-in
// rather than generating a real one: what is under test is which identity a device lands on.
const testKey = `"PublicKey":{"Kty":"EC","Crv":"P-256","X":"x","Y":"y"}`

func enrolRaw(api *API, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	api.enrolDevice(rec, httptest.NewRequest("POST", "/v1/devices/enrol", strings.NewReader("{"+testKey+","+body+"}")))
	return rec
}

type enrolResult struct {
	DeviceID     string `json:"deviceId"`
	UserID       string `json:"userId"`
	RecoveryCode string `json:"recoveryCode"`
}

func decodeEnrol(t *testing.T, rec *httptest.ResponseRecorder) enrolResult {
	t.Helper()
	var out enrolResult
	if e := json.Unmarshal(rec.Body.Bytes(), &out); e != nil {
		t.Fatalf("enrolment response %s: %v", rec.Body.String(), e)
	}
	return out
}

// A reinstall destroys the keystore key, so without a way back to the same identity the phone
// enrols as a stranger and its own lists and backups become invisible to it.
func TestPairingKeyRelinkRestoresIdentityAndBackups(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub(), pairingKey: "correct-master-key"}
	api.s.KnownUsers["jim"] = User{ID: "jim", FirstName: "Jim"}
	api.s.Households["H1"] = Household{ID: "H1", Name: "Home", Members: []Member{
		{User: User{ID: "jim", FirstName: "Jim"}, Role: "owner", CanInvite: true, AllowMultipleDevices: true},
	}}
	api.s.Lists["L1"] = List{ID: "L1", HouseholdID: "H1", OwnerID: "jim", Title: "Groceries"}
	api.s.Backups["jim"] = []Backup{{ID: "B1", Name: "TidyShop.json", Payload: "{}"}}

	denied := enrolRaw(api, `"PairingKey":"wrong-master-key","ClaimUserID":"jim"`)
	if denied.Code != 403 {
		t.Fatalf("wrong pairing key status %d, want 403", denied.Code)
	}

	rec := enrolRaw(api, `"PairingKey":"correct-master-key","ClaimUserID":"jim","DeviceName":"New phone"`)
	if rec.Code != 201 {
		t.Fatalf("re-link status %d, body %s", rec.Code, rec.Body.String())
	}
	out := decodeEnrol(t, rec)
	if out.UserID != "jim" {
		t.Fatalf("re-linked as %q, want jim", out.UserID)
	}
	if got := len(api.s.Households["H1"].Members); got != 1 {
		t.Fatalf("household grew to %d members, want 1", got)
	}
	if permission(api.s.Lists["L1"], out.UserID) != "edit" {
		t.Fatal("the re-linked identity does not own its own list")
	}
	backups := httptest.NewRecorder()
	api.listBackups(backups, User{ID: out.UserID})
	if !strings.Contains(backups.Body.String(), `"id":"B1"`) {
		t.Fatalf("re-linked identity cannot see its backups: %s", backups.Body.String())
	}
	if api.s.Devices[out.DeviceID].UserID != "jim" {
		t.Fatal("the new device speaks for the wrong identity")
	}
	if out.RecoveryCode == "" {
		t.Fatal("an identity with no recovery code was not given one")
	}
}

// A household with a surviving owner device can re-admit somebody else's phone without the
// server's pairing key ever leaving the administrator's hands.
func TestOwnerMintsRelinkCodeForAnotherMember(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.KnownUsers["owner"] = User{ID: "owner"}
	api.s.KnownUsers["kid"] = User{ID: "kid"}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "owner"}, Role: "owner"},
		{User: User{ID: "kid"}, Role: "member", AllowMultipleDevices: false},
	}}
	api.s.Devices["D1"] = Device{ID: "D1", UserID: "kid"} // the install that was wiped

	denied := httptest.NewRecorder()
	api.linkDevice(denied, httptest.NewRequest("POST", "/v1/devices/link", strings.NewReader(`{"MemberID":"owner"}`)), User{ID: "kid"})
	if denied.Code != 403 {
		t.Fatalf("member minting for someone else status %d, want 403", denied.Code)
	}

	rec := httptest.NewRecorder()
	api.linkDevice(rec, httptest.NewRequest("POST", "/v1/devices/link", strings.NewReader(`{"MemberID":"kid"}`)), User{ID: "owner"})
	if rec.Code != 201 {
		t.Fatalf("owner link status %d, body %s", rec.Code, rec.Body.String())
	}
	var inv struct{ Code, Subject, Kind string }
	if e := json.Unmarshal(rec.Body.Bytes(), &inv); e != nil {
		t.Fatalf("invite response %s: %v", rec.Body.String(), e)
	}
	if inv.Subject != "kid" || inv.Kind != "device" {
		t.Fatalf("code names %q kind %q, want kid/device", inv.Subject, inv.Kind)
	}

	redeemed := enrolRaw(api, `"Code":"`+inv.Code+`"`)
	if redeemed.Code != 201 {
		t.Fatalf("redeem status %d, body %s", redeemed.Code, redeemed.Body.String())
	}
	if out := decodeEnrol(t, redeemed); out.UserID != "kid" {
		t.Fatalf("redeemed as %q, want kid", out.UserID)
	}
	if _, alive := api.s.Devices["D1"]; alive {
		t.Fatal("the one-device cap kept the dead device instead of replacing it")
	}
	if got := countDevices(api.s.Devices, "kid"); got != 1 {
		t.Fatalf("kid has %d devices, want 1", got)
	}
	if got := len(api.s.Households["H1"].Members); got != 2 {
		t.Fatalf("household grew to %d members, want 2", got)
	}
}

// The recovery code is the path for a member who does not administer the server and has no
// other device left. It is single-use, and its replacement comes back in the same response.
func TestRecoveryCodeReattachesAndRotates(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub(), pairingKey: "correct-master-key"}

	first := enrolRaw(api, `"PairingKey":"correct-master-key","DisplayName":"Jim"`)
	if first.Code != 201 {
		t.Fatalf("first enrolment status %d, body %s", first.Code, first.Body.String())
	}
	start := decodeEnrol(t, first)
	if start.RecoveryCode == "" {
		t.Fatal("a new identity was not given a recovery code")
	}
	api.s.Backups[start.UserID] = []Backup{{ID: "B1", Name: "TidyShop.json", Payload: "{}"}}

	// Typed back by a person: separators and case must not matter.
	typed := strings.ToLower(strings.ReplaceAll(start.RecoveryCode, "-", " "))
	back := enrolRaw(api, `"RecoveryCode":"`+typed+`"`)
	if back.Code != 201 {
		t.Fatalf("recovery status %d, body %s", back.Code, back.Body.String())
	}
	again := decodeEnrol(t, back)
	if again.UserID != start.UserID {
		t.Fatalf("recovery enrolled %q, want %q", again.UserID, start.UserID)
	}
	if again.RecoveryCode == "" || again.RecoveryCode == start.RecoveryCode {
		t.Fatalf("spent code was not rotated: %q", again.RecoveryCode)
	}

	spent := enrolRaw(api, `"RecoveryCode":"`+start.RecoveryCode+`"`)
	if spent.Code != 403 {
		t.Fatalf("reused code status %d, want 403", spent.Code)
	}

	// Rotating from a signed-in device invalidates the code that device was handed before.
	rot := httptest.NewRecorder()
	api.rotateRecovery(rot, httptest.NewRequest("POST", "/v1/devices/recovery", nil), User{ID: start.UserID})
	if rot.Code != 201 {
		t.Fatalf("rotate status %d, body %s", rot.Code, rot.Body.String())
	}
	stale := enrolRaw(api, `"RecoveryCode":"`+again.RecoveryCode+`"`)
	if stale.Code != 403 {
		t.Fatalf("rotated-away code status %d, want 403", stale.Code)
	}
}

func TestClaimableIdentitiesNeedThePairingKey(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub(), pairingKey: "correct-master-key"}
	api.s.Households["H1"] = Household{ID: "H1", Name: "Home", Members: []Member{
		{User: User{ID: "jim", FirstName: "Jim"}, Role: "owner"},
	}}

	denied := httptest.NewRecorder()
	api.claimableIdentities(denied, httptest.NewRequest("POST", "/v1/devices/claimable", strings.NewReader(`{"PairingKey":"nope"}`)))
	if denied.Code != 403 {
		t.Fatalf("wrong key status %d, want 403", denied.Code)
	}

	listed := httptest.NewRecorder()
	api.claimableIdentities(listed, httptest.NewRequest("POST", "/v1/devices/claimable", strings.NewReader(`{"PairingKey":"correct-master-key"}`)))
	if listed.Code != 200 || !strings.Contains(listed.Body.String(), `"userId":"jim"`) {
		t.Fatalf("claimable status %d, body %s", listed.Code, listed.Body.String())
	}
}

func TestNudgeReachesEveryoneButTheSender(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "alice"}, Role: "owner"},
		{User: User{ID: "bob"}},
	}}
	api.s.Lists["L1"] = List{ID: "L1", HouseholdID: "H1", OwnerID: "alice", Title: "PLUS",
		Permissions: map[string]string{"bob": "check"},
		Items:       []Item{{ID: "i1", Name: "Milk"}, {ID: "i2", Name: "Bread", Checked: true}}}

	alice := api.hub.add("alice")
	bob := api.hub.add("bob")

	// check-only is deliberate: asking for the shopping is not an owner-only act
	rec := httptest.NewRecorder()
	api.nudgeList(rec, httptest.NewRequest("POST", "/v1/lists/L1/nudge", strings.NewReader("{}")), User{ID: "bob"})
	if rec.Code != 200 {
		t.Fatalf("nudge: status %d, body %s", rec.Code, rec.Body.String())
	}

	stored := api.s.Lists["L1"]
	if stored.NudgedAt == 0 || stored.NudgedBy != "bob" {
		t.Fatalf("nudge not recorded on the list: %+v", stored)
	}

	// the response has to carry the nudge, so the sender's own copy stays in step
	var body List
	if e := json.Unmarshal(rec.Body.Bytes(), &body); e != nil {
		t.Fatalf("response is not a list: %v", e)
	}
	if body.NudgedAt != stored.NudgedAt || body.NudgedBy != "bob" {
		t.Fatalf("response did not carry the nudge: %+v", body)
	}

	select {
	case f := <-alice.ch:
		if !strings.Contains(string(f), "list.nudged") {
			t.Fatalf("the owner got the wrong frame: %s", f)
		}
	default:
		t.Fatal("the owner was not notified")
	}
	select {
	case f := <-bob.ch:
		t.Fatalf("the sender was notified of their own nudge: %s", f)
	default:
	}
}

func TestNudgeIsRateLimitedAndSurvivesAListUpdate(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{{User: User{ID: "alice"}, Role: "owner"}}}
	api.s.Lists["L1"] = List{ID: "L1", HouseholdID: "H1", OwnerID: "alice", Title: "PLUS"}

	first := httptest.NewRecorder()
	api.nudgeList(first, httptest.NewRequest("POST", "/v1/lists/L1/nudge", strings.NewReader("{}")), User{ID: "alice"})
	at := api.s.Lists["L1"].NudgedAt
	if at == 0 {
		t.Fatal("the first nudge was not recorded")
	}

	// a second tap inside the cooldown is answered, but must not re-notify
	second := httptest.NewRecorder()
	api.nudgeList(second, httptest.NewRequest("POST", "/v1/lists/L1/nudge", strings.NewReader("{}")), User{ID: "alice"})
	if second.Code != 200 {
		t.Fatalf("second nudge: status %d, body %s", second.Code, second.Body.String())
	}
	if api.s.Lists["L1"].NudgedAt != at {
		t.Fatal("the cooldown did not hold the nudge timestamp")
	}

	// an ordinary edit must preserve the nudge, the way it already preserves permissions
	rec := httptest.NewRecorder()
	api.updateList(rec, httptest.NewRequest("PUT", "/v1/lists/L1",
		strings.NewReader(`{"HouseholdID":"H1","Title":"PLUS","Items":[{"ID":"i1","Name":"Milk"}]}`)), User{ID: "alice"})
	if rec.Code != 200 {
		t.Fatalf("update: status %d, body %s", rec.Code, rec.Body.String())
	}
	if after := api.s.Lists["L1"]; after.NudgedAt != at || after.NudgedBy != "alice" {
		t.Fatalf("a list update wiped the nudge: %+v", after)
	}
}

func TestNudgeRefusesSomeoneWithoutAccess(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Lists["L1"] = List{ID: "L1", HouseholdID: "H1", OwnerID: "alice", Title: "PLUS"}

	rec := httptest.NewRecorder()
	api.nudgeList(rec, httptest.NewRequest("POST", "/v1/lists/L1/nudge", strings.NewReader("{}")), User{ID: "mallory"})
	if rec.Code != 404 {
		t.Fatalf("stranger nudge: status %d, want 404", rec.Code)
	}
	if api.s.Lists["L1"].NudgedAt != 0 {
		t.Fatal("a stranger moved the nudge timestamp")
	}
}

func TestUpdatePermissionsSetsAndValidatesEveryone(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.KnownUsers["bob"] = User{ID: "bob"}
	api.s.Lists["L1"] = List{ID: "L1", HouseholdID: "H1", OwnerID: "alice", Title: "PLUS"}

	bad := httptest.NewRecorder()
	api.updatePermissions(bad, httptest.NewRequest("PUT", "/v1/lists/L1/permissions",
		strings.NewReader(`{"Permissions":{},"EveryonePermission":"nonsense"}`)), User{ID: "alice"})
	if bad.Code != 400 {
		t.Fatalf("invalid everyone level: status %d, want 400", bad.Code)
	}
	if api.s.Lists["L1"].EveryonePermission != "" {
		t.Fatal("a rejected update still wrote the everyone level")
	}

	ok := httptest.NewRecorder()
	api.updatePermissions(ok, httptest.NewRequest("PUT", "/v1/lists/L1/permissions",
		strings.NewReader(`{"Permissions":{"bob":"check"},"EveryonePermission":"view"}`)), User{ID: "alice"})
	if ok.Code != 200 {
		t.Fatalf("valid update: status %d, body %s", ok.Code, ok.Body.String())
	}
	if got := api.s.Lists["L1"].EveryonePermission; got != "view" {
		t.Fatalf("everyone level = %q, want view", got)
	}

	// An ordinary content update must not silently turn sharing off again.
	rec := httptest.NewRecorder()
	api.updateList(rec, httptest.NewRequest("PUT", "/v1/lists/L1",
		strings.NewReader(`{"HouseholdID":"H1","Title":"PLUS 2"}`)), User{ID: "alice"})
	if rec.Code != 200 {
		t.Fatalf("content update: status %d, body %s", rec.Code, rec.Body.String())
	}
	if got := api.s.Lists["L1"].EveryonePermission; got != "view" {
		t.Fatalf("a content-only update wiped everyone sharing, level = %q", got)
	}
}

func TestSubscribableListsExcludesWhatIsAlreadyReachable(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "alice"}, Role: "owner"},
		{User: User{ID: "bob"}},
		{User: User{ID: "carol"}},
	}}
	api.s.Lists["shared"] = List{ID: "shared", HouseholdID: "H1", OwnerID: "alice", Title: "Everyone list", EveryonePermission: "view"}
	api.s.Lists["private"] = List{ID: "private", HouseholdID: "H1", OwnerID: "alice", Title: "Not shared"}
	api.s.Lists["already-bobs"] = List{ID: "already-bobs", HouseholdID: "H1", OwnerID: "alice", Title: "Already Bob's",
		EveryonePermission: "view", Permissions: map[string]string{"bob": "edit"}}

	bob := httptest.NewRecorder()
	api.subscribableLists(bob, User{ID: "bob"})
	if bob.Code != 200 || !strings.Contains(bob.Body.String(), "shared") || strings.Contains(bob.Body.String(), "already-bobs") || strings.Contains(bob.Body.String(), "private") {
		t.Fatalf("bob's subscribable list: status %d, body %s", bob.Code, bob.Body.String())
	}

	// carol is a plain member: subscribing offers the everyone level, not edit.
	carol := httptest.NewRecorder()
	api.subscribableLists(carol, User{ID: "carol"})
	if !strings.Contains(carol.Body.String(), `"permission":"view"`) {
		t.Fatalf("carol's offered level: %s, want view", carol.Body.String())
	}

	// alice is the household admin as well as the list owner: an admin sees edit offered.
	admin := httptest.NewRecorder()
	api.s.Lists["other-owner"] = List{ID: "other-owner", HouseholdID: "H1", OwnerID: "bob", Title: "Bob's shared list", EveryonePermission: "check"}
	api.subscribableLists(admin, User{ID: "alice"})
	if !strings.Contains(admin.Body.String(), `"id":"other-owner"`) || !strings.Contains(admin.Body.String(), `"permission":"edit"`) {
		t.Fatalf("admin's offered level: %s, want edit", admin.Body.String())
	}
}

func TestSubscribeGrantsEveryoneLevelAndAdminAlwaysGetsEdit(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "alice"}, Role: "owner"},
		{User: User{ID: "bob"}},
	}}
	api.s.Lists["L1"] = List{ID: "L1", HouseholdID: "H1", OwnerID: "alice", Title: "PLUS", EveryonePermission: "view"}

	// bob, a plain member, subscribes and lands on the everyone level.
	bobRec := httptest.NewRecorder()
	api.subscribeToList(bobRec, httptest.NewRequest("POST", "/v1/lists/L1/subscribe", strings.NewReader("{}")), User{ID: "bob"})
	if bobRec.Code != 200 {
		t.Fatalf("bob subscribe: status %d, body %s", bobRec.Code, bobRec.Body.String())
	}
	if got := api.s.Lists["L1"].Permissions["bob"]; got != "view" {
		t.Fatalf("bob's granted level = %q, want view", got)
	}

	// an admin who subscribes always lands on edit, even though everyone else gets view.
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "alice"}, Role: "owner"},
		{User: User{ID: "bob"}},
		{User: User{ID: "admin2"}, Role: "owner"},
	}}
	adminRec := httptest.NewRecorder()
	api.subscribeToList(adminRec, httptest.NewRequest("POST", "/v1/lists/L1/subscribe", strings.NewReader("{}")), User{ID: "admin2"})
	if adminRec.Code != 200 {
		t.Fatalf("admin subscribe: status %d, body %s", adminRec.Code, adminRec.Body.String())
	}
	if got := api.s.Lists["L1"].Permissions["admin2"]; got != "edit" {
		t.Fatalf("admin's granted level = %q, want edit", got)
	}

	// a stranger to the household cannot subscribe.
	strangerRec := httptest.NewRecorder()
	api.subscribeToList(strangerRec, httptest.NewRequest("POST", "/v1/lists/L1/subscribe", strings.NewReader("{}")), User{ID: "mallory"})
	if strangerRec.Code != 403 {
		t.Fatalf("stranger subscribe: status %d, want 403", strangerRec.Code)
	}

	// a list that is not shared with everyone cannot be subscribed to.
	api.s.Lists["L2"] = List{ID: "L2", HouseholdID: "H1", OwnerID: "alice", Title: "Private"}
	notShared := httptest.NewRecorder()
	api.subscribeToList(notShared, httptest.NewRequest("POST", "/v1/lists/L2/subscribe", strings.NewReader("{}")), User{ID: "bob"})
	if notShared.Code != 403 {
		t.Fatalf("subscribe to non-everyone list: status %d, want 403", notShared.Code)
	}
}

// An admin must never be locked out of a list simply because nobody chose to share it: they
// can take over any list in their own household, landing on edit, while an ordinary member
// still cannot touch it.
func TestAdminSubscribesToAnyListRegardlessOfEveryoneSetting(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "alice"}, Role: "owner"},
		{User: User{ID: "bob"}},
		{User: User{ID: "carol"}},
	}}
	api.s.Lists["private"] = List{ID: "private", HouseholdID: "H1", OwnerID: "bob", Title: "Bob's private list"}

	// the admin's own private-list-management view offers it, at edit.
	adminView := httptest.NewRecorder()
	api.subscribableLists(adminView, User{ID: "alice"})
	if !strings.Contains(adminView.Body.String(), `"id":"private"`) || !strings.Contains(adminView.Body.String(), `"permission":"edit"`) {
		t.Fatalf("admin's subscribable list did not offer the private list at edit: %s", adminView.Body.String())
	}

	// a plain member's view does not offer it -- they have no more reach than before.
	memberView := httptest.NewRecorder()
	api.subscribableLists(memberView, User{ID: "carol"})
	if strings.Contains(memberView.Body.String(), `"id":"private"`) {
		t.Fatalf("a plain member's subscribable list leaked a private list: %s", memberView.Body.String())
	}
	memberSubscribe := httptest.NewRecorder()
	api.subscribeToList(memberSubscribe, httptest.NewRequest("POST", "/v1/lists/private/subscribe", strings.NewReader("{}")), User{ID: "carol"})
	if memberSubscribe.Code != 403 {
		t.Fatalf("plain member subscribe to a private list: status %d, want 403", memberSubscribe.Code)
	}

	// the admin can actually subscribe to it too, not just see it offered.
	adminSubscribe := httptest.NewRecorder()
	api.subscribeToList(adminSubscribe, httptest.NewRequest("POST", "/v1/lists/private/subscribe", strings.NewReader("{}")), User{ID: "alice"})
	if adminSubscribe.Code != 200 {
		t.Fatalf("admin subscribe to a private list: status %d, body %s", adminSubscribe.Code, adminSubscribe.Body.String())
	}
	if got := api.s.Lists["private"].Permissions["alice"]; got != "edit" {
		t.Fatalf("admin's granted level on a private list = %q, want edit", got)
	}
	// the original owner keeps ownership; the admin gained access, not a takeover of the record.
	if got := api.s.Lists["private"].OwnerID; got != "bob" {
		t.Fatalf("subscribing changed list ownership to %q, want bob unchanged", got)
	}
}
