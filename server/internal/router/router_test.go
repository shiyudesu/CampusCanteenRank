package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestStallAndCanteenEndpoints(t *testing.T) {
	engine := NewEngine("test-secret-12345678901234567890")

	resp := requestNoBody(t, engine, http.MethodGet, "/api/v1/canteens")
	if resp.Code != http.StatusOK {
		t.Fatalf("canteens status = %d, want 200", resp.Code)
	}
	envelope := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, envelope["code"]); got != 0 {
		t.Fatalf("canteens code = %d, want 0", got)
	}
	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("canteens data should be object")
	}
	items, ok := data["items"].([]interface{})
	if !ok || len(items) == 0 {
		t.Fatalf("canteens items should not be empty")
	}

	resp = requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls?limit=2&sort=score_desc")
	if resp.Code != http.StatusOK {
		t.Fatalf("stalls status = %d, want 200", resp.Code)
	}
	firstPage := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, firstPage["code"]); got != 0 {
		t.Fatalf("stalls code = %d, want 0", got)
	}
	firstData, ok := firstPage["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("stalls data should be object")
	}
	firstItems, ok := firstData["items"].([]interface{})
	if !ok || len(firstItems) != 2 {
		t.Fatalf("first page items size = %d, want 2", len(firstItems))
	}
	firstItem, ok := firstItems[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first item should be object")
	}
	secondItem, ok := firstItems[1].(map[string]interface{})
	if !ok {
		t.Fatalf("second item should be object")
	}
	firstAvg, ok := firstItem["avgRating"].(float64)
	if !ok {
		t.Fatalf("first item avgRating should be number")
	}
	secondAvg, ok := secondItem["avgRating"].(float64)
	if !ok {
		t.Fatalf("second item avgRating should be number")
	}
	if firstAvg < secondAvg {
		t.Fatalf("items should be sorted by avgRating desc")
	}
	if hasMore, ok := firstData["hasMore"].(bool); !ok || !hasMore {
		t.Fatalf("first page hasMore should be true")
	}
	nextCursor, ok := firstData["nextCursor"].(string)
	if !ok || nextCursor == "" {
		t.Fatalf("first page nextCursor should not be empty")
	}

	resp = requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls?limit=2&sort=score_desc&cursor="+url.QueryEscape(nextCursor))
	if resp.Code != http.StatusOK {
		t.Fatalf("stalls second page status = %d, want 200", resp.Code)
	}
	secondPage := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, secondPage["code"]); got != 0 {
		t.Fatalf("stalls second page code = %d, want 0", got)
	}
	secondData, ok := secondPage["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("stalls second page data should be object")
	}
	secondItems, ok := secondData["items"].([]interface{})
	if !ok || len(secondItems) != 1 {
		t.Fatalf("second page items size = %d, want 1", len(secondItems))
	}
	if hasMore, ok := secondData["hasMore"].(bool); !ok || hasMore {
		t.Fatalf("second page hasMore should be false")
	}
	if _, exists := secondData["nextCursor"]; !exists {
		t.Fatalf("second page should contain nextCursor field")
	}

	resp = requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls/101")
	if resp.Code != http.StatusOK {
		t.Fatalf("stall detail status = %d, want 200", resp.Code)
	}
	detail := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, detail["code"]); got != 0 {
		t.Fatalf("stall detail code = %d, want 0", got)
	}
	detailData, ok := detail["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("stall detail data should be object")
	}
	if _, exists := detailData["myRating"]; exists {
		t.Fatalf("guest stall detail should not include myRating")
	}

	loginBody := map[string]interface{}{
		"email":    "detail@example.com",
		"nickname": "Detail",
		"password": "Pass@123456",
	}
	_ = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", loginBody)
	loginResp := requestJSON(t, engine, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    "detail@example.com",
		"password": "Pass@123456",
	})
	if loginResp.Code != http.StatusOK {
		t.Fatalf("detail login status = %d, want 200", loginResp.Code)
	}
	loginEnvelope := decodeEnvelope(t, loginResp.Body.Bytes())
	loginData, ok := loginEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("login data should be object")
	}
	accessToken, ok := loginData["accessToken"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("access token should not be empty")
	}
	authedResp := requestWithAuth(t, engine, http.MethodGet, "/api/v1/stalls/101", accessToken)
	if authedResp.Code != http.StatusOK {
		t.Fatalf("authed stall detail status = %d, want 200", authedResp.Code)
	}
	authedDetail := decodeEnvelope(t, authedResp.Body.Bytes())
	authedData, ok := authedDetail["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("authed stall detail data should be object")
	}
	if _, exists := authedData["myRating"]; !exists {
		t.Fatalf("authed stall detail should include myRating field")
	}

	resp = requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls/999999")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("stall detail not found status = %d, want 404", resp.Code)
	}
	notFound := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, notFound["code"]); got != 40401 {
		t.Fatalf("stall detail not found code = %d, want 40401", got)
	}
}

func TestStallListInvalidParams(t *testing.T) {
	engine := NewEngine("test-secret-12345678901234567890")

	resp := requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls?limit=bad")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit status = %d, want 400", resp.Code)
	}
	envelope := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, envelope["code"]); got != 40001 {
		t.Fatalf("invalid limit code = %d, want 40001", got)
	}

	resp = requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls?cursor=bad-cursor")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid cursor status = %d, want 400", resp.Code)
	}
	envelope = decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, envelope["code"]); got != 40001 {
		t.Fatalf("invalid cursor code = %d, want 40001", got)
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

func requestNoBody(t *testing.T, handler http.Handler, method string, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func requestWithAuth(t *testing.T, handler http.Handler, method string, path string, accessToken string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
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
