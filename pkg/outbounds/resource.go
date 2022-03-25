package outbounds

import (
	"github.com/go-chi/chi"
	"github.com/go-redis/redis"
	"github.com/jmoiron/sqlx"
)

type Resource struct {
	db *sqlx.DB
	rd *redis.Client
}

// NewResource creates and returns a resource.
func NewResource(db *sqlx.DB, rd *redis.Client) *Resource {
	return &Resource{
		db: db,
		rd: rd,
	}
}

// Router ...
func (rs *Resource) Router() *chi.Mux {
	r := chi.NewRouter()

	repo := NewRepository(rs.db, rs.rd)
	svc := NewService(repo)
	hndlr := NewHandler(svc)

	r.Post("/sms", hndlr.post)

	return r
}
