// Package httpx provides HTTP response helpers.
package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorBody is the standard API error payload.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Envelope is the standard API response wrapper.
type Envelope struct {
	Data  any            `json:"data,omitempty"`
	Error *ErrorBody     `json:"error,omitempty"`
	Meta  map[string]any `json:"meta,omitempty"`
}

// WriteJSON writes a successful JSON envelope.
func WriteJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(Envelope{Data: data}); err != nil {
		return fmt.Errorf("encode json response: %w", err)
	}

	return nil
}

// WriteError writes an error JSON envelope.
func WriteError(w http.ResponseWriter, status int, code, message string) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(Envelope{
		Error: &ErrorBody{Code: code, Message: message},
	}); err != nil {
		return fmt.Errorf("encode json error response: %w", err)
	}

	return nil
}

// DecodeJSON decodes a JSON request body and closes it.
func DecodeJSON(r *http.Request, dst any) (err error) {
	defer func() {
		closeErr := r.Body.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("close request body: %w", closeErr)
		}
	}()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if decodeErr := dec.Decode(dst); decodeErr != nil {
		return fmt.Errorf("decode json body: %w", decodeErr)
	}

	return nil
}
