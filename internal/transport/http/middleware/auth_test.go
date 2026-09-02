package middleware_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/arrowwhi/kinolist/internal/transport/http/middleware"
)

const botToken = "123456:TEST-TOKEN"

func secretKey() []byte {
	h := hmac.New(sha256.New, []byte("WebAppData"))
	h.Write([]byte(botToken))
	return h.Sum(nil)
}

// signInitData повторяет то, что делает клиент Telegram, чтобы тест
// проверял настоящую схему подписи, а не наш же парсер.
func signInitData(t *testing.T, values url.Values) string {
	t.Helper()

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(k + "=" + values.Get(k))
	}

	mac := hmac.New(sha256.New, secretKey())
	mac.Write([]byte(b.String()))
	values.Set("hash", hex.EncodeToString(mac.Sum(nil)))

	return values.Encode()
}

func validValues(authDate time.Time) url.Values {
	return url.Values{
		"auth_date": {strconv.FormatInt(authDate.Unix(), 10)},
		"query_id":  {"AAF"},
		"user":      {`{"id":777,"username":"tester","first_name":"Иван","language_code":"ru"}`},
	}
}

func TestParseInitDataAcceptsAGenuineSignature(t *testing.T) {
	t.Parallel()

	now := time.Now()
	raw := signInitData(t, validValues(now))

	identity, err := middleware.ParseInitData(raw, secretKey(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if identity.TelegramID != 777 {
		t.Fatalf("telegram id = %d, want 777", identity.TelegramID)
	}
	if identity.Username != "tester" || identity.LanguageCode != "ru" {
		t.Fatalf("identity not parsed: %+v", identity)
	}
}

func TestParseInitDataRejectsForgeries(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name    string
		raw     func() string
		wantErr error
	}{
		{
			name: "tampered user id",
			raw: func() string {
				raw := signInitData(t, validValues(now))
				return strings.Replace(raw, "777", "778", 1)
			},
			wantErr: middleware.ErrBadSignature,
		},
		{
			name: "hash from a different bot token",
			raw: func() string {
				values := validValues(now)
				other := hmac.New(sha256.New, []byte("WebAppData"))
				other.Write([]byte("999:OTHER-TOKEN"))
				mac := hmac.New(sha256.New, other.Sum(nil))
				mac.Write([]byte("auth_date=" + values.Get("auth_date")))
				values.Set("hash", hex.EncodeToString(mac.Sum(nil)))
				return values.Encode()
			},
			wantErr: middleware.ErrBadSignature,
		},
		{
			name:    "no hash at all",
			raw:     func() string { return validValues(now).Encode() },
			wantErr: middleware.ErrBadSignature,
		},
		{
			name: "expired init data",
			raw: func() string {
				return signInitData(t, validValues(now.Add(-48*time.Hour)))
			},
			wantErr: middleware.ErrExpiredInitData,
		},
		{
			name: "valid signature but no user payload",
			raw: func() string {
				return signInitData(t, url.Values{"auth_date": {strconv.FormatInt(now.Unix(), 10)}})
			},
			wantErr: middleware.ErrBadSignature,
		},
		{
			name:    "garbage input",
			raw:     func() string { return "%%%not-a-query%%%" },
			wantErr: middleware.ErrBadSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := middleware.ParseInitData(tt.raw(), secretKey(), now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
