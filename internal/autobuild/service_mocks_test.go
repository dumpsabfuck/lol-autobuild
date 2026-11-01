package autobuild

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type tokenProviderStub struct {
	token  string
	claims domain.TokenClaims
	err    error
}

func (t tokenProviderStub) AccessToken(_ context.Context) (string, error) {
	if t.err != nil {
		return "", t.err
	}

	return t.token, nil
}

func (t tokenProviderStub) Refresh(_ context.Context) (domain.TokenPair, error) {
	return domain.TokenPair{AccessToken: t.token, ExpiresAt: time.Now().Add(10 * time.Minute)}, t.err
}

func (t tokenProviderStub) Claims(_ context.Context) (domain.TokenClaims, error) {
	if t.err != nil {
		return domain.TokenClaims{}, t.err
	}

	return t.claims, nil
}

var defaultBuilds = domain.RecommendedBuilds{
	Role: 4,
	Builds: []domain.RecommendedBuild{{
		Tier:    0,
		Winrate:  52.3,
		Runes: &domain.BuildRunes{
			PrimaryTree:    domain.RuneStyleResolve,
			SecondaryTree: domain.RuneStyleSorcery,
			// real payload shape: 6 runes + 3 shards, the 9 perks the LCU requires
			SelectedRunes:  []int{domain.RuneKeystoneGraspOfTheUndying, 8446, 8444, 8451, 8224, 8233},
			SelectedShards: []int{5008, 5008, 5002},
		},
		ItemBlocks: []domain.BuildBlock{
			{Type: domain.BlockStarter, Items: []domain.BuildBlockItem{{ItemID: 1055}}},
		},
	}},
	SpellDuos: []domain.SpellDuo{{SpellIDs: []int{4, 14}}},
}

type coachlessStub struct {
	mu              sync.Mutex
	getPatchesCalls int
	patches         []domain.PatchInfo
	buildsReq       *domain.RecommendedBuildsRequest
	builds          *domain.RecommendedBuilds
	buildsErr       error
}

func (c *coachlessStub) Refresh(_ context.Context, _ string) (domain.TokenPair, error) {
	return domain.TokenPair{}, errors.New("unused")
}

func (c *coachlessStub) GetPatches(_ context.Context, _ string) ([]domain.PatchInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.getPatchesCalls++
	if c.patches != nil {
		return slices.Clone(c.patches), nil
	}

	return []domain.PatchInfo{{Label: "16.7", Major: 16, Patch: 7}}, nil
}

func (c *coachlessStub) GetRecommendedBuilds(_ context.Context, _ string, req domain.RecommendedBuildsRequest) (domain.RecommendedBuilds, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.buildsReq = &req
	if c.buildsErr != nil {
		return domain.RecommendedBuilds{}, c.buildsErr
	}
	if c.builds != nil {
		return *c.builds, nil
	}

	return defaultBuilds, nil
}

type lcuStub struct {
	detectedSelection        domain.DetectedSelection
	detectErr                error
	detectCalls              int
	itemSetCalls             []domain.ApplyItemSetRequest
	runePageCalls            []domain.ApplyRunePageRequest
	runePageErr              error
	spellCalls               []domain.ApplySummonerSpellsRequest
	watchCalls               int
	watchEventsWithNoticesFn func(context.Context, chan<- domain.LCUEvent, chan<- domain.LCUWatchNotice) error
	watchErr                 error
}

func (l *lcuStub) DetectSelection(_ context.Context) (domain.DetectedSelection, error) {
	l.detectCalls++
	if l.detectErr != nil {
		return domain.DetectedSelection{}, l.detectErr
	}
	return l.detectedSelection, nil
}

func (l *lcuStub) ApplyItemSet(_ context.Context, req domain.ApplyItemSetRequest) error {
	l.itemSetCalls = append(l.itemSetCalls, req)
	return nil
}

func (l *lcuStub) ApplyRunePage(_ context.Context, req domain.ApplyRunePageRequest) error {
	l.runePageCalls = append(l.runePageCalls, req)
	return l.runePageErr
}

func (l *lcuStub) ApplySummonerSpells(_ context.Context, req domain.ApplySummonerSpellsRequest) error {
	l.spellCalls = append(l.spellCalls, req)
	return nil
}

func (l *lcuStub) WatchEventsWithNotices(ctx context.Context, out chan<- domain.LCUEvent, notices chan<- domain.LCUWatchNotice) error {
	l.watchCalls++
	if l.watchEventsWithNoticesFn != nil {
		return l.watchEventsWithNoticesFn(ctx, out, notices)
	}
	return l.watchErr
}
