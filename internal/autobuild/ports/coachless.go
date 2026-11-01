package ports

import (
	"context"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type CoachlessClient interface {
	GetPatches(ctx context.Context, accessToken string) ([]domain.PatchInfo, error)
	GetRecommendedBuilds(ctx context.Context, accessToken string, req domain.RecommendedBuildsRequest) (domain.RecommendedBuilds, error)
}
