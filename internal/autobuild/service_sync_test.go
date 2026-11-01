package autobuild

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func newTestService(coachless *coachlessStub, lcu *lcuStub) (Service, error) {
	return NewService(ServiceDeps{
		Coachless: coachless,
		Tokens:    tokenProviderStub{token: "t"},
		LCU:       lcu,
	})
}

func topSelection() domain.DetectedSelection {
	return domain.DetectedSelection{
		ChampionID:   240,
		ChampionName: "Kled",
		Position:     domain.Top,
		QueueID:      420,
	}
}

func TestSyncDryRunDetectsChampionButDoesNotApplyLCU(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{detectedSelection: topSelection()}
	svc, err := newTestService(coachless, lcu)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if got.DetectedChampionID != 240 || got.DetectedChampionName != "Kled" ||
		got.DetectedPosition != domain.Top.String() || got.DetectedQueueID != 420 {
		t.Fatalf("unexpected detection: %#v", got)
	}
	if got.ItemSetApplied || got.RunePageApplied || got.SpellsApplied {
		t.Fatalf("expected no applied flags in dry-run, got %#v", got)
	}
	if lcu.detectCalls != 1 {
		t.Fatalf("expected exactly one detect call, got %d", lcu.detectCalls)
	}
	if len(lcu.itemSetCalls)+len(lcu.runePageCalls)+len(lcu.spellCalls) != 0 {
		t.Fatalf("expected no LCU apply calls in dry-run")
	}

	coachless.mu.Lock()
	req := coachless.buildsReq
	coachless.mu.Unlock()
	if req == nil {
		t.Fatal("expected one recommended builds call")
	}
	if req.CommonFilters.Role != 0 {
		t.Fatalf("expected detected top role code 0, got %d", req.CommonFilters.Role)
	}

	found := false
	for _, warning := range got.Warnings {
		if strings.Contains(warning, "dry-run enabled") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected dry-run warning, got %#v", got.Warnings)
	}
}

func TestSyncUsesDetectedChampionIDInApplyRequests(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID:   777,
			ChampionName: "Yone",
			Position:     domain.Support,
			QueueID:      440,
			IsAutofilled: true,
		},
	}
	svc, err := newTestService(coachless, lcu)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{
		ApplyItems:  true,
		ApplyRunes:  true,
		ApplySpells: true,
		KeepFlash:   true,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if len(lcu.itemSetCalls) != 1 || len(lcu.runePageCalls) != 1 || len(lcu.spellCalls) != 1 {
		t.Fatalf("unexpected LCU apply calls: items=%d runes=%d spells=%d", len(lcu.itemSetCalls), len(lcu.runePageCalls), len(lcu.spellCalls))
	}
	if lcu.itemSetCalls[0].ChampionID != 777 || lcu.runePageCalls[0].ChampionID != 777 || lcu.spellCalls[0].ChampionID != 777 {
		t.Fatalf("apply calls must use detected champion id")
	}
	if lcu.itemSetCalls[0].ChampionName != "Yone" || lcu.runePageCalls[0].ChampionName != "Yone" {
		t.Fatalf("item and rune apply calls must use detected champion name")
	}
	if lcu.itemSetCalls[0].Position != domain.Support || lcu.runePageCalls[0].Position != domain.Support || lcu.spellCalls[0].Position != domain.Support {
		t.Fatalf("apply calls must use detected role")
	}
	if !lcu.spellCalls[0].KeepFlash {
		t.Fatalf("summoner spell apply should keep flash")
	}

	wantPage := domain.RunePage{
		PrimaryStyleID:  domain.RuneStyleResolve,
		SubStyleID:      domain.RuneStyleSorcery,
		SelectedPerkIDs: []int{domain.RuneKeystoneGraspOfTheUndying, 8446, 8444, 8451, 8224, 8233, 5008, 5008, 5002},
	}
	if !slices.Equal(lcu.runePageCalls[0].Page.SelectedPerkIDs, wantPage.SelectedPerkIDs) ||
		lcu.runePageCalls[0].Page.PrimaryStyleID != wantPage.PrimaryStyleID ||
		lcu.runePageCalls[0].Page.SubStyleID != wantPage.SubStyleID {
		t.Fatalf("unexpected rune page: %#v", lcu.runePageCalls[0].Page)
	}

	wantSpells := []int{4, 14}
	if !slices.Equal(lcu.spellCalls[0].SpellIDs, wantSpells) {
		t.Fatalf("unexpected spells: %#v", lcu.spellCalls[0].SpellIDs)
	}

	coachless.mu.Lock()
	req := coachless.buildsReq
	coachless.mu.Unlock()
	if req.CommonFilters.Role != 4 {
		t.Fatalf("expected support role code 4, got %d", req.CommonFilters.Role)
	}
	if len(req.CommonFilters.ChampionIDs) != 1 || req.CommonFilters.ChampionIDs[0] != 777 {
		t.Fatalf("coachless filters must include detected champion id 777")
	}
	if !slices.Contains(got.Warnings, "autofill detected in current champ select") {
		t.Fatalf("expected autofill warning, got %#v", got.Warnings)
	}
}

func TestSyncAppliesFirstRecommendedBuild(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{builds: &domain.RecommendedBuilds{
		Builds: []domain.RecommendedBuild{
			defaultBuilds.Builds[0],
			{
				ItemBlocks: []domain.BuildBlock{
					{Type: domain.BlockStarter, Items: []domain.BuildBlockItem{{ItemID: 1056}}},
				},
			},
		},
		SpellDuos: defaultBuilds.SpellDuos,
	}}
	lcu := &lcuStub{detectedSelection: topSelection()}
	svc, err := newTestService(coachless, lcu)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{ApplyItems: true, ApplyRunes: true, ApplySpells: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	blocks := lcu.itemSetCalls[0].Blocks
	for _, b := range blocks {
		t.Logf("block %q items=%v", b.Type, b.ItemIDs)
	}
	if len(blocks) != 1 || blocks[0].Type != "Starter" || !slices.Equal(blocks[0].ItemIDs, []int{1055}) {
		t.Fatalf("expected first build's starter block, got %d blocks", len(blocks))
	}
	if !slices.Contains(got.Warnings, "selected build: tier S, 52.3% win rate") {
		t.Fatalf("expected build tier warning, got %#v", got.Warnings)
	}
}

func TestSyncWarnsWhenNoBuildAvailable(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{builds: &domain.RecommendedBuilds{}}
	lcu := &lcuStub{detectedSelection: topSelection()}
	svc, err := newTestService(coachless, lcu)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{ApplyItems: true, ApplyRunes: true, ApplySpells: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if got.ItemSetApplied || got.RunePageApplied || got.SpellsApplied {
		t.Fatalf("expected no applies, got %#v", got)
	}
	if len(lcu.itemSetCalls)+len(lcu.runePageCalls)+len(lcu.spellCalls) != 0 {
		t.Fatalf("expected no LCU apply calls")
	}
	if !slices.Contains(got.Warnings, "no build recommendation was available") {
		t.Fatalf("expected no-build warning, got %#v", got.Warnings)
	}
}

func TestSyncWarnsWhenBuildHasNoRunes(t *testing.T) {
	t.Parallel()

	noRunes := defaultBuilds
	noRunes.Builds = slices.Clone(defaultBuilds.Builds)
	noRunes.Builds[0].Runes = nil

	coachless := &coachlessStub{builds: &noRunes}
	lcu := &lcuStub{detectedSelection: topSelection()}
	svc, err := newTestService(coachless, lcu)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{ApplyItems: true, ApplyRunes: true, ApplySpells: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if got.RunePageApplied {
		t.Fatalf("expected no rune page apply, got %#v", got)
	}
	if len(lcu.runePageCalls) != 0 {
		t.Fatalf("expected no rune page apply call")
	}
	if !got.ItemSetApplied || !got.SpellsApplied {
		t.Fatalf("expected spells and items to still be applied, got %#v", got)
	}
	if !slices.Contains(got.Warnings, "apply runes requested but no rune page recommendation was available") {
		t.Fatalf("expected no-runes warning, got %#v", got.Warnings)
	}
}

func TestSyncTranslatesRunePageLimitWarning(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{
		detectedSelection: topSelection(),
		runePageErr:       fmt.Errorf(`apply rune page failed: {"errorCode":"RPC_ERROR","message":"Max pages reached"}: %w`, domain.ErrRunePageLimitReached),
	}
	svc, err := newTestService(coachless, lcu)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := svc.Sync(context.Background(), SyncRequest{ApplyRunes: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	if got.RunePageApplied {
		t.Fatalf("expected rune page apply to fail, got %#v", got)
	}
	if len(lcu.runePageCalls) != 1 {
		t.Fatalf("expected one rune page apply call, got %d", len(lcu.runePageCalls))
	}

	foundLimitWarning := false
	for _, warning := range got.Warnings {
		if warning == RunePageLimitReachedWarning {
			foundLimitWarning = true
		}
		if strings.Contains(warning, "RPC_ERROR") || strings.Contains(warning, "Max pages reached") {
			t.Fatalf("expected translated warning without raw LCU body, got %#v", got.Warnings)
		}
	}
	if !foundLimitWarning {
		t.Fatalf("expected rune page limit warning, got %#v", got.Warnings)
	}
}

func TestSyncFailsFastWhenChampionDetectionFails(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{}
	lcu := &lcuStub{detectErr: errors.New("no session")}
	svc, err := newTestService(coachless, lcu)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = svc.Sync(context.Background(), SyncRequest{ApplyItems: true, ApplyRunes: true, ApplySpells: true})
	if err == nil {
		t.Fatal("expected sync error when detect champion id fails")
	}
	if lcu.detectCalls != 1 {
		t.Fatalf("expected one detect call, got %d", lcu.detectCalls)
	}
	if coachless.getPatchesCalls != 0 {
		t.Fatalf("coachless should not be called when detection fails, got getPatches=%d", coachless.getPatchesCalls)
	}
	if len(lcu.itemSetCalls)+len(lcu.runePageCalls)+len(lcu.spellCalls) != 0 {
		t.Fatalf("LCU apply should not run when detection fails")
	}
}

func TestSyncReturnsErrorWhenBuildsQueryFails(t *testing.T) {
	t.Parallel()

	queryErr := errors.New("builds endpoint failed")
	coachless := &coachlessStub{buildsErr: queryErr}
	lcu := &lcuStub{detectedSelection: topSelection()}
	svc, err := newTestService(coachless, lcu)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = svc.Sync(context.Background(), SyncRequest{ApplyItems: true, ApplyRunes: true, ApplySpells: true})
	if err == nil {
		t.Fatal("expected sync error when builds query fails")
	}
	if !errors.Is(err, queryErr) {
		t.Fatalf("expected wrapped query error, got %v", err)
	}
	if len(lcu.itemSetCalls)+len(lcu.runePageCalls)+len(lcu.spellCalls) != 0 {
		t.Fatalf("LCU apply should not run when builds query fails")
	}
}

func TestSyncUsesAdvancedCoachlessFilters(t *testing.T) {
	t.Parallel()

	coachless := &coachlessStub{
		patches: []domain.PatchInfo{
			{Label: "16.6", Major: 16, Patch: 6},
			{Label: "16.7", Major: 16, Patch: 7},
			{Label: "16.8", Major: 16, Patch: 8},
		},
	}
	lcu := &lcuStub{
		detectedSelection: domain.DetectedSelection{
			ChampionID: 240,
			Position:   domain.Top,
			QueueID:    420,
			EnemyChampions: []domain.ChampionRef{
				{ID: 10, Name: "Kayle"},
				{ID: 20, Name: "Nunu & Willump"},
				{ID: 30, Name: "Ashe"},
				{ID: 40, Name: "Jhin"},
				{ID: 50, Name: "Karma"},
				{ID: 60, Name: "Elise"},
			},
		},
	}
	svc, err := NewService(ServiceDeps{
		Coachless: coachless,
		Tokens:    tokenProviderStub{token: "t", claims: domain.TokenClaims{Subscribed: true}},
		LCU:       lcu,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	_, err = svc.Sync(context.Background(), SyncRequest{
		PatchAdditionsMode: PatchAdditionsModeManual,
		PatchAdditions:     PatchAdditionsDefault,
		LeagueTierPreset:   LeagueTierPresetDiamondPlus,
		Regions:            []int{CoachlessRegionNA, CoachlessRegionBR, CoachlessRegionNA},
		MatchupChampionIDs: []int{60, 20, 0, 20, 10, 30, 40, 50},
		ApplyItems:         true,
		ApplyRunes:         true,
		ApplySpells:        true,
		DryRun:             true,
	})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	coachless.mu.Lock()
	req := coachless.buildsReq
	coachless.mu.Unlock()
	if req == nil {
		t.Fatal("expected one recommended builds call")
	}

	gotFilters := req.CommonFilters
	if gotFilters.Patch.Major != 16 || gotFilters.Patch.Patch != 8 || gotFilters.Patch.PatchAdditions != PatchAdditionsDefault {
		t.Fatalf("unexpected patch filter: %#v", gotFilters.Patch)
	}
	if !slices.Equal(gotFilters.LeagueTiers, []int{6, 7}) {
		t.Fatalf("league tiers = %#v, want %#v", gotFilters.LeagueTiers, []int{6, 7})
	}
	if !slices.Equal(gotFilters.Regions, []int{CoachlessRegionBR, CoachlessRegionNA}) {
		t.Fatalf("regions = %#v, want %#v", gotFilters.Regions, []int{CoachlessRegionBR, CoachlessRegionNA})
	}
	if !slices.Equal(gotFilters.MatchupChampionIDs, []int{10, 20, 30, 40, 60}) {
		t.Fatalf("matchups = %#v, want filtered roster order", gotFilters.MatchupChampionIDs)
	}
}

func TestResolvePatch(t *testing.T) {
	t.Parallel()

	defaultPatches := []domain.PatchInfo{
		{Label: "16.4", Major: 16, Patch: 4},
		{Label: "16.5", Major: 16, Patch: 5},
		{Label: "16.6", Major: 16, Patch: 6},
		{Label: "16.7", Major: 16, Patch: 7},
		{Label: "16.8", Major: 16, Patch: 8},
	}

	tests := []struct {
		name             string
		rawPatch         string
		mode             string
		additions        int
		patches          []domain.PatchInfo
		subscribed       bool
		wantLabel        string
		wantPatch        int
		wantAdditions    int
		wantWarningCount int
		wantErr          bool
	}{
		{name: "free auto selects latest non-premium patch", mode: PatchAdditionsModeAuto, patches: defaultPatches, wantLabel: "16.7", wantPatch: 7},
		{name: "premium auto selects latest patch with two additions", mode: PatchAdditionsModeAuto, patches: defaultPatches, subscribed: true, wantLabel: "16.8", wantPatch: 8, wantAdditions: PatchAdditionsDefault},
		{name: "premium manual current patch", mode: PatchAdditionsModeManual, patches: defaultPatches, additions: 0, subscribed: true, wantLabel: "16.8", wantPatch: 8},
		{name: "premium manual one addition", mode: PatchAdditionsModeManual, patches: defaultPatches, additions: 1, subscribed: true, wantLabel: "16.8", wantPatch: 8, wantAdditions: 1},
		{name: "premium manual two additions", mode: PatchAdditionsModeManual, patches: defaultPatches, additions: PatchAdditionsDefault, subscribed: true, wantLabel: "16.8", wantPatch: 8, wantAdditions: PatchAdditionsDefault},
		{name: "premium manual four additions", mode: PatchAdditionsModeManual, patches: defaultPatches, additions: PatchAdditionsMax, subscribed: true, wantLabel: "16.8", wantPatch: 8, wantAdditions: PatchAdditionsMax},
		{name: "free manual additions require premium", mode: PatchAdditionsModeManual, patches: defaultPatches, additions: 1, wantErr: true},
		{name: "explicit older patch is allowed for free", rawPatch: "16.6", mode: PatchAdditionsModeAuto, patches: defaultPatches, wantLabel: "16.6", wantPatch: 6},
		{name: "explicit latest patch requires premium for free", rawPatch: "16.8", mode: PatchAdditionsModeAuto, patches: defaultPatches, wantErr: true},
		{name: "manual additions clamp to available history", mode: PatchAdditionsModeManual, patches: defaultPatches[:2], additions: PatchAdditionsMax, subscribed: true, wantLabel: "16.5", wantPatch: 5, wantAdditions: 1, wantWarningCount: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, label, warnings, err := resolvePatch(tt.rawPatch, tt.mode, tt.additions, tt.patches, tt.subscribed)
			if tt.wantErr {
				if err == nil {
					t.Fatal("resolvePatch() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolvePatch() error = %v", err)
			}
			if label != tt.wantLabel {
				t.Fatalf("label = %q, want %q", label, tt.wantLabel)
			}
			if got.Patch != tt.wantPatch || got.PatchAdditions != tt.wantAdditions {
				t.Fatalf("filter = %#v, want patch=%d additions=%d", got, tt.wantPatch, tt.wantAdditions)
			}
			if len(warnings) != tt.wantWarningCount {
				t.Fatalf("warnings = %#v, want count %d", warnings, tt.wantWarningCount)
			}
		})
	}
}

func TestResolveLeagueTierPreset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		preset  string
		want    []int
		wantErr bool
	}{
		{name: "empty defaults to emerald plus", want: []int{5, 6, 7}},
		{name: "gold plus", preset: LeagueTierPresetGoldPlus, want: []int{3, 4, 5, 6, 7}},
		{name: "platinum plus", preset: LeagueTierPresetPlatinumPlus, want: []int{4, 5, 6, 7}},
		{name: "emerald plus", preset: LeagueTierPresetEmeraldPlus, want: []int{5, 6, 7}},
		{name: "diamond plus", preset: LeagueTierPresetDiamondPlus, want: []int{6, 7}},
		{name: "master plus", preset: LeagueTierPresetMasterPlus, want: []int{7}},
		{name: "invalid", preset: "silver_plus", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveLeagueTierPreset(tt.preset)
			if tt.wantErr {
				if err == nil {
					t.Fatal("resolveLeagueTierPreset() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveLeagueTierPreset() error = %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("tiers = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNormalizeCoachlessRegions(t *testing.T) {
	t.Parallel()

	allRegions := CoachlessRegionIDs()
	tests := []struct {
		name    string
		regions []int
		want    []int
		wantErr bool
	}{
		{name: "empty means all"},
		{name: "subset sorted by coachless order", regions: []int{CoachlessRegionNA, CoachlessRegionBR, CoachlessRegionNA}, want: []int{CoachlessRegionBR, CoachlessRegionNA}},
		{name: "all means all", regions: allRegions},
		{name: "invalid region", regions: []int{10}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeCoachlessRegions(tt.regions)
			if tt.wantErr {
				if err == nil {
					t.Fatal("NormalizeCoachlessRegions() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeCoachlessRegions() error = %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("regions = %#v, want %#v", got, tt.want)
			}
		})
	}
}
