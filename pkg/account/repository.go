package account

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/jmoiron/sqlx"
)

var (
	_ Repository = repository{} // Verify that repository implements Repository.
)

type Repository interface {
	FindByUsername(string) (*Account, error)
}

type repository struct {
	db *sqlx.DB
	rd *redis.Client
}

func NewRepository(db *sqlx.DB, rd *redis.Client) Repository {
	return &repository{db: db, rd: rd}
}

func (r repository) FindByUsername(username string) (*Account, error) {
	var s Account

	fmt.Println(username)
	err := r.db.Get(&s, `SELECT * FROM account WHERE username = $1`, username)
	if err != nil {
		return nil, err
	}

	return &s, nil
}
