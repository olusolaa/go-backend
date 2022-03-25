package pkg

import (
	"fmt"
	"github.com/go-chi/render"
	"github.com/gobuffalo/validate"
	"github.com/gobuffalo/validate/validators"
	"net/http"
	"strings"
)

type PostReq struct {
	From string `json:"from" min:"6" max:"16"`
	To   string `json:"to" min:"6" max:"16"`
	Text string `json:"text" min:"1" max:"160"`
}

func (v *PostReq) Bind(r *http.Request) error {
	err1 := validate.Validate(
		&validators.StringIsPresent{Name: "from", Field: v.From, Message: fmt.Sprintf("%s is missing", "from")},
		&validators.StringIsPresent{Name: "to", Field: v.To, Message: fmt.Sprintf("%s is missing", "to")},
		&validators.StringIsPresent{Name: "text", Field: v.Text, Message: fmt.Sprintf("%s is missing", "text")},
		&validators.StringLengthInRange{Name: "from", Field: v.From, Min: 6, Max: 16, Message: fmt.Sprintf("%s is invalid", "from")},
		&validators.StringLengthInRange{Name: "to", Field: v.To, Min: 6, Max: 16, Message: fmt.Sprintf("%s is invalid", "to")},
		&validators.StringLengthInRange{Name: "text", Field: v.Text, Min: 1, Max: 160, Message: fmt.Sprintf("%s is invalid", "text")},
	)

	v.Text = strings.TrimSpace(strings.ToLower(v.Text))
	if err1.HasAny() {
		return err1
	}
	return nil
}

var postRequest PostReq

func DecodePostRequest() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req PostReq
			if err := render.Bind(r, &req); err != nil {
				Render(w, r, err)
				return
			}
			postRequest = req
			next.ServeHTTP(w, r)
		})
	}
}

func GetDecodedPostRequest() PostReq {
	return postRequest
}
