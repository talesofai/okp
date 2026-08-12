package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/talesofai/okp/internal/config"
	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/testutil"
)

func TestPrivateDomainHTTPPermissionsAndDeletion(t *testing.T) {
	db := testutil.OpenDatabase(t)
	previousConfig := config.C
	config.C = config.Config{ExecutionGrantKey: "test-key"}
	t.Cleanup(func() { config.C = previousConfig })

	for _, user := range []model.User{
		{UUID: "creator", AuthType: "test", Role: "reader"},
		{UUID: "global-admin", AuthType: "test", Role: "admin"},
		{UUID: "writer", AuthType: "test", Role: "reader"},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}

	router := NewRouter()
	creatorToken := testExecutionToken(t, "test-key", "creator")
	adminToken := testExecutionToken(t, "test-key", "global-admin")
	writerToken := testExecutionToken(t, "test-key", "writer")

	res := testRequest(t, router, creatorToken, http.MethodPut, "/api/v1/domains/private-team", map[string]any{
		"readme":     "# Private team",
		"visibility": "private",
	})
	assertStatus(t, res, http.StatusCreated)

	res = testRequest(t, router, creatorToken, http.MethodPut, "/api/v1/concepts/private-team/Note/plan", map[string]any{
		"domain": "private-team",
		"type":   "Note",
		"title":  "Plan",
		"provenance": map[string]any{
			"source": "test",
			"agent":  "test",
		},
	})
	assertStatus(t, res, http.StatusOK)

	res = testRequest(t, router, adminToken, http.MethodGet, "/api/v1/domains", nil)
	assertStatus(t, res, http.StatusOK)
	var domains []map[string]any
	decodeResponse(t, res, &domains)
	if len(domains) != 0 {
		t.Fatalf("global admin discovered private domain: %+v", domains)
	}

	res = testRequest(t, router, adminToken, http.MethodGet, "/api/v1/concepts?q=Plan", nil)
	assertStatus(t, res, http.StatusOK)
	var concepts []map[string]any
	decodeResponse(t, res, &concepts)
	if len(concepts) != 0 || res.Header().Get("X-Total-Count") != "0" {
		t.Fatalf("global admin searched private concept: %+v", concepts)
	}

	assertStatus(t, testRequest(t, router, adminToken, http.MethodGet, "/api/v1/concepts/private-team/Note/plan", nil), http.StatusNotFound)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodGet, "/api/v1/domains/private-team/members", nil), http.StatusNotFound)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodDelete, "/api/v1/domains/private-team", nil), http.StatusNotFound)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodPut, "/api/v1/domains/public-team", map[string]any{
		"readme": "# Public team",
	}), http.StatusCreated)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodPut, "/api/v1/concepts/private-team/Note/plan", map[string]any{
		"domain": "public-team",
		"type":   "Note",
		"title":  "Probe",
		"provenance": map[string]any{
			"source": "test",
			"agent":  "test",
		},
	}), http.StatusNotFound)

	readerInvite := createHTTPInvite(t, router, creatorToken, "private-team", "reader")
	res = testRequest(t, router, adminToken, http.MethodPost, "/api/v1/invites/accept", map[string]any{"code": readerInvite})
	assertStatus(t, res, http.StatusOK)

	assertStatus(t, testRequest(t, router, adminToken, http.MethodGet, "/api/v1/concepts/private-team/Note/plan", nil), http.StatusOK)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodGet, "/api/v1/domains/private-team/members", nil), http.StatusOK)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodPut, "/api/v1/concepts/private-team/Note/plan", map[string]any{
		"domain": "private-team",
		"type":   "Note",
		"title":  "Changed",
		"provenance": map[string]any{
			"source": "test",
			"agent":  "test",
		},
	}), http.StatusForbidden)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodDelete, "/api/v1/concepts/private-team/Note/plan", nil), http.StatusForbidden)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodPut, "/api/v1/links/private-team/Note/plan", map[string]any{"links": []any{}}), http.StatusForbidden)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodPost, "/api/v1/embed/batch?domain=private-team", nil), http.StatusForbidden)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodPost, "/api/v1/domains/private-team/invites", map[string]any{"role": "writer"}), http.StatusForbidden)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodDelete, "/api/v1/domains/private-team", nil), http.StatusForbidden)

	writerInvite := createHTTPInvite(t, router, creatorToken, "private-team", "writer")
	assertStatus(t, testRequest(t, router, writerToken, http.MethodPost, "/api/v1/invites/accept", map[string]any{"code": writerInvite}), http.StatusOK)
	assertStatus(t, testRequest(t, router, writerToken, http.MethodPut, "/api/v1/concepts/private-team/Note/draft", map[string]any{
		"domain": "private-team",
		"type":   "Note",
		"title":  "Draft",
		"provenance": map[string]any{
			"source": "test",
			"agent":  "test",
		},
	}), http.StatusOK)
	assertStatus(t, testRequest(t, router, writerToken, http.MethodDelete, "/api/v1/domains/private-team", nil), http.StatusForbidden)
	assertStatus(t, testRequest(t, router, writerToken, http.MethodDelete, "/api/v1/concepts/private-team/Note/draft", nil), http.StatusNoContent)

	assertStatus(t, testRequest(t, router, creatorToken, http.MethodDelete, "/api/v1/domains/private-team", nil), http.StatusNoContent)
	assertStatus(t, testRequest(t, router, creatorToken, http.MethodGet, "/api/v1/domains/private-team", nil), http.StatusNotFound)
	assertStatus(t, testRequest(t, router, adminToken, http.MethodGet, "/api/v1/concepts/private-team/Note/plan", nil), http.StatusNotFound)
}

func createHTTPInvite(t *testing.T, handler http.Handler, token, domain, role string) string {
	t.Helper()
	res := testRequest(t, handler, token, http.MethodPost, "/api/v1/domains/"+domain+"/invites", map[string]any{
		"role":             role,
		"expires_in_hours": 1,
		"max_uses":         1,
	})
	assertStatus(t, res, http.StatusCreated)
	var body map[string]any
	decodeResponse(t, res, &body)
	code, ok := body["code"].(string)
	if !ok || code == "" {
		t.Fatalf("invite response omitted code: %+v", body)
	}
	return code
}

func testRequest(t *testing.T, handler http.Handler, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func decodeResponse(t *testing.T, res *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(res.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response %q: %v", res.Body.String(), err)
	}
}

func assertStatus(t *testing.T, res *httptest.ResponseRecorder, want int) {
	t.Helper()
	if res.Code != want {
		t.Fatalf("status=%d want=%d body=%s", res.Code, want, res.Body.String())
	}
}

func testExecutionToken(t *testing.T, key, userID string) string {
	t.Helper()
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
	payload := map[string]any{
		"actorUserId": userID,
		"spaceId":     "test-space",
		"sessionId":   "test-session",
		"turnId":      "test-turn",
		"source":      "test",
		"iat":         time.Now().Unix(),
		"exp":         time.Now().Add(time.Hour).Unix(),
	}
	encode := func(value any) string {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(data)
	}
	signingInput := encode(header) + "." + encode(payload)
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(signingInput))
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
