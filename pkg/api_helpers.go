package pkg

import (
	"github.com/go-chi/render"
	"net/http"
)

type response struct {
	Message interface{} `json:"message,omitempty"`
	Err     interface{} `json:"error,omitempty"`
}

func Render(w http.ResponseWriter, r *http.Request, res interface{}) {
	switch res.(type) {
	case render.Renderer:
		w.WriteHeader(http.StatusOK)
		render.JSON(w, r, response{Message: res.(render.Renderer), Err: ""})
	case error:
		w.WriteHeader(http.StatusInternalServerError)
		render.JSON(w, r, response{Message: "", Err: res.(error).Error()})
	default:
		w.WriteHeader(http.StatusOK)
		render.JSON(w, r, response{Message: res, Err: ""})
	}
}
