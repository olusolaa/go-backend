package inbounds

import (
	"context"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/jmoiron/sqlx"
	"github.com/olusolaa/go-backend/pkg"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"time"
)

var (
	_ Repository = repository{} // Verify that repository implements Repository.
)

type Repository interface {
	post(ctx context.Context, req pkg.PostReq, accountId int64) error
}

type repository struct {
	db *sqlx.DB
	rd *redis.Client
}

func NewRepository(db *sqlx.DB, rd *redis.Client) Repository {
	return &repository{db: db, rd: rd}
}

func (r repository) post(ctx context.Context, req pkg.PostReq, accountId int64) error {
	var count int
	if err := r.db.GetContext(ctx, &count, "SELECT count(id) FROM phone_number WHERE account_id = $1 AND number = $2", accountId, req.To); err != nil {
		return err
	}

	if count <= 0 {
		return errors.New("to parameter not found")
	}

	if req.Text == "stop" {
		log.Info("STOP command received")
		err := r.rd.Set(fmt.Sprintf("%s:%s", req.From, req.To),
			fmt.Sprintf("%s:%s", req.To, req.From), time.Hour*4).Err()
		if err != nil {
			log.Info("error %s", err)
		}
	}

	return nil
}
