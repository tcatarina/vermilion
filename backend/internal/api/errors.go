package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ErrorCode string

const (
	CodeInternal       ErrorCode = "INTERNAL"
	CodeInvalidRequest ErrorCode = "INVALID_REQUEST"
	CodeNotFound       ErrorCode = "NOT_FOUND"
	CodeUnauthorized   ErrorCode = "UNAUTHORIZED"
	CodeNotImplemented ErrorCode = "NOT_IMPLEMENTED"
	CodeUnavailable    ErrorCode = "UNAVAILABLE"
)

type APIError struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type errorEnvelope struct {
	Error APIError `json:"error"`
}

func WriteError(w http.ResponseWriter, status int, e APIError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: e})
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func NotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "resource not found"
	}
	WriteError(w, http.StatusNotFound, APIError{Code: CodeNotFound, Message: message})
}

func BadRequest(w http.ResponseWriter, message string, details map[string]any) {
	WriteError(w, http.StatusBadRequest, APIError{Code: CodeInvalidRequest, Message: message, Details: details})
}

func Unauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = "authentication required"
	}
	WriteError(w, http.StatusUnauthorized, APIError{Code: CodeUnauthorized, Message: message})
}

func Internal(w http.ResponseWriter, _ error) {
	WriteError(w, http.StatusInternalServerError, APIError{
		Code:    CodeInternal,
		Message: "internal server error",
	})
}

func NotImplemented(w http.ResponseWriter, feature string) {
	WriteError(w, http.StatusNotImplemented, APIError{
		Code:    CodeNotImplemented,
		Message: fmt.Sprintf("%s is not implemented yet", feature),
	})
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, out any) bool {
	if r.Body == nil {
		BadRequest(w, "request body is required", nil)
		return false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		BadRequest(w, "invalid JSON body: "+err.Error(), nil)
		return false
	}
	return true
}
