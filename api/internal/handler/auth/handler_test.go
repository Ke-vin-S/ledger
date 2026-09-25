package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockTokenServer returns a test server that simulates the Google token endpoint.
// It verifies the POSTed form fields and responds with the given access token.
func mockTokenServer(t *testing.T, wantCode, accessToken string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.FormValue("code") != wantCode || r.FormValue("grant_type") != "authorization_code" {
			http.Error(w, "unexpected form values", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"access_token": accessToken})
	}))
}

// mockUserInfoServer returns a test server that simulates the Google userinfo endpoint.
// It verifies the Bearer token and responds with the given user info.
func mockUserInfoServer(t *testing.T, wantBearer string, info googleUserInfo) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+wantBearer {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
}

func overrideEndpoints(t *testing.T, tokenURL, userInfoURL string) {
	t.Helper()
	origToken, origUserInfo := googleTokenEndpoint, googleUserInfoEndpoint
	googleTokenEndpoint, googleUserInfoEndpoint = tokenURL, userInfoURL
	t.Cleanup(func() {
		googleTokenEndpoint = origToken
		googleUserInfoEndpoint = origUserInfo
	})
}

func TestExchangeGoogleCode_Success(t *testing.T) {
	want := googleUserInfo{Sub: "12345", Email: "test@example.com", EmailVerified: true, Name: "Test User", Picture: "https://example.com/pic.jpg"}

	tokenSrv := mockTokenServer(t, "auth-code-abc", "tok-xyz")
	defer tokenSrv.Close()
	infoSrv := mockUserInfoServer(t, "tok-xyz", want)
	defer infoSrv.Close()

	overrideEndpoints(t, tokenSrv.URL, infoSrv.URL)

	got, err := exchangeGoogleCode(context.Background(), "auth-code-abc", "client-id", "client-secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Sub != want.Sub || got.Email != want.Email {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestExchangeGoogleCode_TokenEndpointError(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
	}))
	defer tokenSrv.Close()

	overrideEndpoints(t, tokenSrv.URL, "http://unused")

	_, err := exchangeGoogleCode(context.Background(), "bad-code", "cid", "csecret")
	if err == nil {
		t.Fatal("expected error for token exchange failure, got nil")
	}
}

func TestExchangeGoogleCode_NoAccessToken(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{}) // empty — no access_token
	}))
	defer tokenSrv.Close()

	overrideEndpoints(t, tokenSrv.URL, "http://unused")

	_, err := exchangeGoogleCode(context.Background(), "code", "cid", "csecret")
	if err == nil {
		t.Fatal("expected error for missing access_token, got nil")
	}
}

func TestExchangeGoogleCode_UserInfoError(t *testing.T) {
	tokenSrv := mockTokenServer(t, "code", "access-token")
	defer tokenSrv.Close()

	infoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer infoSrv.Close()

	overrideEndpoints(t, tokenSrv.URL, infoSrv.URL)

	_, err := exchangeGoogleCode(context.Background(), "code", "cid", "csecret")
	if err == nil {
		t.Fatal("expected error for userinfo failure, got nil")
	}
}

func TestExchangeGoogleCode_MissingSub(t *testing.T) {
	tokenSrv := mockTokenServer(t, "code", "access-token")
	defer tokenSrv.Close()

	infoSrv := mockUserInfoServer(t, "access-token", googleUserInfo{Sub: "", Email: "x@x.com"})
	defer infoSrv.Close()

	overrideEndpoints(t, tokenSrv.URL, infoSrv.URL)

	_, err := exchangeGoogleCode(context.Background(), "code", "cid", "csecret")
	if err == nil {
		t.Fatal("expected error for missing sub, got nil")
	}
}

func TestExchangeGoogleCode_UnverifiedEmailRejected(t *testing.T) {
	tokenSrv := mockTokenServer(t, "code", "access-token")
	defer tokenSrv.Close()
	infoSrv := mockUserInfoServer(t, "access-token", googleUserInfo{Sub: "123", Email: "x@x.com", EmailVerified: false})
	defer infoSrv.Close()
	overrideEndpoints(t, tokenSrv.URL, infoSrv.URL)

	_, err := exchangeGoogleCode(context.Background(), "code", "cid", "csecret")
	if err == nil {
		t.Fatal("expected unverified email to be rejected")
	}
}

func TestRefreshCookiePolicy(t *testing.T) {
	tests := []struct {
		name       string
		isLocal    bool
		wantSame   http.SameSite
		wantSecure bool
	}{
		{name: "production cross-site", isLocal: false, wantSame: http.SameSiteNoneMode, wantSecure: true},
		{name: "local same-site", isLocal: true, wantSame: http.SameSiteStrictMode, wantSecure: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			setRefreshCookie(recorder, "token", tt.isLocal)
			cookies := recorder.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("got %d cookies, want 1", len(cookies))
			}
			cookie := cookies[0]
			if !cookie.HttpOnly || cookie.Secure != tt.wantSecure || cookie.SameSite != tt.wantSame {
				t.Fatalf("cookie policy = HttpOnly:%v Secure:%v SameSite:%v, want HttpOnly:true Secure:%v SameSite:%v", cookie.HttpOnly, cookie.Secure, cookie.SameSite, tt.wantSecure, tt.wantSame)
			}
		})
	}
}

func TestClearRefreshCookieMatchesProductionPolicy(t *testing.T) {
	recorder := httptest.NewRecorder()
	clearRefreshCookie(recorder, false)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.MaxAge >= 0 || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteNoneMode {
		t.Fatalf("clear cookie policy = MaxAge:%d HttpOnly:%v Secure:%v SameSite:%v", cookie.MaxAge, cookie.HttpOnly, cookie.Secure, cookie.SameSite)
	}
}
