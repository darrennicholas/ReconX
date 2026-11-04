package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

type Envelope struct {
	Success   bool   `json:"success"`
	Data      any    `json:"data,omitempty"`
	Meta      any    `json:"meta,omitempty"`
	Error     any    `json:"error,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func JSON(w http.ResponseWriter, r *http.Request, status int, data any, meta any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Success: status < 400, Data: data, Meta: meta, RequestID: r.Header.Get("X-Request-ID")})
}

func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Success: false, Error: map[string]string{"code": code, "message": message}, RequestID: r.Header.Get("X-Request-ID")})
}

func Failure(w http.ResponseWriter, r *http.Request, status int, code, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Success: false, Data: data, Error: map[string]string{"code": code, "message": message}, RequestID: r.Header.Get("X-Request-ID")})
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, maxBytes int64, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
