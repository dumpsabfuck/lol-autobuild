package autobuild

import (
	"slices"
	"strings"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

// Block groups mirror the coachless.gg LCU export: blocks are merged per
// group, empty groups are dropped, and any choose-one item suffixes the label.
var buildBlockGroups = []struct {
	labels []string
	types  []int
}{
	{labels: []string{"Starter", "Early Game"}, types: []int{domain.BlockStarter, domain.BlockEarly}},
	{labels: []string{"Core Build & Boots (In Order)"}, types: []int{domain.BlockCore}},
	{labels: []string{"3rd Item"}, types: []int{domain.BlockThirdOption}},
	{labels: []string{"Late Game"}, types: []int{domain.BlockLategame}},
}

func itemSetBlocksFromBuild(build domain.RecommendedBuild) []domain.ApplyItemSetBlock {
	var out []domain.ApplyItemSetBlock
	for _, group := range buildBlockGroups {
		var (
			ids       []int
			chooseOne bool
			labels    []string
		)
		for idx, blockType := range group.types {
			present := false
			for _, block := range build.ItemBlocks {
				if block.Type != blockType {
					continue
				}
				for _, item := range block.Items {
					if item.ItemID <= 0 {
						continue
					}
					if !slices.Contains(ids, item.ItemID) {
						ids = append(ids, item.ItemID)
					}
					chooseOne = chooseOne || item.ChooseOne
					present = true
				}
			}
			if present {
				labels = append(labels, group.labels[idx])
			}
		}
		if len(ids) == 0 {
			continue
		}

		label := strings.Join(labels, " | ")
		if chooseOne {
			label += " (Choose One)"
		}
		out = append(out, domain.ApplyItemSetBlock{Type: label, ItemIDs: ids})
	}
	return out
}

func runePageFromBuild(build domain.RecommendedBuild) (domain.RunePage, bool) {
	if build.Runes == nil {
		return domain.RunePage{}, false
	}

	perks := make([]int, 0, len(build.Runes.SelectedRunes)+len(build.Runes.SelectedShards))
	perks = append(perks, build.Runes.SelectedRunes...)
	perks = append(perks, build.Runes.SelectedShards...)

	return domain.RunePage{
		PrimaryStyleID:  build.Runes.PrimaryTree,
		SubStyleID:      build.Runes.SecondaryTree,
		SelectedPerkIDs: perks,
	}, true
}
