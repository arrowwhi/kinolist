package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/arrowwhi/kinolist/internal/domain/movie"
	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
)

type statisticsStub struct {
	collectionUseCase
	calls int
	stats movie.Statistics
	err   error
}

func (s *statisticsStub) Statistics(_ context.Context, userID int64) (movie.Statistics, error) {
	if userID != 7 {
		panic("statistics called for wrong user")
	}
	s.calls++
	return s.stats, s.err
}

func statisticsInitData() string {
	date := strconv.FormatInt(time.Now().Unix(), 10)
	rawUser := `{"id":777,"first_name":"Test"}`
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte("test-token"))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte("auth_date=" + date + "\nuser=" + rawUser))
	return url.Values{"auth_date": {date}, "user": {rawUser}, "hash": {hex.EncodeToString(mac.Sum(nil))}}.Encode()
}

func TestStatisticsHTTPScopesUserAndIgnoresListParameters(t *testing.T) {
	for _, stats := range []movie.Statistics{{}, {TotalCount: 42, WantToWatchCount: 24, WatchedCount: 18}} {
		uc := &statisticsStub{stats: stats}
		handler := NewMovie(uc)
		authenticated := middleware.TelegramAuth("test-token", func(context.Context, middleware.Identity) (int64, error) { return 7, nil })(http.HandlerFunc(handler.Statistics))
		r := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/statistics?user_id=999&media_type=tv&status=watched&genre_ids=18&q=no-match&limit=1", nil)
		r.Header.Set("X-Telegram-Init-Data", statisticsInitData())
		w := httptest.NewRecorder()
		authenticated.ServeHTTP(w, r)
		var got map[string]int
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || uc.calls != 1 || len(got) != 3 || got["total_count"] != stats.TotalCount || got["want_to_watch_count"] != stats.WantToWatchCount || got["watched_count"] != stats.WatchedCount {
			t.Fatalf("response: %d %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("personal statistics must not be cached")
		}
	}
}

func TestStatisticsHTTPRequiresAuth(t *testing.T) {
	uc := &statisticsStub{}
	handler := NewMovie(uc)
	w := httptest.NewRecorder()
	handler.Statistics(w, httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/statistics", nil))
	if w.Code != 401 || uc.calls != 0 {
		t.Fatalf("unauthenticated response: %d, calls %d", w.Code, uc.calls)
	}
}

func TestStatisticsHTTPHidesDatabaseErrors(t *testing.T) {
	uc := &statisticsStub{err: errors.New("private database error")}
	handler := NewMovie(uc)
	authenticated := middleware.TelegramAuth("test-token", func(context.Context, middleware.Identity) (int64, error) { return 7, nil })(http.HandlerFunc(handler.Statistics))
	r := httptest.NewRequestWithContext(context.Background(), "GET", "/api/v1/statistics", nil)
	r.Header.Set("X-Telegram-Init-Data", statisticsInitData())
	w := httptest.NewRecorder()
	authenticated.ServeHTTP(w, r)
	var response struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if w.Code != 500 || response.Code != "internal" || response.Message == uc.err.Error() {
		t.Fatalf("response: %d %s", w.Code, w.Body.String())
	}
}
