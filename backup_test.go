package main

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func postBackup(t *testing.T, api *API, user, name string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"Name": name, "Payload": `{"app":"TidyShop"}`})
	rec := httptest.NewRecorder()
	api.createBackup(rec, httptest.NewRequest("POST", "/v1/backups", strings.NewReader(string(body))), User{ID: user})
	if rec.Code != 201 {
		t.Fatalf("createBackup: %d %s", rec.Code, rec.Body.String())
	}
	var out struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out.ID
}

func TestBackupsAreCappedAndNewestFirst(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	for i := 0; i < maxBackupsPerUser+4; i++ {
		postBackup(t, api, "alice", fmt.Sprintf("b%d.json", i))
	}
	kept := api.s.Backups["alice"]
	if len(kept) != maxBackupsPerUser {
		t.Fatalf("kept %d backups, want %d", len(kept), maxBackupsPerUser)
	}
	// Newest is first, so the oldest are the ones dropped.
	if kept[0].Name != fmt.Sprintf("b%d.json", maxBackupsPerUser+3) {
		t.Fatalf("newest backup is not first: %s", kept[0].Name)
	}
}

func TestBackupsAreNotVisibleToOtherUsers(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	id := postBackup(t, api, "alice", "alice.json")
	postBackup(t, api, "bob", "bob.json")

	rec := httptest.NewRecorder()
	api.listBackups(rec, User{ID: "bob"})
	if strings.Contains(rec.Body.String(), "alice.json") {
		t.Fatal("bob can see alice's backups")
	}

	// Bob must not be able to fetch Alice's backup by guessing its id.
	rec2 := httptest.NewRecorder()
	api.fetchBackup(rec2, httptest.NewRequest("GET", "/v1/backups/"+id, nil), User{ID: "bob"})
	if rec2.Code != 404 {
		t.Fatalf("cross-user fetch returned %d, want 404", rec2.Code)
	}

	rec3 := httptest.NewRecorder()
	api.fetchBackup(rec3, httptest.NewRequest("GET", "/v1/backups/"+id, nil), User{ID: "alice"})
	if rec3.Code != 200 || !strings.Contains(rec3.Body.String(), "TidyShop") {
		t.Fatalf("owner fetch failed: %d %s", rec3.Code, rec3.Body.String())
	}
}

func TestOversizedBackupRejected(t *testing.T) {
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	huge, _ := json.Marshal(map[string]string{"Name": "big.json", "Payload": strings.Repeat("x", maxBackupBytes+1)})
	rec := httptest.NewRecorder()
	api.createBackup(rec, httptest.NewRequest("POST", "/v1/backups", strings.NewReader(string(huge))), User{ID: "alice"})
	if rec.Code != 413 {
		t.Fatalf("oversized backup returned %d, want 413", rec.Code)
	}
}
