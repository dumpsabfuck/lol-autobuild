package coachless

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/controlado/lol-autobuild/internal/autobuild/domain"
)

func TestGetPatchesParsesResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ChampionWinprob/GetPatches" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if got := r.Header.Get("Authorization"); got == "" {
			t.Fatalf("missing authorization header")
		}

		_ = json.NewEncoder(w).Encode([]apiPatchInfo{{
			Label:      "16.7",
			Major:      16,
			Patch:      7,
			MatchCount: 123,
		}})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	patches, err := client.GetPatches(context.Background(), "token")
	if err != nil {
		t.Fatalf("GetPatches() error = %v", err)
	}

	if len(patches) != 1 || patches[0].Label != "16.7" {
		t.Fatalf("unexpected patches: %#v", patches)
	}
}

func TestRefreshParsesTokens(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/Auth/refresh" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(map[string]string{
			"accessToken":  "access",
			"refreshToken": "refresh",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	pair, err := client.Refresh(context.Background(), "r1")
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if pair.AccessToken != "access" || pair.RefreshToken != "refresh" {
		t.Fatalf("unexpected pair: %#v", pair)
	}
}

func TestRefreshRejectsMissingTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response map[string]string
	}{
		{
			name: "missing access token",
			response: map[string]string{
				"refreshToken": "refresh",
			},
		},
		{
			name: "missing refresh token",
			response: map[string]string{
				"accessToken": "access",
			},
		},
		{
			name: "blank tokens",
			response: map[string]string{
				"accessToken":  " ",
				"refreshToken": "\t",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/Auth/refresh" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}

				_ = json.NewEncoder(w).Encode(tt.response)
			}))
			defer srv.Close()

			client := NewClient(srv.URL, 2*time.Second)
			if _, err := client.Refresh(context.Background(), "r1"); err == nil {
				t.Fatal("expected Refresh() error")
			}
		})
	}
}

func TestGetRecommendedBuildsToleratesNullPerkAndSpellSlots(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"role": 4,
			"builds": [{
				"keystone": 8437,
				"tier": 0,
				"winrate": 52.3,
				"runes": {
					"primaryTree": 8400,
					"secondaryTree": 8200,
					"selectedRunes": [8437, 8446, 8444, 8451, 8224, 8233],
					"selectedShards": [5008, null, 5011]
				},
				"itemBlocks": []
			}],
			"spellDuos": [{"spellIds": [4, null]}]
		}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	builds, err := client.GetRecommendedBuilds(context.Background(), "token", domain.RecommendedBuildsRequest{})
	if err != nil {
		t.Fatalf("GetRecommendedBuilds() error = %v", err)
	}

	build := builds.Builds[0]
	if !slices.Equal(build.Runes.SelectedRunes, []int{8437, 8446, 8444, 8451, 8224, 8233}) {
		t.Fatalf("unexpected selected runes: %#v", build.Runes.SelectedRunes)
	}
	if !slices.Equal(build.Runes.SelectedShards, []int{5008, 5011}) {
		t.Fatalf("unexpected selected shards: %#v", build.Runes.SelectedShards)
	}
	if !slices.Equal(builds.SpellDuos[0].SpellIDs, []int{4}) {
		t.Fatalf("unexpected spell duo ids: %#v", builds.SpellDuos[0].SpellIDs)
	}
}

func TestGetRecommendedBuildsSendsBodyAndParsesResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ChampionWinprob/GetRecommendedBuilds" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		var body apiRecommendedBuildsRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.CommonFilters.ChampionIDs) != 1 || body.CommonFilters.ChampionIDs[0] != 240 {
			t.Fatalf("unexpected body: %#v", body)
		}

		_ = json.NewEncoder(w).Encode(apiRecommendedBuildsResponse{
			Role: 4,
			Builds: []apiRecommendedBuild{{
				Tier:    0,
				Winrate:  52.3,
				Runes: &apiBuildRunes{
					PrimaryTree:    8000,
					SecondaryTree:  8100,
					SelectedRunes:  []int{8010, 8112},
					SelectedShards: []int{5008, 5008, 5011},
				},
				ItemBlocks: []apiBuildBlock{
					{Type: 0, Items: []apiBuildBlockItem{{ItemID: 1055}}},
					{Type: 2, Items: []apiBuildBlockItem{
						{ItemID: 1056},
						{ItemID: 3158, ChooseOne: true},
					}},
				},
			}},
			SpellDuos: []apiSpellDuo{{
				SpellIDs: []int{4, 12},
			}},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, 2*time.Second)
	builds, err := client.GetRecommendedBuilds(context.Background(), "token", domain.RecommendedBuildsRequest{
		CommonFilters: domain.CommonFilters{
			ChampionIDs: []int{240},
			Role:        4,
		},
	})
	if err != nil {
		t.Fatalf("GetRecommendedBuilds() error = %v", err)
	}

	if builds.Role != 4 || len(builds.Builds) != 1 {
		t.Fatalf("unexpected builds: %#v", builds)
	}
	build := builds.Builds[0]
	if build.Tier != 0 || build.Winrate != 52.3 {
		t.Fatalf("unexpected build: %#v", build)
	}
	if build.Runes == nil || build.Runes.PrimaryTree != 8000 || build.Runes.SecondaryTree != 8100 ||
		len(build.Runes.SelectedRunes) != 2 || len(build.Runes.SelectedShards) != 3 {
		t.Fatalf("unexpected runes: %#v", build.Runes)
	}
	if len(build.ItemBlocks) != 2 || build.ItemBlocks[1].Items[1].ItemID != 3158 || !build.ItemBlocks[1].Items[1].ChooseOne {
		t.Fatalf("unexpected item blocks: %#v", build.ItemBlocks)
	}
	if len(builds.SpellDuos) != 1 || builds.SpellDuos[0].SpellIDs[0] != 4 || builds.SpellDuos[0].SpellIDs[1] != 12 {
		t.Fatalf("unexpected spell duos: %#v", builds.SpellDuos)
	}
}
