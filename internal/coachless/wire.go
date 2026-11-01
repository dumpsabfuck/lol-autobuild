package coachless

import (
	"encoding/json"
	"slices"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

type apiRefreshResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type apiPatchInfo struct {
	Label      string `json:"label"`
	Major      int    `json:"major"`
	Patch      int    `json:"patch"`
	MatchCount int    `json:"matchCount"`
}

type apiPatchFilter struct {
	Major          int `json:"major"`
	Patch          int `json:"patch"`
	PatchAdditions int `json:"patchAdditions"`
}

type apiCommonFilters struct {
	Patch              apiPatchFilter `json:"patch"`
	ChampionIDs        []int          `json:"championIds"`
	MatchupChampionIDs []int          `json:"matchupChampionIds,omitempty"`
	LeagueTiers        []int          `json:"leagueTiers"`
	Regions            []int          `json:"regions,omitempty"`
	Role               int            `json:"role"`
}

type apiRecommendedBuildsRequest struct {
	CommonFilters apiCommonFilters `json:"commonFilters"`
}

type apiRecommendedBuildsResponse struct {
	Role      int                   `json:"role"`
	Builds    []apiRecommendedBuild `json:"builds"`
	SpellDuos []apiSpellDuo         `json:"spellDuos"`
}

type apiRecommendedBuild struct {
	Tier       int             `json:"tier"`
	Winrate    float64         `json:"winrate"`
	Runes      *apiBuildRunes  `json:"runes"`
	ItemBlocks []apiBuildBlock `json:"itemBlocks"`
}

// apiIntList tolerates JSON null entries (e.g. unpicked rune shards and spell slots).
type apiIntList []int

func (l *apiIntList) UnmarshalJSON(data []byte) error {
	var raw []any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	out := make(apiIntList, 0, len(raw))
	for _, value := range raw {
		if number, ok := value.(float64); ok {
			out = append(out, int(number))
		}
	}
	*l = out
	return nil
}

type apiBuildRunes struct {
	PrimaryTree    int        `json:"primaryTree"`
	SecondaryTree  int        `json:"secondaryTree"`
	SelectedRunes  apiIntList `json:"selectedRunes"`
	SelectedShards apiIntList `json:"selectedShards"`
}

type apiBuildBlock struct {
	Type  int                 `json:"type"`
	Items []apiBuildBlockItem `json:"items"`
}

type apiBuildBlockItem struct {
	ItemID    int  `json:"itemId"`
	ChooseOne bool `json:"chooseOne"`
}

type apiSpellDuo struct {
	SpellIDs apiIntList `json:"spellIds"`
}

func apiRecommendedBuildsRequestFromDomain(req domain.RecommendedBuildsRequest) apiRecommendedBuildsRequest {
	return apiRecommendedBuildsRequest{CommonFilters: apiCommonFiltersFromDomain(req.CommonFilters)}
}

func apiCommonFiltersFromDomain(filters domain.CommonFilters) apiCommonFilters {
	return apiCommonFilters{
		Patch:              apiPatchFilterFromDomain(filters.Patch),
		ChampionIDs:        slices.Clone(filters.ChampionIDs),
		MatchupChampionIDs: slices.Clone(filters.MatchupChampionIDs),
		LeagueTiers:        slices.Clone(filters.LeagueTiers),
		Regions:            slices.Clone(filters.Regions),
		Role:               filters.Role,
	}
}

func apiPatchFilterFromDomain(filter domain.PatchFilter) apiPatchFilter {
	return apiPatchFilter{
		Major:          filter.Major,
		Patch:          filter.Patch,
		PatchAdditions: filter.PatchAdditions,
	}
}

func patchInfosFromAPI(in []apiPatchInfo) []domain.PatchInfo {
	out := make([]domain.PatchInfo, 0, len(in))
	for _, patch := range in {
		out = append(out, domain.PatchInfo{
			Label: patch.Label,
			Major: patch.Major,
			Patch: patch.Patch,
		})
	}
	return out
}

func recommendedBuildsFromAPI(in apiRecommendedBuildsResponse) domain.RecommendedBuilds {
	out := domain.RecommendedBuilds{
		Role:      in.Role,
		Builds:    make([]domain.RecommendedBuild, 0, len(in.Builds)),
		SpellDuos: make([]domain.SpellDuo, 0, len(in.SpellDuos)),
	}
	for _, build := range in.Builds {
		out.Builds = append(out.Builds, recommendedBuildFromAPI(build))
	}
	for _, duo := range in.SpellDuos {
		out.SpellDuos = append(out.SpellDuos, domain.SpellDuo{SpellIDs: slices.Clone(duo.SpellIDs)})
	}
	return out
}

func recommendedBuildFromAPI(in apiRecommendedBuild) domain.RecommendedBuild {
	build := domain.RecommendedBuild{
		Tier:       in.Tier,
		Winrate:    in.Winrate,
		ItemBlocks: make([]domain.BuildBlock, 0, len(in.ItemBlocks)),
	}
	if in.Runes != nil {
		build.Runes = &domain.BuildRunes{
			PrimaryTree:    in.Runes.PrimaryTree,
			SecondaryTree:  in.Runes.SecondaryTree,
			SelectedRunes:  slices.Clone([]int(in.Runes.SelectedRunes)),
			SelectedShards: slices.Clone([]int(in.Runes.SelectedShards)),
		}
	}
	for _, block := range in.ItemBlocks {
		items := make([]domain.BuildBlockItem, 0, len(block.Items))
		for _, item := range block.Items {
			items = append(items, domain.BuildBlockItem{ItemID: item.ItemID, ChooseOne: item.ChooseOne})
		}
		build.ItemBlocks = append(build.ItemBlocks, domain.BuildBlock{Type: block.Type, Items: items})
	}
	return build
}
