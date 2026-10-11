package main

// The household product catalogue and its photos.
//
// An item can carry a Department (the aisle it lives in) and a Photo. Whenever a list is saved,
// every item that has either is remembered here under its lower-cased name, so the next person
// to type "brea" gets "Bread" suggested with its aisle and picture already attached.
//
// Photos are small JPEGs, stored as files next to the state file and named by the SHA-256 of
// their bytes. That makes an upload idempotent, and means two phones that picked the same
// picture end up with the same id.

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Product is what the household knows about something it buys. Name keeps the casing the
// user typed; the catalogue itself is keyed by productKey(Name).
type Product struct {
	Name       string
	Department string `json:",omitempty"`
	Photo      string `json:",omitempty"`
	UpdatedAt  int64
}

const (
	maxPhotoBytes     = 1 << 20
	maxDepartmentRune = 60
	maxProductName    = 120
)

var photoID = regexp.MustCompile(`^[0-9a-f]{64}$`)

func productKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func cleanDepartment(v string) string {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) > maxDepartmentRune {
		v = string([]rune(v)[:maxDepartmentRune])
	}
	return v
}

// cleanPhoto drops anything that is not a photo id, which also keeps path separators out of
// the file names built from it.
func cleanPhoto(v string) string {
	if photoID.MatchString(v) {
		return v
	}
	return ""
}

// prepareItems tidies the items of an incoming list. Apps that predate item details do not send
// Department, Photo or CreatedAt, and would otherwise wipe them on every save, so for those the
// stored values are carried over. Newer apps set ItemDetails and are taken at their word, which
// is how a department or photo gets cleared.
func prepareItems(old *List, in List) []Item {
	before := map[string]Item{}
	if old != nil {
		for _, it := range old.Items {
			before[it.ID] = it
		}
	}
	out := make([]Item, 0, len(in.Items))
	for _, it := range in.Items {
		prev, known := before[it.ID]
		if !in.ItemDetails && known {
			it.Department, it.Photo = prev.Department, prev.Photo
		}
		if it.CreatedAt == 0 && known {
			it.CreatedAt = prev.CreatedAt
		}
		if it.CreatedAt == 0 {
			it.CreatedAt = it.UpdatedAt
		}
		it.Department = cleanDepartment(it.Department)
		it.Photo = cleanPhoto(it.Photo)
		out = append(out, it)
	}
	return out
}

// rememberProducts folds the details of saved items into the household catalogue. A field an
// item leaves empty never erases what the catalogue already knows. Caller holds the write lock.
func (api *API) rememberProducts(householdID string, items []Item, now int64) {
	if householdID == "" {
		return
	}
	for _, it := range items {
		key := productKey(it.Name)
		if key == "" || len(key) > maxProductName || (it.Department == "" && it.Photo == "") {
			continue
		}
		catalogue := api.s.Products[householdID]
		if catalogue == nil {
			catalogue = map[string]Product{}
			api.s.Products[householdID] = catalogue
		}
		p := catalogue[key]
		if p.Name == "" {
			p.Name = strings.TrimSpace(it.Name)
		}
		if it.Department != "" {
			p.Department = it.Department
		}
		if it.Photo != "" {
			p.Photo = it.Photo
		}
		p.UpdatedAt = now
		catalogue[key] = p
	}
}

func productView(key string, p Product) map[string]any {
	return map[string]any{"key": key, "name": p.Name, "department": p.Department, "photo": p.Photo, "updatedAt": p.UpdatedAt}
}

// GET /v1/products: everything the caller's household has saved, by name.
func (api *API) listProducts(w http.ResponseWriter, u User) {
	api.s.RLock()
	defer api.s.RUnlock()
	h, ok := api.householdFor(u.ID)
	out := []map[string]any{}
	if ok {
		keys := make([]string, 0, len(api.s.Products[h.ID]))
		for key := range api.s.Products[h.ID] {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			out = append(out, productView(key, api.s.Products[h.ID][key]))
		}
	}
	write(w, 200, out)
}

// PUT /v1/products/{name}: replaces what is known about one product. Both fields empty removes it.
func (api *API) putProduct(w http.ResponseWriter, r *http.Request, u User) {
	key := productKey(strings.TrimPrefix(r.URL.Path, "/v1/products/"))
	var in struct{ Name, Department, Photo string }
	if decode(w, r, &in) != nil {
		return
	}
	if key == "" || len(key) > maxProductName {
		write(w, 400, map[string]string{"error": "a product name is required"})
		return
	}
	api.s.Lock()
	defer api.s.Unlock()
	h, ok := api.householdFor(u.ID)
	if !ok {
		write(w, 403, map[string]string{"error": "not a household member"})
		return
	}
	p := Product{Name: strings.TrimSpace(in.Name), Department: cleanDepartment(in.Department), Photo: cleanPhoto(in.Photo), UpdatedAt: time.Now().UnixMilli()}
	if p.Name == "" {
		p.Name = key
	}
	if p.Department == "" && p.Photo == "" {
		delete(api.s.Products[h.ID], key)
		_ = api.s.save()
		write(w, 200, map[string]any{"key": key, "removed": true})
		return
	}
	if api.s.Products[h.ID] == nil {
		api.s.Products[h.ID] = map[string]Product{}
	}
	api.s.Products[h.ID][key] = p
	_ = api.s.save()
	write(w, 200, productView(key, p))
}

// DELETE /v1/products/{name}
func (api *API) deleteProduct(w http.ResponseWriter, r *http.Request, u User) {
	key := productKey(strings.TrimPrefix(r.URL.Path, "/v1/products/"))
	api.s.Lock()
	defer api.s.Unlock()
	h, ok := api.householdFor(u.ID)
	if !ok {
		write(w, 403, map[string]string{"error": "not a household member"})
		return
	}
	delete(api.s.Products[h.ID], key)
	_ = api.s.save()
	write(w, 200, map[string]any{"key": key, "removed": true})
}

func (s *Store) photoPath(householdID, id string) string {
	return filepath.Join(filepath.Dir(s.path), "photos", householdID, id+".jpg")
}

// PUT /v1/photos: the body is the JPEG itself. Returns the id to put on an item or product.
func (api *API) putPhoto(w http.ResponseWriter, r *http.Request, u User) {
	api.s.RLock()
	h, ok := api.householdFor(u.ID)
	api.s.RUnlock()
	if !ok {
		write(w, 403, map[string]string{"error": "not a household member"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPhotoBytes))
	if err != nil {
		write(w, 413, map[string]string{"error": "photo is larger than 1 MiB"})
		return
	}
	if len(body) < 4 || body[0] != 0xFF || body[1] != 0xD8 || body[2] != 0xFF {
		write(w, 415, map[string]string{"error": "photo must be a JPEG"})
		return
	}
	sum := sha256.Sum256(body)
	id := hex.EncodeToString(sum[:])
	path := api.s.photoPath(h.ID, id)
	if _, err := os.Stat(path); err != nil {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			write(w, 500, map[string]string{"error": "save failed"})
			return
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, body, 0600); err != nil {
			write(w, 500, map[string]string{"error": "save failed"})
			return
		}
		if err := os.Rename(tmp, path); err != nil {
			write(w, 500, map[string]string{"error": "save failed"})
			return
		}
	}
	write(w, 201, map[string]string{"id": id})
}

// GET /v1/photos/{id}: only members of the household that uploaded it can read it.
func (api *API) getPhoto(w http.ResponseWriter, r *http.Request, u User) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/photos/")
	api.s.RLock()
	h, ok := api.householdFor(u.ID)
	api.s.RUnlock()
	if !ok || cleanPhoto(id) == "" {
		write(w, 404, map[string]string{"error": "photo not found"})
		return
	}
	data, err := os.ReadFile(api.s.photoPath(h.ID, id))
	if err != nil {
		write(w, 404, map[string]string{"error": "photo not found"})
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.WriteHeader(200)
	_, _ = w.Write(data)
}
