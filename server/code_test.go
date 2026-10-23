//go:build !wasm

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	otpm "github.com/dim13/otpauth/migration"
	"github.com/gin-gonic/gin"
	"github.com/mlctrez/twofactor/store"
)

// dummy secrets only; the first is the RFC 6238 SHA1 test seed
const rfcSeed = "12345678901234567890"

func writeTestStorage(t *testing.T) string {
	t.Helper()
	s := &store.Storage{OtpParams: []*otpm.Payload_OtpParameters{
		{Secret: []byte(rfcSeed), Name: "alice", Issuer: "Example", Type: otpm.Payload_OTP_TYPE_TOTP},
		{Secret: []byte(rfcSeed), Name: "eight", Digits: otpm.Payload_DIGIT_COUNT_EIGHT, Type: otpm.Payload_OTP_TYPE_TOTP},
		{Secret: []byte("dummy-hotp-secret"), Name: "counter", Type: otpm.Payload_OTP_TYPE_HOTP},
		{Secret: []byte("dummy-secret-0001"), Name: "dup"},
		{Secret: []byte("dummy-secret-0002"), Name: "dup"},
	}}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "storage.json")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func testEngine(storagePath string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	registerCodeRoutes(engine, storagePath, func() time.Time { return time.Unix(59, 0) })
	return engine
}

func get(t *testing.T, engine *gin.Engine, target string) (int, map[string]interface{}, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	body := rec.Body.String()
	var decoded map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("%s: response is not json: %q", target, body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("%s: missing Cache-Control: no-store", target)
	}
	return rec.Code, decoded, body
}

func codeURL(label string) string {
	return "/api/code?label=" + url.QueryEscape(label)
}

func assertNoSecrets(t *testing.T, body string) {
	t.Helper()
	for _, secret := range []string{rfcSeed, "dummy-", "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ", "secret"} {
		if strings.Contains(body, secret) {
			t.Errorf("response leaks secret material %q: %s", secret, body)
		}
	}
}

func TestCodeHandler(t *testing.T) {
	engine := testEngine(writeTestStorage(t))

	tests := []struct {
		target     string
		wantStatus int
		wantCode   string
	}{
		{codeURL("Example alice"), http.StatusOK, "287082"},
		{codeURL("example ALICE"), http.StatusOK, "287082"},
		{codeURL("eight"), http.StatusOK, "94287082"},
		{codeURL("missing"), http.StatusNotFound, ""},
		{codeURL("dup"), http.StatusConflict, ""},
		{codeURL("counter"), http.StatusBadRequest, ""},
		{"/api/code", http.StatusBadRequest, ""},
	}
	for _, tt := range tests {
		status, decoded, body := get(t, engine, tt.target)
		assertNoSecrets(t, body)
		if status != tt.wantStatus {
			t.Errorf("%s: status %d want %d (%s)", tt.target, status, tt.wantStatus, body)
			continue
		}
		if tt.wantStatus != http.StatusOK {
			if _, ok := decoded["error"]; !ok {
				t.Errorf("%s: expected error field: %s", tt.target, body)
			}
			continue
		}
		if decoded["code"] != tt.wantCode {
			t.Errorf("%s: code %v want %s", tt.target, decoded["code"], tt.wantCode)
		}
		if decoded["remaining"] != float64(1) || decoded["period"] != float64(30) {
			t.Errorf("%s: unexpected remaining/period: %s", tt.target, body)
		}
		if len(decoded) != 4 {
			t.Errorf("%s: unexpected fields: %s", tt.target, body)
		}
	}

	_, decoded, _ := get(t, engine, codeURL("dup"))
	if matches, ok := decoded["matches"].([]interface{}); !ok || len(matches) != 2 {
		t.Errorf("ambiguous response should list matches: %v", decoded)
	}
}

func TestLabelsHandler(t *testing.T) {
	engine := testEngine(writeTestStorage(t))
	status, decoded, body := get(t, engine, "/api/labels")
	assertNoSecrets(t, body)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	labels, ok := decoded["labels"].([]interface{})
	if !ok || len(labels) != 5 || labels[0] != "Example alice" {
		t.Fatalf("unexpected labels: %s", body)
	}
}

func TestStorageMissingOrInvalid(t *testing.T) {
	dir := t.TempDir()
	invalid := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalid, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(dir, "missing.json"), invalid} {
		engine := testEngine(path)
		for _, target := range []string{codeURL("Example alice"), "/api/labels"} {
			status, decoded, body := get(t, engine, target)
			if status != http.StatusInternalServerError || decoded["error"] == nil {
				t.Errorf("%s with %s: got %d %s", target, filepath.Base(path), status, body)
			}
		}
	}
}
