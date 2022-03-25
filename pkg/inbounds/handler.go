package inbounds

import (
	"github.com/olusolaa/go-backend/pkg"
	"net/http"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h Handler) post(w http.ResponseWriter, r *http.Request) {

	err := h.svc.post(r.Context(), pkg.GetDecodedPostRequest())
	if err != nil {
		pkg.Render(w, r, err)
		return
	}
	pkg.Render(w, r, "inbound sms ok")
}
