package autobuild

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func (s *syncService) Sync(ctx context.Context, req SyncRequest) (SyncResult, error) {
	selection, err := s.deps.LCU.DetectSelection(ctx)
	if err != nil {
		return SyncResult{}, fmt.Errorf("detect local selection: %w", err)
	}

	accessToken, err := s.deps.Tokens.AccessToken(ctx)
	if err != nil {
		return SyncResult{}, fmt.Errorf("get access token: %w", err)
	}

	tokenClaims, err := s.deps.Tokens.Claims(ctx)
	if err != nil {
		return SyncResult{}, fmt.Errorf("get token claims: %w", err)
	}

	patches, err := s.deps.Coachless.GetPatches(ctx, accessToken)
	if err != nil {
		return SyncResult{}, fmt.Errorf("get patches: %w", err)
	}

	patchFilter, patchLabel, patchWarnings, err := resolvePatch(
		req.Patch,
		req.PatchAdditionsMode,
		req.PatchAdditions,
		patches,
		tokenClaims.IsSubscribed(),
	)
	if err != nil {
		return SyncResult{}, err
	}

	leagueTiers, err := resolveLeagueTierPreset(req.LeagueTierPreset)
	if err != nil {
		return SyncResult{}, err
	}

	regions, err := NormalizeCoachlessRegions(req.Regions)
	if err != nil {
		return SyncResult{}, err
	}

	filters := domain.CommonFilters{
		Patch:              patchFilter,
		ChampionIDs:        []int{selection.ChampionID},
		MatchupChampionIDs: domain.MatchupChampionIDsForRoster(req.MatchupChampionIDs, selection.EnemyChampions, domain.MaxMatchupChampionIDs),
		LeagueTiers:        leagueTiers,
		Regions:            regions,
		Role:               selection.Position.Code(),
	}

	builds, err := s.deps.Coachless.GetRecommendedBuilds(ctx, accessToken, domain.RecommendedBuildsRequest{CommonFilters: filters})
	if err != nil {
		return SyncResult{}, fmt.Errorf("get recommended builds: %w", err)
	}

	result := SyncResult{
		DetectedChampionID:   selection.ChampionID,
		DetectedChampionName: selection.ChampionName,
		DetectedPosition:     selection.Position.String(),
		DetectedQueueID:      selection.QueueID,
		Warnings: slices.Concat(
			[]string{selectedPatchWarning(patchLabel, patchFilter.PatchAdditions)},
			patchWarnings,
		),
	}

	if selection.IsAutofilled {
		result.Warnings = append(result.Warnings, "autofill detected in current champ select")
	}

	if len(builds.Builds) == 0 {
		result.Warnings = append(result.Warnings, "no build recommendation was available")
		return result, nil
	}

	build := builds.Builds[0]
	result.Warnings = append(result.Warnings, selectedBuildWarning(build))

	if req.DryRun {
		result.Warnings = append(result.Warnings, "dry-run enabled: no LCU changes were applied")
		return result, nil
	}

	if req.ApplyRunes {
		page, ok := runePageFromBuild(build)
		if !ok {
			result.Warnings = append(result.Warnings, "apply runes requested but no rune page recommendation was available")
		} else if err := s.deps.LCU.ApplyRunePage(ctx, domain.ApplyRunePageRequest{
			ChampionID:   selection.ChampionID,
			ChampionName: selection.ChampionName,
			Position:     selection.Position,
			Page:         page,
			DryRun:       false,
		}); err != nil {
			result.Warnings = append(result.Warnings, runePageApplyWarning(err))
		} else {
			result.RunePageApplied = true
		}
	}

	if req.ApplySpells {
		var spellIDs []int
		if len(builds.SpellDuos) > 0 {
			spellIDs = builds.SpellDuos[0].SpellIDs
		}

		if len(spellIDs) == 0 {
			result.Warnings = append(result.Warnings, "apply spells requested but no spell recommendation was available")
		} else if err := s.deps.LCU.ApplySummonerSpells(ctx, domain.ApplySummonerSpellsRequest{
			ChampionID: selection.ChampionID,
			Position:   selection.Position,
			SpellIDs:   spellIDs,
			KeepFlash:  req.KeepFlash,
			DryRun:     false,
		}); err != nil {
			result.Warnings = append(result.Warnings, "failed to apply summoner spells: "+err.Error())
		} else {
			result.SpellsApplied = true
		}
	}

	if req.ApplyItems {
		blocks := itemSetBlocksFromBuild(build)
		if len(blocks) == 0 {
			result.Warnings = append(result.Warnings, "apply items requested but no item recommendation was available")
		} else if err := s.deps.LCU.ApplyItemSet(ctx, domain.ApplyItemSetRequest{
			ChampionID:   selection.ChampionID,
			ChampionName: selection.ChampionName,
			Position:     selection.Position,
			Blocks:       blocks,
			DryRun:       false,
		}); err != nil {
			result.Warnings = append(result.Warnings, "failed to apply item set: "+err.Error())
		} else {
			result.ItemSetApplied = true
		}
	}

	return result, nil
}

var buildTierLabels = []string{"S", "A", "B"}

func selectedBuildWarning(build domain.RecommendedBuild) string {
	label := "?"
	if build.Tier >= 0 && build.Tier < len(buildTierLabels) {
		label = buildTierLabels[build.Tier]
	}

	return fmt.Sprintf("selected build: tier %s, %.1f%% win rate", label, build.Winrate)
}

func resolvePatch(rawPatch string, rawPatchAdditionsMode string, requestedPatchAdditions int, patches []domain.PatchInfo, subscribed bool) (domain.PatchFilter, string, []string, error) {
	if len(patches) == 0 {
		return domain.PatchFilter{}, "", nil, errors.New("no patch data available")
	}

	patchAdditionsMode := strings.TrimSpace(rawPatchAdditionsMode)
	if patchAdditionsMode == "" {
		patchAdditionsMode = PatchAdditionsModeAuto
	}
	if patchAdditionsMode != PatchAdditionsModeAuto && patchAdditionsMode != PatchAdditionsModeManual {
		return domain.PatchFilter{}, "", nil, fmt.Errorf("patch additions mode %q is invalid", rawPatchAdditionsMode)
	}
	if requestedPatchAdditions < 0 || requestedPatchAdditions > PatchAdditionsMax {
		return domain.PatchFilter{}, "", nil, fmt.Errorf("patch additions %d must be between 0 and %d", requestedPatchAdditions, PatchAdditionsMax)
	}

	selectedIndex := len(patches) - 1
	if strings.TrimSpace(rawPatch) != "" {
		wanted := strings.TrimSpace(rawPatch)
		selectedIndex = slices.IndexFunc(patches, func(p domain.PatchInfo) bool {
			return p.Label == wanted
		})
		if selectedIndex < 0 {
			return domain.PatchFilter{}, "", nil, fmt.Errorf("requested patch %q not found", rawPatch)
		}
	} else if !subscribed && len(patches) > 1 {
		selectedIndex = len(patches) - 2
	}

	if !subscribed && selectedIndex == len(patches)-1 && len(patches) > 1 {
		return domain.PatchFilter{}, "", nil, fmt.Errorf("requested patch %q requires Coachless Premium", patches[selectedIndex].Label)
	}

	selected := patches[selectedIndex]
	patchAdditions, warnings, err := resolvePatchAdditions(patchAdditionsMode, requestedPatchAdditions, selectedIndex, subscribed)
	if err != nil {
		return domain.PatchFilter{}, "", nil, err
	}

	return domain.PatchFilter{
		Major:          selected.Major,
		Patch:          selected.Patch,
		PatchAdditions: patchAdditions,
	}, selected.Label, warnings, nil
}

func resolvePatchAdditions(mode string, requestedPatchAdditions int, selectedPatchIndex int, subscribed bool) (int, []string, error) {
	if mode == PatchAdditionsModeAuto {
		if !subscribed {
			return 0, nil, nil
		}
		return min(PatchAdditionsDefault, selectedPatchIndex), nil, nil
	}

	if requestedPatchAdditions > 0 && !subscribed {
		return 0, nil, errors.New("requested patch additions require Coachless Premium")
	}

	maxAvailable := min(PatchAdditionsMax, selectedPatchIndex)
	if requestedPatchAdditions <= maxAvailable {
		return requestedPatchAdditions, nil, nil
	}

	warnings := []string{
		fmt.Sprintf("patch additions reduced from %d to %d because only %d previous patches are available", requestedPatchAdditions, maxAvailable, maxAvailable),
	}
	return maxAvailable, warnings, nil
}

func selectedPatchWarning(patchLabel string, patchAdditions int) string {
	switch {
	case patchAdditions <= 0:
		return fmt.Sprintf("selected patch: %s", patchLabel)
	case patchAdditions == 1:
		return fmt.Sprintf("selected patch: %s (+1 previous patch)", patchLabel)
	default:
		return fmt.Sprintf("selected patch: %s (+%d previous patches)", patchLabel, patchAdditions)
	}
}

func resolveLeagueTierPreset(rawPreset string) ([]int, error) {
	preset := strings.TrimSpace(rawPreset)
	if preset == "" {
		preset = LeagueTierPresetDefault
	}

	switch preset {
	case LeagueTierPresetGoldPlus:
		return []int{3, 4, 5, 6, 7}, nil
	case LeagueTierPresetPlatinumPlus:
		return []int{4, 5, 6, 7}, nil
	case LeagueTierPresetEmeraldPlus:
		return []int{5, 6, 7}, nil
	case LeagueTierPresetDiamondPlus:
		return []int{6, 7}, nil
	case LeagueTierPresetMasterPlus:
		return []int{7}, nil
	default:
		return nil, fmt.Errorf("league tier preset %q is invalid", rawPreset)
	}
}
