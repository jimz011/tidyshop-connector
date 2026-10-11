package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func productsAPI(t *testing.T) *API {
	t.Helper()
	api := &API{s: newStore(t.TempDir() + "/state.json"), hub: newHub()}
	api.s.Households["H1"] = Household{ID: "H1", Members: []Member{
		{User: User{ID: "alice"}, Role: "owner"},
		{User: User{ID: "bob"}, Role: "member"},
	}}
	return api
}

func put(api *API, user, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	api.updateList(rec, httptest.NewRequest("PUT", path, strings.NewReader(body)), User{ID: user})
	return rec
}

// Saving a list is how a product gets into the catalogue, and a later save without details
// must not erase what the household already knows.
func TestSavingItemsBuildsTheCatalogue(t *testing.T) {
	api := productsAPI(t)
	photo := strings.Repeat("a", 64)
	body := `{"HouseholdID":"H1","Title":"Shop","ItemDetails":true,"Items":[` +
		`{"ID":"i1","Name":"Bread","Department":"Bakery","Photo":"` + photo + `","CreatedAt":5,"UpdatedAt":9},` +
		`{"ID":"i2","Name":"Milk","UpdatedAt":9}]}`
	if rec := put(api, "alice", "/v1/lists/list-0001", body); rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	got := api.s.Products["H1"]["bread"]
	if got.Name != "Bread" || got.Department != "Bakery" || got.Photo != photo {
		t.Fatalf("bread not remembered: %+v", got)
	}
	if _, ok := api.s.Products["H1"]["milk"]; ok {
		t.Fatal("an item with no details should not enter the catalogue")
	}

	// Same product again with no details leaves the catalogue alone.
	again := `{"HouseholdID":"H1","Title":"Shop","ItemDetails":true,"Items":[{"ID":"i3","Name":"bread","UpdatedAt":10}]}`
	if rec := put(api, "alice", "/v1/lists/list-0001", again); rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	if api.s.Products["H1"]["bread"].Department != "Bakery" {
		t.Fatal("an empty department erased the catalogue entry")
	}
}

// Phones running an older app never send details; their saves must not strip them.
func TestOlderAppsDoNotWipeItemDetails(t *testing.T) {
	api := productsAPI(t)
	photo := strings.Repeat("b", 64)
	create := `{"HouseholdID":"H1","Title":"Shop","ItemDetails":true,"Items":[` +
		`{"ID":"i1","Name":"Bread","Department":"Bakery","Photo":"` + photo + `","CreatedAt":5,"UpdatedAt":9}]}`
	put(api, "alice", "/v1/lists/list-0002", create)

	old := `{"Title":"Shop","Items":[{"ID":"i1","Name":"Bread","Checked":true,"UpdatedAt":12}]}`
	if rec := put(api, "alice", "/v1/lists/list-0002", old); rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	item := api.s.Lists["list-0002"].Items[0]
	if item.Department != "Bakery" || item.Photo != photo || item.CreatedAt != 5 || !item.Checked {
		t.Fatalf("details lost or check ignored: %+v", item)
	}

	// A newer app clears a department by sending the marker and an empty value.
	clear := `{"Title":"Shop","ItemDetails":true,"Items":[{"ID":"i1","Name":"Bread","Photo":"` + photo + `","CreatedAt":5,"UpdatedAt":13}]}`
	put(api, "alice", "/v1/lists/list-0002", clear)
	if api.s.Lists["list-0002"].Items[0].Department != "" {
		t.Fatal("a newer app could not clear a department")
	}
}

func TestBadPhotoIdsAreDropped(t *testing.T) {
	api := productsAPI(t)
	body := `{"HouseholdID":"H1","Title":"Shop","ItemDetails":true,"Items":[{"ID":"i1","Name":"Bread","Photo":"../../etc/passwd","UpdatedAt":1}]}`
	put(api, "alice", "/v1/lists/list-0003", body)
	if p := api.s.Lists["list-0003"].Items[0].Photo; p != "" {
		t.Fatalf("path-like photo id survived: %q", p)
	}
}

func TestProductsCanBeListedReplacedAndRemoved(t *testing.T) {
	api := productsAPI(t)
	api.s.Products["H1"] = map[string]Product{"bread": {Name: "Bread", Department: "Bakery"}}

	rec := httptest.NewRecorder()
	api.listProducts(rec, User{ID: "bob"})
	var listed []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed) != 1 || listed[0]["name"] != "Bread" {
		t.Fatalf("list: %v %s", err, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	api.putProduct(rec, httptest.NewRequest("PUT", "/v1/products/Bread", strings.NewReader(`{"Name":"Bread","Department":"Fresh"}`)), User{ID: "bob"})
	if rec.Code != 200 || api.s.Products["H1"]["bread"].Department != "Fresh" {
		t.Fatalf("replace: %d %+v", rec.Code, api.s.Products["H1"])
	}

	rec = httptest.NewRecorder()
	api.putProduct(rec, httptest.NewRequest("PUT", "/v1/products/bread", strings.NewReader(`{"Name":"Bread"}`)), User{ID: "bob"})
	if _, ok := api.s.Products["H1"]["bread"]; ok {
		t.Fatal("a product with no details should be removed")
	}

	rec = httptest.NewRecorder()
	api.listProducts(rec, User{ID: "mallory"})
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("a stranger saw products: %s", rec.Body.String())
	}
}

func TestPhotosRoundTripWithinTheHousehold(t *testing.T) {
	api := productsAPI(t)
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{7}, 200)...)

	rec := httptest.NewRecorder()
	api.putPhoto(rec, httptest.NewRequest("PUT", "/v1/photos", bytes.NewReader(jpeg)), User{ID: "alice"})
	if rec.Code != 201 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var out struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	sum := sha256.Sum256(jpeg)
	if out.ID != hex.EncodeToString(sum[:]) {
		t.Fatalf("id %q is not the SHA-256 of the bytes", out.ID)
	}

	rec = httptest.NewRecorder()
	api.getPhoto(rec, httptest.NewRequest("GET", "/v1/photos/"+out.ID, nil), User{ID: "bob"})
	if rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), jpeg) || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("download: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	api.getPhoto(rec, httptest.NewRequest("GET", "/v1/photos/"+out.ID, nil), User{ID: "mallory"})
	if rec.Code != 404 {
		t.Fatalf("a stranger read a photo: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	api.putPhoto(rec, httptest.NewRequest("PUT", "/v1/photos", strings.NewReader("<html>not an image</html>")), User{ID: "alice"})
	if rec.Code != 415 {
		t.Fatalf("non-JPEG accepted: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	api.putPhoto(rec, httptest.NewRequest("PUT", "/v1/photos", bytes.NewReader(append([]byte{0xFF, 0xD8, 0xFF}, make([]byte, maxPhotoBytes)...))), User{ID: "alice"})
	if rec.Code != 413 {
		t.Fatalf("oversized photo accepted: %d", rec.Code)
	}
}
