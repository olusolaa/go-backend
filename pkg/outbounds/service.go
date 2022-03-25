package outbounds

import (
	"context"
	middleware2 "github.com/olusolaa/go-backend/middleware"
	"github.com/olusolaa/go-backend/pkg"
)

var _ Service = service{} // Verify that service implements Service.

type Service interface {
	post(context context.Context, req pkg.PostReq) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	svc := &service{
		repo: repo,
	}
	return svc
}

func (s service) post(ctx context.Context, req pkg.PostReq) error {
	accountId := middleware2.GetAuthUserId()
	return s.repo.post(ctx, req, accountId)
}
