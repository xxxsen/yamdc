package web

import "net/http"

func (a *API) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeSuccess(w, http.StatusOK, "ok", map[string]string{"status": "ok"})
}
