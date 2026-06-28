package qui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// fakeQui builds an httptest server emulating the slice of the qui API the
// client uses: GET /api/instances and paginated
// GET /api/instances/{id}/torrents. torrentsByInstance maps instance id to
// the full ordered torrent list; the server pages it by limit.
func fakeQui(t *testing.T, instances []Instance, torrentsByInstance map[int][]torrent) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/instances", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(instances)
	})

	mux.HandleFunc("/api/instances/", func(w http.ResponseWriter, r *http.Request) {
		// path: /api/instances/{id}/torrents
		var id int
		if _, err := fmt.Sscanf(r.URL.Path, "/api/instances/%d/torrents", &id); err != nil {
			http.NotFound(w, r)
			return
		}
		all := torrentsByInstance[id]
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page == 0 {
			page = 1
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 100
		}
		start := (page - 1) * limit
		end := start + limit
		if start > len(all) {
			start = len(all)
		}
		if end > len(all) {
			end = len(all)
		}
		json.NewEncoder(w).Encode(torrentsResponse{
			Torrents: all[start:end],
			Total:    len(all),
			Page:     page,
			Limit:    limit,
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAllTorrents_AggregatesAcrossInstances(t *testing.T) {
	instances := []Instance{{ID: 1, Name: "main"}, {ID: 2, Name: "seedbox"}}
	torrents := map[int][]torrent{
		1: {{Hash: "a", Name: "alpha", DlSpeed: 100, State: "downloading"}},
		2: {{Hash: "b", Name: "bravo", DlSpeed: 0, State: "stalledUP"}},
	}
	srv := fakeQui(t, instances, torrents)

	got, err := New(srv.URL).AllTorrents(context.Background())
	if err != nil {
		t.Fatalf("AllTorrents: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 torrents, got %d", len(got))
	}
	// Tagged with their instance name.
	byHash := map[string]string{}
	for _, tr := range got {
		byHash[tr.Hash] = tr.Instance
	}
	if byHash["a"] != "main" || byHash["b"] != "seedbox" {
		t.Fatalf("instance tagging wrong: %#v", byHash)
	}
}

func TestAllTorrents_SortsByDownloadSpeedDesc(t *testing.T) {
	instances := []Instance{{ID: 1, Name: "main"}}
	torrents := map[int][]torrent{
		1: {
			{Hash: "slow", DlSpeed: 10},
			{Hash: "fast", DlSpeed: 1000},
			{Hash: "mid", DlSpeed: 500},
		},
	}
	srv := fakeQui(t, instances, torrents)

	got, err := New(srv.URL).AllTorrents(context.Background())
	if err != nil {
		t.Fatalf("AllTorrents: %v", err)
	}
	order := []string{got[0].Hash, got[1].Hash, got[2].Hash}
	want := []string{"fast", "mid", "slow"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("sort wrong: got %v want %v", order, want)
		}
	}
}

func TestAllTorrents_NormalizesCamelCaseSpeeds(t *testing.T) {
	instances := []Instance{{ID: 1, Name: "main"}}
	torrents := map[int][]torrent{
		1: {{Hash: "a", DlSpeed: 123, UpSpeed: 456, Progress: 0.5, Eta: 60, Size: 999}},
	}
	srv := fakeQui(t, instances, torrents)

	got, err := New(srv.URL).AllTorrents(context.Background())
	if err != nil {
		t.Fatalf("AllTorrents: %v", err)
	}
	tr := got[0]
	if tr.Dlspeed != 123 || tr.Upspeed != 456 || tr.Progress != 0.5 || tr.Eta != 60 || tr.Size != 999 {
		t.Fatalf("normalization wrong: %#v", tr)
	}
}

func TestAllTorrents_PagesThroughAllTorrents(t *testing.T) {
	instances := []Instance{{ID: 1, Name: "main"}}
	var big []torrent
	for i := 0; i < 250; i++ {
		big = append(big, torrent{Hash: fmt.Sprintf("h%d", i), DlSpeed: int64(i)})
	}
	srv := fakeQui(t, instances, map[int][]torrent{1: big})

	got, err := New(srv.URL).AllTorrents(context.Background())
	if err != nil {
		t.Fatalf("AllTorrents: %v", err)
	}
	if len(got) != 250 {
		t.Fatalf("pagination dropped torrents: want 250, got %d", len(got))
	}
}

func TestAllTorrents_ErrorsWhenInstancesUnreachable(t *testing.T) {
	srv := fakeQui(t, nil, nil)
	url := srv.URL
	srv.Close() // force connection failure

	if _, err := New(url).AllTorrents(context.Background()); err == nil {
		t.Fatal("expected error when qui unreachable, got nil")
	}
}
