package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"CampusCanteenRank/server/internal/testkit"
)

func newTestEngine(secret string) http.Handler {
	return NewEngineWithAllRepositories(
		secret,
		testkit.NewUserRepository(),
		testkit.NewRefreshTokenRepository(),
		testkit.NewStallRepository(),
		testkit.NewCommentRepository(),
		testkit.NewRankingRepository(),
	)
}

func TestAuthRegisterLoginRefreshFlow(t *testing.T) {
	engine := newTestEngine("test-secret-12345678901234567890")

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
	engine := newTestEngine("test-secret-12345678901234567890")

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
	engine := newTestEngine("test-secret-12345678901234567890")

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
	if value, exists := detailData["myRating"]; !exists || value != nil {
		t.Fatalf("guest stall detail myRating should be null")
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
	engine := newTestEngine("test-secret-12345678901234567890")

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

func TestUpsertStallRatingEndpoint(t *testing.T) {
	engine := newTestEngine("test-secret-12345678901234567890")

	_ = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"email":    "rating@example.com",
		"nickname": "Rater",
		"password": "Pass@123456",
	})
	loginResp := requestJSON(t, engine, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    "rating@example.com",
		"password": "Pass@123456",
	})
	if loginResp.Code != http.StatusOK {
		t.Fatalf("rating login status = %d, want 200", loginResp.Code)
	}
	loginEnvelope := decodeEnvelope(t, loginResp.Body.Bytes())
	loginData, ok := loginEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("rating login data should be object")
	}
	accessToken, ok := loginData["accessToken"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("rating access token should not be empty")
	}

	resp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/ratings", map[string]interface{}{"score": 4}, accessToken)
	if resp.Code != http.StatusOK {
		t.Fatalf("upsert rating status = %d, want 200", resp.Code)
	}
	envelope := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, envelope["code"]); got != 0 {
		t.Fatalf("upsert rating code = %d, want 0", got)
	}
	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("upsert rating data should be object")
	}
	if got := asInt(t, data["stallId"]); got != 101 {
		t.Fatalf("upsert rating stallId = %d, want 101", got)
	}
	if got := asInt(t, data["score"]); got != 4 {
		t.Fatalf("upsert rating score = %d, want 4", got)
	}
	createdCount := asInt(t, data["ratingCount"])
	if createdCount != 533 {
		t.Fatalf("upsert rating count = %d, want 533", createdCount)
	}
	createdAvg := asFloat(t, data["avgRating"])
	assertFloatNear(t, createdAvg, 2451.2/533.0, 1e-9)

	detailResp := requestWithAuth(t, engine, http.MethodGet, "/api/v1/stalls/101", accessToken)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail after upsert status = %d, want 200", detailResp.Code)
	}
	detailEnvelope := decodeEnvelope(t, detailResp.Body.Bytes())
	detailData, ok := detailEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("detail data should be object")
	}
	if got := asInt(t, detailData["myRating"]); got != 4 {
		t.Fatalf("detail myRating = %d, want 4", got)
	}

	resp = requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/ratings", map[string]interface{}{"score": 2}, accessToken)
	if resp.Code != http.StatusOK {
		t.Fatalf("update rating status = %d, want 200", resp.Code)
	}
	updated := decodeEnvelope(t, resp.Body.Bytes())
	updatedData, ok := updated["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("updated rating data should be object")
	}
	if got := asInt(t, updatedData["score"]); got != 2 {
		t.Fatalf("updated rating score = %d, want 2", got)
	}
	updatedCount := asInt(t, updatedData["ratingCount"])
	if updatedCount != 533 {
		t.Fatalf("updated rating count = %d, want 533", updatedCount)
	}
	updatedAvg := asFloat(t, updatedData["avgRating"])
	assertFloatNear(t, updatedAvg, 2449.2/533.0, 1e-9)

	detailResp = requestWithAuth(t, engine, http.MethodGet, "/api/v1/stalls/101", accessToken)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail after update status = %d, want 200", detailResp.Code)
	}
	detailEnvelope = decodeEnvelope(t, detailResp.Body.Bytes())
	detailData, ok = detailEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("detail data after update should be object")
	}
	if got := asInt(t, detailData["myRating"]); got != 2 {
		t.Fatalf("detail myRating after update = %d, want 2", got)
	}

	resp = requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/ratings", map[string]interface{}{"score": 0}, accessToken)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid score status = %d, want 400", resp.Code)
	}
	badReq := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, badReq["code"]); got != 40001 {
		t.Fatalf("invalid score code = %d, want 40001", got)
	}

	resp = requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/ratings", map[string]interface{}{"score": 6}, accessToken)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("score too large status = %d, want 400", resp.Code)
	}
	tooLarge := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, tooLarge["code"]); got != 40001 {
		t.Fatalf("score too large code = %d, want 40001", got)
	}

	resp = requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/ratings", map[string]interface{}{}, accessToken)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("missing score status = %d, want 400", resp.Code)
	}
	missing := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, missing["code"]); got != 40001 {
		t.Fatalf("missing score code = %d, want 40001", got)
	}

	resp = requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/ratings", map[string]interface{}{"score": "bad"}, accessToken)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("wrong score type status = %d, want 400", resp.Code)
	}
	wrongType := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, wrongType["code"]); got != 40001 {
		t.Fatalf("wrong score type code = %d, want 40001", got)
	}

	resp = requestJSON(t, engine, http.MethodPost, "/api/v1/stalls/101/ratings", map[string]interface{}{"score": 5})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("guest upsert status = %d, want 401", resp.Code)
	}
	unauthorized := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, unauthorized["code"]); got != 40101 {
		t.Fatalf("guest upsert code = %d, want 40101", got)
	}

	resp = requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/ratings", map[string]interface{}{"score": 5}, "bad-token")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token upsert status = %d, want 401", resp.Code)
	}
	badToken := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, badToken["code"]); got != 40101 {
		t.Fatalf("invalid token upsert code = %d, want 40101", got)
	}

	resp = requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/999999/ratings", map[string]interface{}{"score": 5}, accessToken)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("upsert unknown stall status = %d, want 404", resp.Code)
	}
	notFound := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, notFound["code"]); got != 40401 {
		t.Fatalf("upsert unknown stall code = %d, want 40401", got)
	}
}

func TestCommentCreateAndListTopLevel(t *testing.T) {
	engine := newTestEngine("test-secret-12345678901234567890")

	_ = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"email":    "commenter@example.com",
		"nickname": "Commenter",
		"password": "Pass@123456",
	})
	loginResp := requestJSON(t, engine, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    "commenter@example.com",
		"password": "Pass@123456",
	})
	if loginResp.Code != http.StatusOK {
		t.Fatalf("comment login status = %d, want 200", loginResp.Code)
	}
	loginEnvelope := decodeEnvelope(t, loginResp.Body.Bytes())
	loginData, ok := loginEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("comment login data should be object")
	}
	accessToken, ok := loginData["accessToken"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("comment access token should not be empty")
	}

	createResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/comments", map[string]interface{}{
		"content":       "新开的配菜窗口挺不错",
		"rootId":        0,
		"parentId":      0,
		"replyToUserId": 0,
	}, accessToken)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create top-level comment status = %d, want 200", createResp.Code)
	}
	createdEnvelope := decodeEnvelope(t, createResp.Body.Bytes())
	if got := asInt(t, createdEnvelope["code"]); got != 0 {
		t.Fatalf("create top-level comment code = %d, want 0", got)
	}
	createdData, ok := createdEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("created comment data should be object")
	}
	createdComment, ok := createdData["comment"].(map[string]interface{})
	if !ok {
		t.Fatalf("created comment should be object")
	}
	if got := asInt(t, createdComment["stallId"]); got != 101 {
		t.Fatalf("created comment stallId = %d, want 101", got)
	}
	if got := asInt(t, createdComment["rootId"]); got != 0 {
		t.Fatalf("created comment rootId = %d, want 0", got)
	}
	if got := asInt(t, createdComment["parentId"]); got != 0 {
		t.Fatalf("created comment parentId = %d, want 0", got)
	}
	if got := asInt(t, createdComment["replyToUserId"]); got != 0 {
		t.Fatalf("created comment replyToUserId = %d, want 0", got)
	}
	if content := asString(t, createdComment["content"]); content != "新开的配菜窗口挺不错" {
		t.Fatalf("created comment content = %q", content)
	}
	author, ok := createdComment["author"].(map[string]interface{})
	if !ok {
		t.Fatalf("created comment author should be object")
	}
	authorID := asInt(t, author["id"])
	if authorID <= 0 {
		t.Fatalf("created comment author id should be positive")
	}
	if nickname := asString(t, author["nickname"]); nickname != "Commenter" {
		t.Fatalf("created comment nickname = %q, want Commenter", nickname)
	}
	if likedByMe, ok := createdComment["likedByMe"].(bool); !ok || likedByMe {
		t.Fatalf("created comment likedByMe should be false")
	}
	if createdAt := asString(t, createdComment["createdAt"]); createdAt == "" {
		t.Fatalf("created comment createdAt should not be empty")
	}
	createdRootID := asInt(t, createdComment["id"])

	listResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls/101/comments?limit=2&sort=latest")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list top-level comments status = %d, want 200", listResp.Code)
	}
	listEnvelope := decodeEnvelope(t, listResp.Body.Bytes())
	if got := asInt(t, listEnvelope["code"]); got != 0 {
		t.Fatalf("list top-level comments code = %d, want 0", got)
	}
	listData, ok := listEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("list data should be object")
	}
	items, ok := listData["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("first page items size = %d, want 2", len(items))
	}
	firstItem, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first page first item should be object")
	}
	if got := asInt(t, firstItem["id"]); got != createdRootID {
		t.Fatalf("latest comment id = %d, want %d", got, createdRootID)
	}
	if hasMore, ok := listData["hasMore"].(bool); !ok || !hasMore {
		t.Fatalf("first page hasMore should be true")
	}
	nextCursor, ok := listData["nextCursor"].(string)
	if !ok || nextCursor == "" {
		t.Fatalf("first page nextCursor should not be empty")
	}
	secondInFirst, ok := items[1].(map[string]interface{})
	if !ok {
		t.Fatalf("first page second item should be object")
	}
	secondInFirstID := asInt(t, secondInFirst["id"])

	secondResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls/101/comments?limit=2&sort=latest&cursor="+url.QueryEscape(nextCursor))
	if secondResp.Code != http.StatusOK {
		t.Fatalf("list second page status = %d, want 200", secondResp.Code)
	}
	secondEnvelope := decodeEnvelope(t, secondResp.Body.Bytes())
	if got := asInt(t, secondEnvelope["code"]); got != 0 {
		t.Fatalf("list second page code = %d, want 0", got)
	}
	secondData, ok := secondEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("second page data should be object")
	}
	secondItems, ok := secondData["items"].([]interface{})
	if !ok || len(secondItems) != 2 {
		t.Fatalf("second page items size = %d, want 2", len(secondItems))
	}
	for _, raw := range secondItems {
		item, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("second page item should be object")
		}
		if got := asInt(t, item["id"]); got == secondInFirstID {
			t.Fatalf("cursor paging should not duplicate id=%d", got)
		}
	}
	if hasMore, ok := secondData["hasMore"].(bool); !ok || hasMore {
		t.Fatalf("second page hasMore should be false")
	}

	replyResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/comments", map[string]interface{}{
		"content":       "补充：晚餐时段价格也稳定",
		"rootId":        createdRootID,
		"parentId":      createdRootID,
		"replyToUserId": authorID,
	}, accessToken)
	if replyResp.Code != http.StatusOK {
		t.Fatalf("create reply status = %d, want 200", replyResp.Code)
	}
	replyEnvelope := decodeEnvelope(t, replyResp.Body.Bytes())
	if got := asInt(t, replyEnvelope["code"]); got != 0 {
		t.Fatalf("create reply code = %d, want 0", got)
	}
	replyData, ok := replyEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("reply data should be object")
	}
	replyComment, ok := replyData["comment"].(map[string]interface{})
	if !ok {
		t.Fatalf("reply comment should be object")
	}
	if got := asInt(t, replyComment["rootId"]); got != createdRootID {
		t.Fatalf("reply rootId = %d, want %d", got, createdRootID)
	}
	if got := asInt(t, replyComment["parentId"]); got != createdRootID {
		t.Fatalf("reply parentId = %d, want %d", got, createdRootID)
	}
	if got := asInt(t, replyComment["replyToUserId"]); got != authorID {
		t.Fatalf("reply replyToUserId = %d, want %d", got, authorID)
	}

	refreshFirstPage := requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls/101/comments?limit=2&sort=latest")
	if refreshFirstPage.Code != http.StatusOK {
		t.Fatalf("refresh first page status = %d, want 200", refreshFirstPage.Code)
	}
	refreshEnvelope := decodeEnvelope(t, refreshFirstPage.Body.Bytes())
	refreshData, ok := refreshEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("refresh list data should be object")
	}
	refreshItems, ok := refreshData["items"].([]interface{})
	if !ok || len(refreshItems) == 0 {
		t.Fatalf("refresh list items should not be empty")
	}
	latestRoot, ok := refreshItems[0].(map[string]interface{})
	if !ok {
		t.Fatalf("latest root should be object")
	}
	if got := asInt(t, latestRoot["replyCount"]); got != 1 {
		t.Fatalf("latest root replyCount = %d, want 1", got)
	}

	badCursorResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls/101/comments?cursor=bad-cursor")
	if badCursorResp.Code != http.StatusBadRequest {
		t.Fatalf("bad comment cursor status = %d, want 400", badCursorResp.Code)
	}
	badCursorEnvelope := decodeEnvelope(t, badCursorResp.Body.Bytes())
	if got := asInt(t, badCursorEnvelope["code"]); got != 40001 {
		t.Fatalf("bad comment cursor code = %d, want 40001", got)
	}

	badSortResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/stalls/101/comments?sort=hot")
	if badSortResp.Code != http.StatusBadRequest {
		t.Fatalf("bad comment sort status = %d, want 400", badSortResp.Code)
	}
	badSortEnvelope := decodeEnvelope(t, badSortResp.Body.Bytes())
	if got := asInt(t, badSortEnvelope["code"]); got != 40001 {
		t.Fatalf("bad comment sort code = %d, want 40001", got)
	}

	guestCreateResp := requestJSON(t, engine, http.MethodPost, "/api/v1/stalls/101/comments", map[string]interface{}{
		"content":       "游客不应可发评论",
		"rootId":        0,
		"parentId":      0,
		"replyToUserId": 0,
	})
	if guestCreateResp.Code != http.StatusUnauthorized {
		t.Fatalf("guest create comment status = %d, want 401", guestCreateResp.Code)
	}
	guestCreateEnvelope := decodeEnvelope(t, guestCreateResp.Body.Bytes())
	if got := asInt(t, guestCreateEnvelope["code"]); got != 40101 {
		t.Fatalf("guest create comment code = %d, want 40101", got)
	}

	invalidReplyResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/comments", map[string]interface{}{
		"content":       "replyToUserId 与 parent 不一致",
		"rootId":        createdRootID,
		"parentId":      createdRootID,
		"replyToUserId": authorID + 999,
	}, accessToken)
	if invalidReplyResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid reply relation status = %d, want 400", invalidReplyResp.Code)
	}
	invalidReplyEnvelope := decodeEnvelope(t, invalidReplyResp.Body.Bytes())
	if got := asInt(t, invalidReplyEnvelope["code"]); got != 40001 {
		t.Fatalf("invalid reply relation code = %d, want 40001", got)
	}

	notFoundStallResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/999999/comments", map[string]interface{}{
		"content":       "不存在窗口",
		"rootId":        0,
		"parentId":      0,
		"replyToUserId": 0,
	}, accessToken)
	if notFoundStallResp.Code != http.StatusNotFound {
		t.Fatalf("create comment on unknown stall status = %d, want 404", notFoundStallResp.Code)
	}
	notFoundStallEnvelope := decodeEnvelope(t, notFoundStallResp.Body.Bytes())
	if got := asInt(t, notFoundStallEnvelope["code"]); got != 40401 {
		t.Fatalf("create comment on unknown stall code = %d, want 40401", got)
	}
}

func TestCommentCursorPaginationSameSecondNoLoss(t *testing.T) {
	engine := newTestEngine("test-secret-12345678901234567890")

	_ = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"email":    "same-second@example.com",
		"nickname": "SameSecond",
		"password": "Pass@123456",
	})
	loginResp := requestJSON(t, engine, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    "same-second@example.com",
		"password": "Pass@123456",
	})
	if loginResp.Code != http.StatusOK {
		t.Fatalf("same-second login status = %d, want 200", loginResp.Code)
	}
	loginEnvelope := decodeEnvelope(t, loginResp.Body.Bytes())
	loginData, ok := loginEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("same-second login data should be object")
	}
	accessToken, ok := loginData["accessToken"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("same-second access token should not be empty")
	}

	createdIDs := make(map[int]int)
	for i := 0; i < 4; i++ {
		resp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/comments", map[string]interface{}{
			"content":       "same-second comment #" + strconv.Itoa(i),
			"rootId":        0,
			"parentId":      0,
			"replyToUserId": 0,
		}, accessToken)
		if resp.Code != http.StatusOK {
			t.Fatalf("create same-second comment[%d] status = %d, want 200", i, resp.Code)
		}
		env := decodeEnvelope(t, resp.Body.Bytes())
		data, ok := env["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("create same-second data should be object")
		}
		comment, ok := data["comment"].(map[string]interface{})
		if !ok {
			t.Fatalf("create same-second comment should be object")
		}
		id := asInt(t, comment["id"])
		createdIDs[id] = i
	}

	seen := make(map[int]struct{})
	var cursorText string
	for page := 0; page < 20; page++ {
		path := "/api/v1/stalls/101/comments?limit=1&sort=latest"
		if cursorText != "" {
			path += "&cursor=" + url.QueryEscape(cursorText)
		}
		resp := requestNoBody(t, engine, http.MethodGet, path)
		if resp.Code != http.StatusOK {
			t.Fatalf("same-second list page[%d] status = %d, want 200", page, resp.Code)
		}
		env := decodeEnvelope(t, resp.Body.Bytes())
		data, ok := env["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("same-second list data should be object")
		}
		items, ok := data["items"].([]interface{})
		if !ok {
			t.Fatalf("same-second list items should be array")
		}
		if len(items) == 0 {
			break
		}
		item, ok := items[0].(map[string]interface{})
		if !ok {
			t.Fatalf("same-second item should be object")
		}
		id := asInt(t, item["id"])
		if _, exists := seen[id]; exists {
			t.Fatalf("same-second pagination duplicated comment id=%d", id)
		}
		seen[id] = struct{}{}

		hasMore, ok := data["hasMore"].(bool)
		if !ok {
			t.Fatalf("same-second hasMore should be bool")
		}
		nextCursor, _ := data["nextCursor"].(string)
		if !hasMore {
			break
		}
		if nextCursor == "" {
			t.Fatalf("same-second hasMore=true requires non-empty nextCursor")
		}
		cursorText = nextCursor
	}

	if len(seen) < len(createdIDs) {
		for id := range createdIDs {
			if _, exists := seen[id]; !exists {
				t.Fatalf("same-second pagination missed created comment id=%d", id)
			}
		}
	}
}

func TestCommentLikeEndpoints(t *testing.T) {
	engine := newTestEngine("test-secret-12345678901234567890")

	_ = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"email":    "liker@example.com",
		"nickname": "Liker",
		"password": "Pass@123456",
	})
	loginResp := requestJSON(t, engine, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    "liker@example.com",
		"password": "Pass@123456",
	})
	if loginResp.Code != http.StatusOK {
		t.Fatalf("like login status = %d, want 200", loginResp.Code)
	}
	loginEnvelope := decodeEnvelope(t, loginResp.Body.Bytes())
	loginData, ok := loginEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("like login data should be object")
	}
	accessToken, ok := loginData["accessToken"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("like access token should not be empty")
	}

	createResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/comments", map[string]interface{}{
		"content":       "点赞接口测试评论",
		"rootId":        0,
		"parentId":      0,
		"replyToUserId": 0,
	}, accessToken)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create comment for like status = %d, want 200", createResp.Code)
	}
	createdEnvelope := decodeEnvelope(t, createResp.Body.Bytes())
	createdData, ok := createdEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("create comment for like data should be object")
	}
	createdComment, ok := createdData["comment"].(map[string]interface{})
	if !ok {
		t.Fatalf("create comment for like comment should be object")
	}
	commentID := asInt(t, createdComment["id"])

	likePath := "/api/v1/comments/" + strconv.Itoa(commentID) + "/like"

	guestLikeResp := requestJSON(t, engine, http.MethodPost, likePath, map[string]interface{}{})
	if guestLikeResp.Code != http.StatusUnauthorized {
		t.Fatalf("guest like status = %d, want 401", guestLikeResp.Code)
	}
	guestLikeEnvelope := decodeEnvelope(t, guestLikeResp.Body.Bytes())
	if got := asInt(t, guestLikeEnvelope["code"]); got != 40101 {
		t.Fatalf("guest like code = %d, want 40101", got)
	}
	if _, ok := guestLikeEnvelope["data"].(map[string]interface{}); !ok {
		t.Fatalf("guest like data should be object")
	}

	invalidLikeResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/comments/abc/like", map[string]interface{}{}, accessToken)
	if invalidLikeResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid comment id like status = %d, want 400", invalidLikeResp.Code)
	}
	invalidLikeEnvelope := decodeEnvelope(t, invalidLikeResp.Body.Bytes())
	if got := asInt(t, invalidLikeEnvelope["code"]); got != 40001 {
		t.Fatalf("invalid comment id like code = %d, want 40001", got)
	}

	invalidZeroLikeResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/comments/0/like", map[string]interface{}{}, accessToken)
	if invalidZeroLikeResp.Code != http.StatusBadRequest {
		t.Fatalf("zero comment id like status = %d, want 400", invalidZeroLikeResp.Code)
	}
	invalidZeroLikeEnvelope := decodeEnvelope(t, invalidZeroLikeResp.Body.Bytes())
	if got := asInt(t, invalidZeroLikeEnvelope["code"]); got != 40001 {
		t.Fatalf("zero comment id like code = %d, want 40001", got)
	}

	invalidNegativeUnlikeResp := requestJSONWithAuth(t, engine, http.MethodDelete, "/api/v1/comments/-1/like", map[string]interface{}{}, accessToken)
	if invalidNegativeUnlikeResp.Code != http.StatusBadRequest {
		t.Fatalf("negative comment id unlike status = %d, want 400", invalidNegativeUnlikeResp.Code)
	}
	invalidNegativeUnlikeEnvelope := decodeEnvelope(t, invalidNegativeUnlikeResp.Body.Bytes())
	if got := asInt(t, invalidNegativeUnlikeEnvelope["code"]); got != 40001 {
		t.Fatalf("negative comment id unlike code = %d, want 40001", got)
	}

	likeResp := requestJSONWithAuth(t, engine, http.MethodPost, likePath, map[string]interface{}{}, accessToken)
	if likeResp.Code != http.StatusOK {
		t.Fatalf("like status = %d, want 200", likeResp.Code)
	}
	likeEnvelope := decodeEnvelope(t, likeResp.Body.Bytes())
	if got := asInt(t, likeEnvelope["code"]); got != 0 {
		t.Fatalf("like code = %d, want 0", got)
	}
	likeData, ok := likeEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("like data should be object")
	}
	if liked, ok := likeData["liked"].(bool); !ok || !liked {
		t.Fatalf("like liked should be true")
	}
	if got := asInt(t, likeData["likeCount"]); got != 1 {
		t.Fatalf("like count = %d, want 1", got)
	}

	idempotentLikeResp := requestJSONWithAuth(t, engine, http.MethodPost, likePath, map[string]interface{}{}, accessToken)
	if idempotentLikeResp.Code != http.StatusOK {
		t.Fatalf("idempotent like status = %d, want 200", idempotentLikeResp.Code)
	}
	idempotentLikeEnvelope := decodeEnvelope(t, idempotentLikeResp.Body.Bytes())
	idempotentLikeData, ok := idempotentLikeEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("idempotent like data should be object")
	}
	if got := asInt(t, idempotentLikeData["likeCount"]); got != 1 {
		t.Fatalf("idempotent like count = %d, want 1", got)
	}

	unlikeResp := requestJSONWithAuth(t, engine, http.MethodDelete, likePath, map[string]interface{}{}, accessToken)
	if unlikeResp.Code != http.StatusOK {
		t.Fatalf("unlike status = %d, want 200", unlikeResp.Code)
	}
	unlikeEnvelope := decodeEnvelope(t, unlikeResp.Body.Bytes())
	unlikeData, ok := unlikeEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("unlike data should be object")
	}
	if liked, ok := unlikeData["liked"].(bool); !ok || liked {
		t.Fatalf("unlike liked should be false")
	}
	if got := asInt(t, unlikeData["likeCount"]); got != 0 {
		t.Fatalf("unlike count = %d, want 0", got)
	}

	idempotentUnlikeResp := requestJSONWithAuth(t, engine, http.MethodDelete, likePath, map[string]interface{}{}, accessToken)
	if idempotentUnlikeResp.Code != http.StatusOK {
		t.Fatalf("idempotent unlike status = %d, want 200", idempotentUnlikeResp.Code)
	}
	idempotentUnlikeEnvelope := decodeEnvelope(t, idempotentUnlikeResp.Body.Bytes())
	idempotentUnlikeData, ok := idempotentUnlikeEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("idempotent unlike data should be object")
	}
	if got := asInt(t, idempotentUnlikeData["likeCount"]); got != 0 {
		t.Fatalf("idempotent unlike count = %d, want 0", got)
	}

	notFoundResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/comments/999999/like", map[string]interface{}{}, accessToken)
	if notFoundResp.Code != http.StatusNotFound {
		t.Fatalf("like unknown comment status = %d, want 404", notFoundResp.Code)
	}
	notFoundEnvelope := decodeEnvelope(t, notFoundResp.Body.Bytes())
	if got := asInt(t, notFoundEnvelope["code"]); got != 40401 {
		t.Fatalf("like unknown comment code = %d, want 40401", got)
	}

	notFoundUnlikeResp := requestJSONWithAuth(t, engine, http.MethodDelete, "/api/v1/comments/999999/like", map[string]interface{}{}, accessToken)
	if notFoundUnlikeResp.Code != http.StatusNotFound {
		t.Fatalf("unlike unknown comment status = %d, want 404", notFoundUnlikeResp.Code)
	}
	notFoundUnlikeEnvelope := decodeEnvelope(t, notFoundUnlikeResp.Body.Bytes())
	if got := asInt(t, notFoundUnlikeEnvelope["code"]); got != 40401 {
		t.Fatalf("unlike unknown comment code = %d, want 40401", got)
	}
}

func TestCommentListRepliesEndpoint(t *testing.T) {
	engine := newTestEngine("test-secret-12345678901234567890")

	_ = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"email":    "replier@example.com",
		"nickname": "Replier",
		"password": "Pass@123456",
	})
	loginResp := requestJSON(t, engine, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    "replier@example.com",
		"password": "Pass@123456",
	})
	if loginResp.Code != http.StatusOK {
		t.Fatalf("reply login status = %d, want 200", loginResp.Code)
	}
	loginEnvelope := decodeEnvelope(t, loginResp.Body.Bytes())
	loginData, ok := loginEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("reply login data should be object")
	}
	accessToken, ok := loginData["accessToken"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("reply access token should not be empty")
	}

	createRoot := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/comments", map[string]interface{}{
		"content":       "root for replies",
		"rootId":        0,
		"parentId":      0,
		"replyToUserId": 0,
	}, accessToken)
	if createRoot.Code != http.StatusOK {
		t.Fatalf("create root comment status = %d, want 200", createRoot.Code)
	}
	rootEnvelope := decodeEnvelope(t, createRoot.Body.Bytes())
	rootData, ok := rootEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("root data should be object")
	}
	rootComment, ok := rootData["comment"].(map[string]interface{})
	if !ok {
		t.Fatalf("root comment should be object")
	}
	rootID := asInt(t, rootComment["id"])
	author, ok := rootComment["author"].(map[string]interface{})
	if !ok {
		t.Fatalf("root author should be object")
	}
	authorID := asInt(t, author["id"])

	for i := 0; i < 3; i++ {
		replyResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/comments", map[string]interface{}{
			"content":       "reply #" + strconv.Itoa(i),
			"rootId":        rootID,
			"parentId":      rootID,
			"replyToUserId": authorID,
		}, accessToken)
		if replyResp.Code != http.StatusOK {
			t.Fatalf("create reply[%d] status = %d, want 200", i, replyResp.Code)
		}
	}

	firstResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/comments/"+strconv.Itoa(rootID)+"/replies?limit=2")
	if firstResp.Code != http.StatusOK {
		t.Fatalf("list replies first page status = %d, want 200", firstResp.Code)
	}
	firstEnvelope := decodeEnvelope(t, firstResp.Body.Bytes())
	if got := asInt(t, firstEnvelope["code"]); got != 0 {
		t.Fatalf("list replies first page code = %d, want 0", got)
	}
	firstData, ok := firstEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("list replies first data should be object")
	}
	firstItems, ok := firstData["items"].([]interface{})
	if !ok || len(firstItems) != 2 {
		t.Fatalf("first replies page size = %d, want 2", len(firstItems))
	}
	if hasMore, ok := firstData["hasMore"].(bool); !ok || !hasMore {
		t.Fatalf("first replies page hasMore should be true")
	}
	nextCursor, ok := firstData["nextCursor"].(string)
	if !ok || nextCursor == "" {
		t.Fatalf("first replies page nextCursor should be non-empty")
	}
	firstReplyItem, ok := firstItems[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first reply item should be object")
	}
	if got := asInt(t, firstReplyItem["rootId"]); got != rootID {
		t.Fatalf("first reply rootId = %d, want %d", got, rootID)
	}
	replyToUser, ok := firstReplyItem["replyToUser"].(map[string]interface{})
	if !ok {
		t.Fatalf("replyToUser should be object")
	}
	if got := asInt(t, replyToUser["id"]); got != authorID {
		t.Fatalf("replyToUser id = %d, want %d", got, authorID)
	}
	firstReplyID := asInt(t, firstReplyItem["id"])
	likeResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/comments/"+strconv.Itoa(firstReplyID)+"/like", map[string]interface{}{}, accessToken)
	if likeResp.Code != http.StatusOK {
		t.Fatalf("like reply status = %d, want 200", likeResp.Code)
	}

	authedAfterLike := requestWithAuth(t, engine, http.MethodGet, "/api/v1/comments/"+strconv.Itoa(rootID)+"/replies?limit=2", accessToken)
	if authedAfterLike.Code != http.StatusOK {
		t.Fatalf("authed replies after like status = %d, want 200", authedAfterLike.Code)
	}
	authedAfterLikeEnvelope := decodeEnvelope(t, authedAfterLike.Body.Bytes())
	authedAfterLikeData, ok := authedAfterLikeEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("authed replies after like data should be object")
	}
	authedAfterLikeItems, ok := authedAfterLikeData["items"].([]interface{})
	if !ok || len(authedAfterLikeItems) == 0 {
		t.Fatalf("authed replies after like items should not be empty")
	}
	likedFound := false
	for _, raw := range authedAfterLikeItems {
		item, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("authed replies item should be object")
		}
		if asInt(t, item["id"]) == firstReplyID {
			if likedByMe, ok := item["likedByMe"].(bool); !ok || !likedByMe {
				t.Fatalf("liked reply likedByMe should be true")
			}
			likedFound = true
		}
	}
	if !likedFound {
		t.Fatalf("liked reply should appear in first page")
	}

	guestAfterLike := requestNoBody(t, engine, http.MethodGet, "/api/v1/comments/"+strconv.Itoa(rootID)+"/replies?limit=2")
	if guestAfterLike.Code != http.StatusOK {
		t.Fatalf("guest replies after like status = %d, want 200", guestAfterLike.Code)
	}
	guestAfterLikeEnvelope := decodeEnvelope(t, guestAfterLike.Body.Bytes())
	guestAfterLikeData, ok := guestAfterLikeEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("guest replies after like data should be object")
	}
	guestAfterLikeItems, ok := guestAfterLikeData["items"].([]interface{})
	if !ok || len(guestAfterLikeItems) == 0 {
		t.Fatalf("guest replies after like items should not be empty")
	}
	for _, raw := range guestAfterLikeItems {
		item, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("guest replies item should be object")
		}
		if asInt(t, item["id"]) == firstReplyID {
			if likedByMe, ok := item["likedByMe"].(bool); !ok || likedByMe {
				t.Fatalf("guest likedByMe should be false")
			}
		}
	}

	secondResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/comments/"+strconv.Itoa(rootID)+"/replies?limit=2&cursor="+url.QueryEscape(nextCursor))
	if secondResp.Code != http.StatusOK {
		t.Fatalf("list replies second page status = %d, want 200", secondResp.Code)
	}
	secondEnvelope := decodeEnvelope(t, secondResp.Body.Bytes())
	secondData, ok := secondEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("list replies second data should be object")
	}
	secondItems, ok := secondData["items"].([]interface{})
	if !ok || len(secondItems) == 0 {
		t.Fatalf("second replies page should contain items")
	}
	if hasMore, ok := secondData["hasMore"].(bool); !ok || hasMore {
		t.Fatalf("second replies page hasMore should be false")
	}

	badCursorResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/comments/"+strconv.Itoa(rootID)+"/replies?cursor=bad-cursor")
	if badCursorResp.Code != http.StatusBadRequest {
		t.Fatalf("bad replies cursor status = %d, want 400", badCursorResp.Code)
	}
	badCursorEnvelope := decodeEnvelope(t, badCursorResp.Body.Bytes())
	if got := asInt(t, badCursorEnvelope["code"]); got != 40001 {
		t.Fatalf("bad replies cursor code = %d, want 40001", got)
	}

	notFoundResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/comments/999999/replies")
	if notFoundResp.Code != http.StatusNotFound {
		t.Fatalf("missing root replies status = %d, want 404", notFoundResp.Code)
	}
	notFoundEnvelope := decodeEnvelope(t, notFoundResp.Body.Bytes())
	if got := asInt(t, notFoundEnvelope["code"]); got != 40401 {
		t.Fatalf("missing root replies code = %d, want 40401", got)
	}

	invalidIDResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/comments/abc/replies")
	if invalidIDResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid root id status = %d, want 400", invalidIDResp.Code)
	}
	invalidIDEnvelope := decodeEnvelope(t, invalidIDResp.Body.Bytes())
	if got := asInt(t, invalidIDEnvelope["code"]); got != 40001 {
		t.Fatalf("invalid root id code = %d, want 40001", got)
	}
}

func TestMeEndpoints(t *testing.T) {
	engine := newTestEngine("test-secret-12345678901234567890")

	_ = requestJSON(t, engine, http.MethodPost, "/api/v1/auth/register", map[string]interface{}{
		"email":    "me@example.com",
		"nickname": "MeUser",
		"password": "Pass@123456",
	})
	loginResp := requestJSON(t, engine, http.MethodPost, "/api/v1/auth/login", map[string]interface{}{
		"email":    "me@example.com",
		"password": "Pass@123456",
	})
	if loginResp.Code != http.StatusOK {
		t.Fatalf("me login status = %d, want 200", loginResp.Code)
	}
	loginEnvelope := decodeEnvelope(t, loginResp.Body.Bytes())
	loginData, ok := loginEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("me login data should be object")
	}
	accessToken, ok := loginData["accessToken"].(string)
	if !ok || accessToken == "" {
		t.Fatalf("me access token should not be empty")
	}

	for i := 0; i < 3; i++ {
		createResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/101/comments", map[string]interface{}{
			"content":       "my comment #" + strconv.Itoa(i),
			"rootId":        0,
			"parentId":      0,
			"replyToUserId": 0,
		}, accessToken)
		if createResp.Code != http.StatusOK {
			t.Fatalf("create my comment[%d] status = %d, want 200", i, createResp.Code)
		}
	}

	myCommentLikeResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/comments/9001/like", map[string]interface{}{}, accessToken)
	if myCommentLikeResp.Code != http.StatusOK {
		t.Fatalf("like existing my comment status = %d, want 200", myCommentLikeResp.Code)
	}

	firstCommentPage := requestWithAuth(t, engine, http.MethodGet, "/api/v1/me/comments?limit=2", accessToken)
	if firstCommentPage.Code != http.StatusOK {
		t.Fatalf("my comments first page status = %d, want 200", firstCommentPage.Code)
	}
	firstCommentEnvelope := decodeEnvelope(t, firstCommentPage.Body.Bytes())
	if got := asInt(t, firstCommentEnvelope["code"]); got != 0 {
		t.Fatalf("my comments first page code = %d, want 0", got)
	}
	firstCommentData, ok := firstCommentEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("my comments first page data should be object")
	}
	firstCommentItems, ok := firstCommentData["items"].([]interface{})
	if !ok || len(firstCommentItems) != 2 {
		t.Fatalf("my comments first page len = %d, want 2", len(firstCommentItems))
	}
	fullCommentPage := requestWithAuth(t, engine, http.MethodGet, "/api/v1/me/comments?limit=20", accessToken)
	if fullCommentPage.Code != http.StatusOK {
		t.Fatalf("my comments full page status = %d, want 200", fullCommentPage.Code)
	}
	fullCommentEnvelope := decodeEnvelope(t, fullCommentPage.Body.Bytes())
	fullCommentData, ok := fullCommentEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("my comments full page data should be object")
	}
	fullCommentItems, ok := fullCommentData["items"].([]interface{})
	if !ok || len(fullCommentItems) == 0 {
		t.Fatalf("my comments full page should contain items")
	}
	likedFound := false
	for _, raw := range fullCommentItems {
		item, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("my comments full page item should be object")
		}
		id := asInt(t, item["id"])
		likedByMe, ok := item["likedByMe"].(bool)
		if !ok {
			t.Fatalf("my comments full page likedByMe should be bool")
		}
		if id == 9001 {
			likedFound = true
			if !likedByMe {
				t.Fatalf("my comments liked item likedByMe should be true")
			}
		}
	}
	if !likedFound {
		t.Fatalf("my comments should contain liked seed comment")
	}
	if hasMore, ok := firstCommentData["hasMore"].(bool); !ok || !hasMore {
		t.Fatalf("my comments first page hasMore should be true")
	}
	nextCommentCursor, ok := firstCommentData["nextCursor"].(string)
	if !ok || nextCommentCursor == "" {
		t.Fatalf("my comments first page nextCursor should be non-empty")
	}

	secondCommentPage := requestWithAuth(t, engine, http.MethodGet, "/api/v1/me/comments?limit=2&cursor="+url.QueryEscape(nextCommentCursor), accessToken)
	if secondCommentPage.Code != http.StatusOK {
		t.Fatalf("my comments second page status = %d, want 200", secondCommentPage.Code)
	}
	secondCommentEnvelope := decodeEnvelope(t, secondCommentPage.Body.Bytes())
	secondCommentData, ok := secondCommentEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("my comments second page data should be object")
	}
	secondCommentItems, ok := secondCommentData["items"].([]interface{})
	if !ok || len(secondCommentItems) == 0 {
		t.Fatalf("my comments second page should contain items")
	}
	if hasMore, ok := secondCommentData["hasMore"].(bool); !ok || hasMore {
		t.Fatalf("my comments second page hasMore should be false")
	}

	for _, payload := range []struct {
		score int
		stall int
	}{{score: 5, stall: 101}, {score: 3, stall: 102}, {score: 4, stall: 201}} {
		rateResp := requestJSONWithAuth(t, engine, http.MethodPost, "/api/v1/stalls/"+strconv.Itoa(payload.stall)+"/ratings", map[string]interface{}{"score": payload.score}, accessToken)
		if rateResp.Code != http.StatusOK {
			t.Fatalf("rate stall %d status = %d, want 200", payload.stall, rateResp.Code)
		}
	}

	firstRatingPage := requestWithAuth(t, engine, http.MethodGet, "/api/v1/me/ratings?limit=2", accessToken)
	if firstRatingPage.Code != http.StatusOK {
		t.Fatalf("my ratings first page status = %d, want 200", firstRatingPage.Code)
	}
	firstRatingEnvelope := decodeEnvelope(t, firstRatingPage.Body.Bytes())
	if got := asInt(t, firstRatingEnvelope["code"]); got != 0 {
		t.Fatalf("my ratings first page code = %d, want 0", got)
	}
	firstRatingData, ok := firstRatingEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("my ratings first page data should be object")
	}
	firstRatingItems, ok := firstRatingData["items"].([]interface{})
	if !ok || len(firstRatingItems) != 2 {
		t.Fatalf("my ratings first page len = %d, want 2", len(firstRatingItems))
	}
	for _, raw := range firstRatingItems {
		item, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("my ratings item should be object")
		}
		if asString(t, item["stallName"]) == "" {
			t.Fatalf("my ratings item stallName should not be empty")
		}
		if asString(t, item["updatedAt"]) == "" {
			t.Fatalf("my ratings item updatedAt should not be empty")
		}
	}
	if hasMore, ok := firstRatingData["hasMore"].(bool); !ok || !hasMore {
		t.Fatalf("my ratings first page hasMore should be true")
	}
	nextRatingCursor, ok := firstRatingData["nextCursor"].(string)
	if !ok || nextRatingCursor == "" {
		t.Fatalf("my ratings first page nextCursor should be non-empty")
	}

	secondRatingPage := requestWithAuth(t, engine, http.MethodGet, "/api/v1/me/ratings?limit=2&cursor="+url.QueryEscape(nextRatingCursor), accessToken)
	if secondRatingPage.Code != http.StatusOK {
		t.Fatalf("my ratings second page status = %d, want 200", secondRatingPage.Code)
	}
	secondRatingEnvelope := decodeEnvelope(t, secondRatingPage.Body.Bytes())
	secondRatingData, ok := secondRatingEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("my ratings second page data should be object")
	}
	secondRatingItems, ok := secondRatingData["items"].([]interface{})
	if !ok || len(secondRatingItems) != 1 {
		t.Fatalf("my ratings second page len = %d, want 1", len(secondRatingItems))
	}
	if hasMore, ok := secondRatingData["hasMore"].(bool); !ok || hasMore {
		t.Fatalf("my ratings second page hasMore should be false")
	}

	guestMyComments := requestNoBody(t, engine, http.MethodGet, "/api/v1/me/comments")
	if guestMyComments.Code != http.StatusUnauthorized {
		t.Fatalf("guest my comments status = %d, want 401", guestMyComments.Code)
	}
	guestMyCommentsEnvelope := decodeEnvelope(t, guestMyComments.Body.Bytes())
	if got := asInt(t, guestMyCommentsEnvelope["code"]); got != 40101 {
		t.Fatalf("guest my comments code = %d, want 40101", got)
	}

	guestMyRatings := requestNoBody(t, engine, http.MethodGet, "/api/v1/me/ratings")
	if guestMyRatings.Code != http.StatusUnauthorized {
		t.Fatalf("guest my ratings status = %d, want 401", guestMyRatings.Code)
	}
	guestMyRatingsEnvelope := decodeEnvelope(t, guestMyRatings.Body.Bytes())
	if got := asInt(t, guestMyRatingsEnvelope["code"]); got != 40101 {
		t.Fatalf("guest my ratings code = %d, want 40101", got)
	}
}

func TestRankingEndpoints(t *testing.T) {
	engine := newTestEngine("test-secret-12345678901234567890")

	resp := requestNoBody(t, engine, http.MethodGet, "/api/v1/rankings?scope=global&scopeId=0&foodTypeId=0&days=30&sort=score_desc&limit=2")
	if resp.Code != http.StatusOK {
		t.Fatalf("ranking first page status = %d, want 200", resp.Code)
	}
	envelope := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, envelope["code"]); got != 0 {
		t.Fatalf("ranking first page code = %d, want 0", got)
	}
	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("ranking first page data should be object")
	}
	items, ok := data["items"].([]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("ranking first page len = %d, want 2", len(items))
	}
	firstItem, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatalf("ranking first item should be object")
	}
	secondItem, ok := items[1].(map[string]interface{})
	if !ok {
		t.Fatalf("ranking second item should be object")
	}
	if asInt(t, firstItem["rank"]) != 1 || asInt(t, secondItem["rank"]) != 2 {
		t.Fatalf("ranking rank should be 1,2 on first page")
	}
	if asFloat(t, firstItem["avgRating"]) < asFloat(t, secondItem["avgRating"]) {
		t.Fatalf("ranking should be score_desc on first page")
	}
	if hasMore, ok := data["hasMore"].(bool); !ok || !hasMore {
		t.Fatalf("ranking first page hasMore should be true")
	}
	nextCursor, ok := data["nextCursor"].(string)
	if !ok || nextCursor == "" {
		t.Fatalf("ranking first page nextCursor should be non-empty")
	}

	resp = requestNoBody(t, engine, http.MethodGet, "/api/v1/rankings?scope=global&scopeId=0&foodTypeId=0&days=30&sort=score_desc&limit=2&cursor="+url.QueryEscape(nextCursor))
	if resp.Code != http.StatusOK {
		t.Fatalf("ranking second page status = %d, want 200", resp.Code)
	}
	secondEnvelope := decodeEnvelope(t, resp.Body.Bytes())
	if got := asInt(t, secondEnvelope["code"]); got != 0 {
		t.Fatalf("ranking second page code = %d, want 0", got)
	}
	secondData, ok := secondEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("ranking second page data should be object")
	}
	secondItems, ok := secondData["items"].([]interface{})
	if !ok || len(secondItems) == 0 {
		t.Fatalf("ranking second page should have items")
	}
	secondFirst, ok := secondItems[0].(map[string]interface{})
	if !ok {
		t.Fatalf("ranking second page item should be object")
	}
	if asInt(t, secondFirst["stallId"]) == asInt(t, firstItem["stallId"]) || asInt(t, secondFirst["stallId"]) == asInt(t, secondItem["stallId"]) {
		t.Fatalf("ranking cursor page should not duplicate first page items")
	}

	hotResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/rankings?scope=global&scopeId=0&foodTypeId=0&days=30&sort=hot_desc&limit=20")
	if hotResp.Code != http.StatusOK {
		t.Fatalf("ranking hot list status = %d, want 200", hotResp.Code)
	}
	hotEnvelope := decodeEnvelope(t, hotResp.Body.Bytes())
	hotData, ok := hotEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("ranking hot data should be object")
	}
	hotItems, ok := hotData["items"].([]interface{})
	if !ok || len(hotItems) < 2 {
		t.Fatalf("ranking hot items should be >= 2")
	}
	hotFirst, ok := hotItems[0].(map[string]interface{})
	if !ok {
		t.Fatalf("ranking hot first item should be object")
	}
	hotSecond, ok := hotItems[1].(map[string]interface{})
	if !ok {
		t.Fatalf("ranking hot second item should be object")
	}
	if asFloat(t, hotFirst["hotScore"]) < asFloat(t, hotSecond["hotScore"]) {
		t.Fatalf("ranking should be hot_desc")
	}

	canteenResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/rankings?scope=canteen&scopeId=1&days=30&sort=score_desc&limit=20")
	if canteenResp.Code != http.StatusOK {
		t.Fatalf("ranking canteen scope status = %d, want 200", canteenResp.Code)
	}
	canteenEnvelope := decodeEnvelope(t, canteenResp.Body.Bytes())
	canteenData, ok := canteenEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("ranking canteen data should be object")
	}
	canteenItems, ok := canteenData["items"].([]interface{})
	if !ok || len(canteenItems) == 0 {
		t.Fatalf("ranking canteen items should not be empty")
	}
	for _, raw := range canteenItems {
		item, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("ranking canteen item should be object")
		}
		if asInt(t, item["canteenId"]) != 1 {
			t.Fatalf("ranking canteen scope should only return canteenId=1")
		}
	}

	foodTypeResp := requestNoBody(t, engine, http.MethodGet, "/api/v1/rankings?scope=global&scopeId=0&foodTypeId=2&days=30&sort=score_desc&limit=20")
	if foodTypeResp.Code != http.StatusOK {
		t.Fatalf("ranking foodType filter status = %d, want 200", foodTypeResp.Code)
	}
	foodTypeEnvelope := decodeEnvelope(t, foodTypeResp.Body.Bytes())
	foodTypeData, ok := foodTypeEnvelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("ranking foodType data should be object")
	}
	foodTypeItems, ok := foodTypeData["items"].([]interface{})
	if !ok || len(foodTypeItems) == 0 {
		t.Fatalf("ranking foodType items should not be empty")
	}
	for _, raw := range foodTypeItems {
		item, ok := raw.(map[string]interface{})
		if !ok {
			t.Fatalf("ranking foodType item should be object")
		}
		if asInt(t, item["foodTypeId"]) != 2 {
			t.Fatalf("ranking foodType filter should only return foodTypeId=2")
		}
	}

	badCases := []string{
		"/api/v1/rankings?scope=unknown&scopeId=0&days=30&sort=score_desc",
		"/api/v1/rankings?scope=canteen&scopeId=0&days=30&sort=score_desc",
		"/api/v1/rankings?scope=global&scopeId=1&days=30&sort=score_desc",
		"/api/v1/rankings?scope=global&scopeId=0&days=15&sort=score_desc",
		"/api/v1/rankings?scope=global&scopeId=0&days=30&sort=latest",
		"/api/v1/rankings?scope=global&scopeId=0&days=30&sort=score_desc&cursor=bad-cursor",
		"/api/v1/rankings?scope=global&scopeId=0&days=30&sort=score_desc&limit=abc",
	}
	for _, path := range badCases {
		badResp := requestNoBody(t, engine, http.MethodGet, path)
		if badResp.Code != http.StatusBadRequest {
			t.Fatalf("ranking bad case %s status = %d, want 400", path, badResp.Code)
		}
		badEnvelope := decodeEnvelope(t, badResp.Body.Bytes())
		if got := asInt(t, badEnvelope["code"]); got != 40001 {
			t.Fatalf("ranking bad case %s code = %d, want 40001", path, got)
		}
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

func TestSwaggerRoutes(t *testing.T) {
	engine := newTestEngine("test-secret-12345678901234567890")

	swaggerResp := requestNoBody(t, engine, http.MethodGet, "/swagger")
	if swaggerResp.Code != http.StatusOK {
		t.Fatalf("swagger page status = %d, want 200", swaggerResp.Code)
	}
	if !strings.Contains(swaggerResp.Body.String(), "/swagger/doc.json") {
		t.Fatalf("swagger page should contain local doc link")
	}

	docResp := requestNoBody(t, engine, http.MethodGet, "/swagger/doc.json")
	if docResp.Code != http.StatusOK {
		t.Fatalf("swagger doc status = %d, want 200", docResp.Code)
	}
	if !strings.Contains(docResp.Body.String(), "\"openapi\": \"3.0.3\"") {
		t.Fatalf("swagger doc should contain openapi version")
	}
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

func requestJSONWithAuth(t *testing.T, handler http.Handler, method string, path string, body map[string]interface{}, accessToken string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
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

func asFloat(t *testing.T, v interface{}) float64 {
	t.Helper()
	f, ok := v.(float64)
	if !ok {
		t.Fatalf("value should be float64 in json map")
	}
	return f
}

func asString(t *testing.T, v interface{}) string {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("value should be string in json map")
	}
	return s
}

func assertFloatNear(t *testing.T, got float64, want float64, tolerance float64) {
	t.Helper()
	delta := got - want
	if delta < 0 {
		delta = -delta
	}
	if delta > tolerance {
		t.Fatalf("float mismatch: got %.12f want %.12f (tol %.12f)", got, want, tolerance)
	}
}
