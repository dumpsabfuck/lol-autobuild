package autobuild

import (
	"context"
	"errors"

	"github.com/controlado/lol-autobuild/internal/autobuild/ports"
)

type ServiceDeps struct {
	Coachless ports.CoachlessClient
	Tokens    ports.TokenProvider
	LCU       ports.LCUClient
}

type syncService struct {
	deps ServiceDeps
}

func NewService(deps ServiceDeps) (Service, error) {
	if deps.Coachless == nil {
		return nil, errors.New("coachless client is required")
	}
	if deps.Tokens == nil {
		return nil, errors.New("token provider is required")
	}
	if deps.LCU == nil {
		return nil, errors.New("lcu client is required")
	}

	return &syncService{deps: deps}, nil
}

func (s *syncService) EnsureCoachlessAuth(ctx context.Context) error {
	if _, err := s.deps.Tokens.AccessToken(ctx); err != nil {
		return err
	}

	return nil
}
