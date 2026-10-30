package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/arrowwhi/kinolist/internal/pkg/logger"
)

type ctxKey struct{}

// Identity — то, что Telegram подтвердил подписью initData.
type Identity struct {
	TelegramID   int64
	Username     string
	FirstName    string
	LanguageCode string
}

var (
	ErrMissingInitData = errors.New("missing telegram init data")
	ErrBadSignature    = errors.New("invalid telegram init data signature")
	ErrExpiredInitData = errors.New("telegram init data expired")
)

// initDataTTL ограничивает окно переигрывания перехваченной подписи.
const initDataTTL = 24 * time.Hour

// TelegramAuth проверяет подпись initData Mini App.
//
// Без этой проверки API открыт всему интернету: telegram_id в заголовке
// подделывается тривиально, а подпись — нет, её знает только владелец
// bot-token.
func TelegramAuth(botToken string, resolve func(context.Context, Identity) (int64, error)) func(http.Handler) http.Handler {
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	secretKey := secret.Sum(nil)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := initDataFrom(r)
			if raw == "" {
				logger.From(r.Context()).Warn("telegram auth rejected", zap.Error(ErrMissingInitData))
				writeAuthError(w, ErrMissingInitData)
				return
			}

			identity, err := ParseInitData(raw, secretKey, time.Now())
			if err != nil {
				logger.From(r.Context()).Warn("telegram auth rejected", zap.Error(err))
				writeAuthError(w, err)
				return
			}

			userID, err := resolve(r.Context(), identity)
			if err != nil {
				// Подпись верна, упал сбор пользователя — без лога это
				// неотличимо от подделки, клиент видит тот же 401.
				logger.From(r.Context()).Error("resolve telegram user failed", zap.Error(err))
				writeAuthError(w, err)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, principal{userID: userID, identity: identity})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type principal struct {
	userID   int64
	identity Identity
}

// UserID достаёт внутренний id пользователя, положенный TelegramAuth.
func UserID(ctx context.Context) (int64, bool) {
	p, ok := ctx.Value(ctxKey{}).(principal)
	return p.userID, ok && p.userID != 0
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	p, ok := ctx.Value(ctxKey{}).(principal)
	return p.identity, ok
}

func initDataFrom(r *http.Request) string {
	if h := r.Header.Get("X-Telegram-Init-Data"); h != "" {
		return h
	}
	// tma-схема из Authorization — второй общепринятый способ передачи.
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "tma ") {
		return strings.TrimPrefix(auth, "tma ")
	}
	return ""
}

// ParseInitData проверяет подпись и срок годности initData и возвращает
// подтверждённые данные пользователя.
func ParseInitData(raw string, secretKey []byte, now time.Time) (Identity, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return Identity{}, ErrBadSignature
	}

	providedHash := values.Get("hash")
	if providedHash == "" {
		return Identity{}, ErrBadSignature
	}
	values.Del("hash")

	// Поле signature (Ed25519 для сторонних проверяющих) клиенты Telegram
	// присылают с Bot API 8.0. Документация включает его в строку для hash,
	// часть реализаций — нет; принимаем оба варианта. Ослаблением это не
	// является: обе строки подписаны тем же секретом бота.
	valid := hashMatches(values, secretKey, providedHash)
	if !valid && values.Has("signature") {
		values.Del("signature")
		valid = hashMatches(values, secretKey, providedHash)
	}
	if !valid {
		return Identity{}, ErrBadSignature
	}

	if authDate := values.Get("auth_date"); authDate != "" {
		ts, err := strconv.ParseInt(authDate, 10, 64)
		if err != nil {
			return Identity{}, ErrBadSignature
		}
		if now.Sub(time.Unix(ts, 0)) > initDataTTL {
			return Identity{}, ErrExpiredInitData
		}
	}

	return parseUserField(values.Get("user"))
}

func hashMatches(values url.Values, secretKey []byte, providedHash string) bool {
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
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(values.Get(k))
	}

	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(b.String()))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(providedHash))
}

func parseUserField(raw string) (Identity, error) {
	if raw == "" {
		return Identity{}, ErrBadSignature
	}

	var u struct {
		ID           int64  `json:"id"`
		Username     string `json:"username"`
		FirstName    string `json:"first_name"`
		LanguageCode string `json:"language_code"`
	}
	if err := jsonUnmarshal([]byte(raw), &u); err != nil || u.ID == 0 {
		return Identity{}, ErrBadSignature
	}

	return Identity{
		TelegramID:   u.ID,
		Username:     u.Username,
		FirstName:    u.FirstName,
		LanguageCode: u.LanguageCode,
	}, nil
}
