package domain

type PatchInfo struct {
	Label string
	Major int
	Patch int
}

type PatchFilter struct {
	Major          int
	Patch          int
	PatchAdditions int
}

type CommonFilters struct {
	Patch              PatchFilter
	ChampionIDs        []int
	MatchupChampionIDs []int
	LeagueTiers        []int
	Regions            []int
	Role               int
}

type RecommendedBuildsRequest struct {
	CommonFilters CommonFilters
}

type RecommendedBuilds struct {
	Role      int
	Builds    []RecommendedBuild
	SpellDuos []SpellDuo
}

type RecommendedBuild struct {
	Tier       int
	Winrate    float64
	Runes      *BuildRunes
	ItemBlocks []BuildBlock
}

type BuildRunes struct {
	PrimaryTree    int
	SecondaryTree  int
	SelectedRunes  []int
	SelectedShards []int
}

// Build block types as sent by coachless GetRecommendedBuilds.
const (
	BlockStarter     = 0
	BlockEarly       = 1
	BlockCore        = 2
	BlockThirdOption = 3
	BlockLategame    = 4
)

type BuildBlock struct {
	Type  int
	Items []BuildBlockItem
}

type BuildBlockItem struct {
	ItemID    int
	ChooseOne bool
}

type SpellDuo struct {
	SpellIDs []int
}
