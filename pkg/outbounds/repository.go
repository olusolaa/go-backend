package outbounds

import (
	"context"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/jmoiron/sqlx"
	"github.com/olusolaa/go-backend/pkg"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
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

	if _, err := r.rd.Get(fmt.Sprintf("%s:%s", req.From, req.To)).Result(); err == nil {
		log.Infof("sms from %s to %s blocked by STOP request", req.From, req.To)
		return errors.Errorf("sms from %s to %s blocked by STOP request", req.From, req.To)
	}

	if err := r.db.GetContext(ctx, &count, "select count(id) from phone_number where account_id = $1 AND number = $2", accountId, req.From); err != nil {
		return err
	}

	if count <= 0 {
		return errors.New("from parameter not found")
	}
	return nil

}
