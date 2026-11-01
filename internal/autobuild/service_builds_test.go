package autobuild

import (
	"reflect"
	"testing"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func TestItemSetBlocksFromBuild(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		build domain.RecommendedBuild
		want  []domain.ApplyItemSetBlock
	}{
		{
			name: "merges starter and early, suffixes choose one",
			build: domain.RecommendedBuild{
				ItemBlocks: []domain.BuildBlock{
					{Type: domain.BlockStarter, Items: []domain.BuildBlockItem{{ItemID: 1055}}},
					{Type: domain.BlockEarly, Items: []domain.BuildBlockItem{{ItemID: 2031}}},
					{Type: domain.BlockCore, Items: []domain.BuildBlockItem{
						{ItemID: 6632},
						{ItemID: 3158, ChooseOne: true},
					}},
				},
			},
			want: []domain.ApplyItemSetBlock{
				{Type: "Starter | Early Game", ItemIDs: []int{1055, 2031}},
				{Type: "Core Build & Boots (In Order) (Choose One)", ItemIDs: []int{6632, 3158}},
			},
		},
		{
			name: "single block keeps only its own label",
			build: domain.RecommendedBuild{
				ItemBlocks: []domain.BuildBlock{
					{Type: domain.BlockStarter, Items: []domain.BuildBlockItem{{ItemID: 1055}}},
				},
			},
			want: []domain.ApplyItemSetBlock{
				{Type: "Starter", ItemIDs: []int{1055}},
			},
		},
		{
			name: "labels third options and lategame",
			build: domain.RecommendedBuild{
				ItemBlocks: []domain.BuildBlock{
					{Type: domain.BlockThirdOption, Items: []domain.BuildBlockItem{{ItemID: 3074}}},
					{Type: domain.BlockLategame, Items: []domain.BuildBlockItem{{ItemID: 3036, ChooseOne: true}}},
				},
			},
			want: []domain.ApplyItemSetBlock{
				{Type: "3rd Item", ItemIDs: []int{3074}},
				{Type: "Late Game (Choose One)", ItemIDs: []int{3036}},
			},
		},
		{
			name: "skips empty and unknown blocks",
			build: domain.RecommendedBuild{
				ItemBlocks: []domain.BuildBlock{
					{Type: domain.BlockStarter},
					{Type: 99, Items: []domain.BuildBlockItem{{ItemID: 1001}}},
					{Type: domain.BlockLategame, Items: []domain.BuildBlockItem{{ItemID: 3036}}},
				},
			},
			want: []domain.ApplyItemSetBlock{
				{Type: "Late Game", ItemIDs: []int{3036}},
			},
		},
		{
			name:  "no blocks yields nil",
			build: domain.RecommendedBuild{},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := itemSetBlocksFromBuild(tt.build)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("itemSetBlocksFromBuild() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRunePageFromBuild(t *testing.T) {
	t.Parallel()

	build := domain.RecommendedBuild{
		Runes: &domain.BuildRunes{
			PrimaryTree:    domain.RuneStylePrecision,
			SecondaryTree:  domain.RuneStyleSorcery,
			SelectedRunes:  []int{8010, 8009, 9104, 8014, 8224, 8233},
			SelectedShards: []int{5008, 5008, 5011},
		},
	}

	page, ok := runePageFromBuild(build)
	if !ok {
		t.Fatal("runePageFromBuild() = false, want true")
	}
	want := domain.RunePage{
		PrimaryStyleID:  domain.RuneStylePrecision,
		SubStyleID:      domain.RuneStyleSorcery,
		SelectedPerkIDs: []int{8010, 8009, 9104, 8014, 8224, 8233, 5008, 5008, 5011},
	}
	if !reflect.DeepEqual(page, want) {
		t.Fatalf("runePageFromBuild() = %#v, want %#v", page, want)
	}

	page, ok = runePageFromBuild(domain.RecommendedBuild{})
	if ok {
		t.Fatalf("runePageFromBuild() without runes = %#v, want !ok", page)
	}
}
