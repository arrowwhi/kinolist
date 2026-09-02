package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
)

var jsonUnmarshal = json.Unmarshal

func writeAuthError(w http.ResponseWriter, err error) {
	code := "unauthorized"
	switch {
	case errors.Is(err, ErrMissingInitData):
		code = "missing_init_data"
	case errors.Is(err, ErrExpiredInitData):
		code = "init_data_expired"
	case errors.Is(err, ErrBadSignature):
		code = "invalid_init_data"
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": "Требуется авторизация через Telegram",
	})
}
