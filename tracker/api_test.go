package tracker

import (
	"context"
	"fmt"
	"github.com/leighmacdonald/mika/config"
	"github.com/leighmacdonald/mika/consts"
	"github.com/leighmacdonald/mika/model"
	"github.com/leighmacdonald/mika/store"
	"github.com/stretchr/testify/require"
	"net/http"
	"os"
	"testing"
	"time"
)

func newTestAPI() (*Tracker, http.Handler) {
	context.Background()
	opts := NewDefaultOpts()
	tkr, err := New(context.Background(), opts)
	if err != nil {
		os.Exit(1)
	}
	return tkr, NewAPIHandler(tkr)
}

func TestTorrentAdd(t *testing.T) {
	tor0 := store.GenerateTestTorrent()
	tkr, handler := newTestAPI()
	tadd := TorrentAddRequest{
		Name:     tor0.ReleaseName,
		InfoHash: tor0.InfoHash.String(),
		MultiUp:  1.0,
		MultiDn:  -1,
	}
	w := performRequest(handler, "POST", "/torrent", tadd)
	require.Equal(t, 200, w.Code)
	var tor1 model.Torrent
	require.NoError(t, tkr.Torrents.Get(&tor1, tor0.InfoHash))
	require.Equal(t, tadd.Name, tor1.ReleaseName)
	require.Equal(t, tadd.MultiUp, tor1.MultiUp)
	require.Equal(t, float64(0), tor1.MultiDn)
}

func TestTorrentDelete(t *testing.T) {
	tor0 := store.GenerateTestTorrent()
	tkr, handler := newTestAPI()
	require.NoError(t, tkr.Torrents.Add(tor0))
	u := fmt.Sprintf("/torrent/%s", tor0.InfoHash.String())
	w := performRequest(handler, "DELETE", u, nil)
	require.Equal(t, 200, w.Code)
	var tor1 model.Torrent
	require.Error(t, tkr.Torrents.Get(&tor1, tor0.InfoHash))

}

func TestTorrentUpdate(t *testing.T) {
	tor0 := store.GenerateTestTorrent()
	tkr, handler := newTestAPI()
	require.NoError(t, tkr.Torrents.Add(tor0))
	tup := model.TorrentUpdate{
		Keys:        []string{"release_name", "is_deleted", "is_enabled", "reason", "multi_up", "multi_dn"},
		ReleaseName: "new_name",
		IsDeleted:   false,
		IsEnabled:   false,
		Reason:      "reason",
		MultiUp:     2.0,
		MultiDn:     0.5,
	}
	p := fmt.Sprintf("/torrent/%s", tor0.InfoHash.String())
	w := performRequest(handler, "PATCH", p, tup)
	require.Equal(t, 200, w.Code)
	var tor1 model.Torrent
	require.NoError(t, tkr.Torrents.Get(&tor1, tor0.InfoHash))
	require.Equal(t, tup.ReleaseName, tor1.ReleaseName)
	require.Equal(t, tup.IsDeleted, tor1.IsDeleted)
	require.Equal(t, tup.IsEnabled, tor1.IsEnabled)
	require.Equal(t, tup.Reason, tor1.Reason)
	require.Equal(t, tup.MultiUp, tor1.MultiUp)
	require.Equal(t, tup.MultiDn, tor1.MultiDn)

	// Deleted torrents should not be fetchable after update
	w2 := performRequest(handler, "PATCH", p, model.TorrentUpdate{
		Keys:      []string{"is_deleted"},
		IsDeleted: true,
	})
	require.Equal(t, 200, w2.Code)
	var tor2 model.Torrent
	require.Equal(t, consts.ErrInvalidInfoHash, tkr.Torrents.Get(&tor2, tor0.InfoHash))
}

func TestConfigUpdate(t *testing.T) {
	toDuration := func(s string) time.Duration {
		d, err := time.ParseDuration(s)
		if err != nil {
			panic("Invalid duration specified")
		}
		return d
	}
	tkr, handler := newTestAPI()
	args := ConfigUpdateRequest{
		UpdateKeys: []config.Key{
			config.TrackerAnnounceInterval,
			config.TrackerAnnounceIntervalMin,
			config.TrackerReaperInterval,
			config.TrackerBatchUpdateInterval,
			config.TrackerMaxPeers,
			config.TrackerAutoRegister,
			config.TrackerAllowNonRoutable,
			config.GeodbEnabled,
		},
		TrackerAnnounceInterval:    "60s",
		TrackerAnnounceIntervalMin: "30s",
		TrackerReaperInterval:      "30s",
		TrackerBatchUpdateInterval: "10s",
		TrackerMaxPeers:            100,
		TrackerAutoRegister:        true,
		TrackerAllowNonRoutable:    true,
		GeodbEnabled:               true,
	}
	w := performRequest(handler, "PATCH", "/config", args)
	require.Equal(t, 200, w.Code)
	require.Equal(t, toDuration(args.TrackerAnnounceInterval), tkr.AnnInterval)
	require.Equal(t, toDuration(args.TrackerAnnounceIntervalMin), tkr.AnnIntervalMin)
	require.Equal(t, toDuration(args.TrackerReaperInterval), tkr.ReaperInterval)
	require.Equal(t, toDuration(args.TrackerBatchUpdateInterval), tkr.BatchInterval)
	require.Equal(t, args.TrackerMaxPeers, tkr.MaxPeers)
	require.Equal(t, args.TrackerAutoRegister, tkr.AutoRegister)
	require.Equal(t, args.TrackerAllowNonRoutable, tkr.AllowNonRoutable)
	require.Equal(t, args.GeodbEnabled, tkr.GeodbEnabled)
}

func TestMain(m *testing.M) {
	_ = config.Read("")
	retVal := m.Run()
	os.Exit(retVal)
}
