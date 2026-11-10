package handlers

import (
	"encoding/json"
	"io"
	"net/http"
)

func WriteSuccess(w http.ResponseWriter, status int, data any, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	resp := map[string]any{
		"success": true,
		"data":    data,
	}
	if message != "" {
		resp["message"] = message
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

func DecodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return dec.Decode(dst)
}

