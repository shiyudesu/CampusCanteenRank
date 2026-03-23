package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthRegisterLoginRefreshFlow(t *testing.T) {
	engine := NewEngine("test-secret-12345678901234567890")

	registerBody := map[string]interface{}{
		"email":    "user@example.com",
		"nickname": "Tom",
		"password": "Pass@123456",
	}
	resp := requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", registerBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("register status = %d, want 200", resp.Code)
	}
	regEnvelope := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, regEnvelope["code"]); got != 0 {
		t.Fatalf("register code = %d, want 0", got)
	}

	loginBody := map[string]interface{}{
		"email":    "user@example.com",
		"password": "Pass@123456",
	}
	resp = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/login", loginBody)
	if resp.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", resp.Code)
	}
	loginEnvelope := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, loginEnvelope["code"]); got != 0 {
		t.Fatalf("login code = %d, want 0", got)
	}
	data, ok := loginEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("login data should be object")
	}
	refreshToken, ok := data["refreshToken"].(string)
	if !ok || refreshToken == "" {
		t.Fatalf("refreshToken should not be empty")
	}

	resp = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/refresh", map[string]interface{}{"refreshToken": refreshToken})
	if resp.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, want 200", resp.Code)
	}
	refreshEnvelope := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, refreshEnvelope["code"]); got != 0 {
		t.Fatalf("refresh code = %d, want 0", got)
	}

	resp = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/refresh", map[string]interface{}{"refreshToken": refreshToken})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("refresh reuse status = %d, want 401", resp.Code)
	}
	reuseEnvelope := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, reuseEnvelope["code"]); got != 40101 {
		t.Fatalf("refresh reuse code = %d, want 40101", got)
	}
}

func TestAuthErrorCases(t *testing.T) {
	engine := NewEngine("test-secret-12345678901234567890")

	resp := requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"email":    "bad-email",
		"nickname": "Tom",
		"password": "Pass@123456",
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("bad register status = %d, want 400", resp.Code)
	}
	badReg := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, badReg["code"]); got != 40001 {
		t.Fatalf("bad register code = %d, want 40001", got)
	}

	validRegister := map[string]interface{}{
		"email":    "dup@example.com",
		"nickname": "Tom",
		"password": "Pass@123456",
	}
	resp = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", validRegister)
	if resp.Code != http.StatusOK {
		t.Fatalf("first register status = %d, want 200", resp.Code)
	}
	resp = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", validRegister)
	if resp.Code != http.StatusConflict {
		t.Fatalf("duplicate register status = %d, want 409", resp.Code)
	}
	dupReg := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, dupReg["code"]); got != 40901 {
		t.Fatalf("duplicate register code = %d, want 40901", got)
	}

	resp = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    "dup@example.com",
		"password": "wrongpass123",
	})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("wrong login status = %d, want 401", resp.Code)
	}
	wrongLogin := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, wrongLogin["code"]); got != 40101 {
		t.Fatalf("wrong login code = %d, want 40101", got)
	}

	resp = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/refresh", map[string]interface{}{"refreshToken": "not-a-jwt"})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("bad refresh status = %d, want 401", resp.Code)
	}
	badRefresh := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, badRefresh["code"]); got != 40101 {
		t.Fatalf("bad refresh code = %d, want 40101", got)
	}
}

func requestJSON(t *testing.T, handler http.Handler, method string, path string, body map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func decodeEnvelope(t *testing.T, raw []byte) map[string]interface{} {
	t.Helper()
	var got map[string]interface{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode envelope failed: %v", err)
	}
	if _, ok := got["code"]; !ok {
		t.Fatalf("envelope missing code")
	}
	if _, ok := got["message"]; !ok {
		t.Fatalf("envelope missing message")
	}
	if _, ok := got["data"]; !ok {
		t.Fatalf("envelope missing data")
	}
	return got
}

func asInt(t *testing.T, v interface{}) int {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("value should be float64 in json map")
	}
	return int(f)
}
