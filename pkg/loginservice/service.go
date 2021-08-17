package loginservice

import (
	"context"
	"loginsvc/repo"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/metrics"
)

type Service interface {
	Name(ctx context.Context, N string) (string, error)
}

func New(logger log.Logger, ints, chars metrics.Counter) Service {
	var svc Service
	{
		svc = NewBasicService()
		svc = LoggingMiddleware(logger)(svc)
		svc = InstrumentingMiddleware(ints, chars)(svc)
	}
	return svc
}

// NewBasicService returns a naïve, stateless implementation of Service.
func NewBasicService() Service {
	return basicService{
		// repo: repo.GetSqliteLoginRepository(),
		repo: repo.GetMySQLLoginRepo(),
	}
}

type basicService struct {
	repo repo.LoginRepository
}

func (s basicService) Name(c context.Context, n string) (string, error) {
	sid, err := s.repo.Name(n)
	if err != nil {
		return "", err
	}
	return sid, nil
}
