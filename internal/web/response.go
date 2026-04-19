package web

import (
	"encoding/json"
	"net/http"
)

type responseBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func writeSuccess(w http.ResponseWriter, message string, data any) {
	writeJSON(w, http.StatusOK, responseBody{
		Code:    0,
		Message: message,
		Data:    data,
	})
}

func writeFail(w http.ResponseWriter, code int, message string) {
	if code == 0 {
		code = errCodeUnknown
	}
	writeJSON(w, http.StatusOK, responseBody{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
