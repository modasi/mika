package redis

import (
	"fmt"
	"github.com/go-redis/redis/v7"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"mika/config"
	"mika/consts"
	"mika/geo"
	"mika/model"
	"mika/store"
	"mika/util"
	"net"
	"strconv"
	"sync"
)

const (
	driverName = "redis"
	clientName = "mika"
)

// TorrentStore is the redis backed store.TorrentStore implementation
type TorrentStore struct {
	client *redis.Client
}

func torrentKey(t model.InfoHash) string {
	return fmt.Sprintf("t:%s", t.String())
}

func torrentPeerPrefix(t model.InfoHash) string {
	return fmt.Sprintf("p:%s:*", t.String())
}

func peerKey(t model.InfoHash, p model.PeerID) string {
	return fmt.Sprintf("p:%s:%s", t.String(), p.String())
}

// AddTorrent adds a new torrent to the redis backing store
func (ts *TorrentStore) AddTorrent(t *model.Torrent) error {
	err := ts.client.HSet(torrentKey(t.InfoHash), map[string]interface{}{
		"torrent_id":       t.TorrentID,
		"release_name":     t.ReleaseName,
		"total_completed":  t.TotalCompleted,
		"total_downloaded": t.TotalDownloaded,
		"total_uploaded":   t.TotalUploaded,
		"reason":           t.Reason,
		"multi_up":         t.MultiUp,
		"multi_dn":         t.MultiDn,
		"info_hash":        t.InfoHash.RawString(),
		"is_deleted":       t.IsDeleted,
		"is_enabled":       t.IsEnabled,
		"created_on":       util.TimeToString(t.CreatedOn),
		"updated_on":       util.TimeToString(t.UpdatedOn),
	}).Err()
	if err != nil {
		return err
	}
	return nil
}

// DeleteTorrent will mark a torrent as deleted in the backing store.
// If dropRow is true, it will permanently remove the torrent from the store
func (ts *TorrentStore) DeleteTorrent(t *model.Torrent, dropRow bool) error {
	if dropRow {
		return ts.client.Del(torrentKey(t.InfoHash)).Err()
	}
	return ts.client.HSet(torrentKey(t.InfoHash), "is_deleted", 1).Err()
}

// GetTorrent returns the Torrent matching the infohash
func (ts *TorrentStore) GetTorrent(hash model.InfoHash) (*model.Torrent, error) {
	v, err := ts.client.HGetAll(torrentKey(hash)).Result()
	if err != nil {
		return nil, err
	}
	_, found := v["info_hash"]
	if !found {
		return nil, consts.ErrInvalidInfoHash
	}
	t := model.Torrent{
		RWMutex:         sync.RWMutex{},
		TorrentID:       util.StringToUInt32(v["torrent_id"], 0),
		ReleaseName:     v["release_name"],
		InfoHash:        model.InfoHashFromString(v["info_hash"]),
		TotalCompleted:  util.StringToInt16(v["total_completed"], 0),
		TotalUploaded:   util.StringToUInt32(v["total_uploaded"], 0),
		TotalDownloaded: util.StringToUInt32(v["total_downloaded"], 0),
		IsDeleted:       util.StringToBool(v["is_deleted"], false),
		IsEnabled:       util.StringToBool(v["is_enabled"], false),
		Reason:          v["reason"],
		MultiUp:         util.StringToFloat64(v["multi_up"], 1.0),
		MultiDn:         util.StringToFloat64(v["multi_dn"], 1.0),
		CreatedOn:       util.StringToTime(v["created_on"]),
		UpdatedOn:       util.StringToTime(v["updated_on"]),
	}
	return &t, nil
}

// Close will close the underlying redis client and clear the caches
func (ts *TorrentStore) Close() error {
	return ts.client.Close()
}

// PeerStore is the redis backed store.PeerStore implementation
type PeerStore struct {
	client *redis.Client
}

// AddPeer inserts a peer into the active swarm for the torrent provided
func (ps *PeerStore) AddPeer(ih model.InfoHash, p *model.Peer) error {
	err := ps.client.HSet(peerKey(ih, p.PeerID), map[string]interface{}{
		"user_peer_id":     p.UserPeerID,
		"speed_up":         p.SpeedUP,
		"speed_dn":         p.SpeedDN,
		"speed_up_max":     p.SpeedUPMax,
		"speed_dn_max":     p.SpeedDNMax,
		"total_uploaded":   p.Uploaded,
		"total_downloaded": p.Downloaded,
		"total_left":       p.Left,
		"total_announces":  p.Announces,
		"total_time":       p.TotalTime,
		"addr_ip":          p.IP.String(),
		"addr_port":        p.Port,
		"last_announce":    util.TimeToString(p.AnnounceLast),
		"first_announce":   util.TimeToString(p.AnnounceFirst),
		"peer_id":          p.PeerID.RawString(),
		"location":         p.Location.String(),
		"user_id":          p.UserID,
		"created_on":       util.TimeToString(p.CreatedOn),
		"updated_on":       util.TimeToString(p.UpdatedOn),
	}).Err()
	if err != nil {
		return errors.Wrap(err, "Failed to AddPeer")
	}
	return nil
}

func (ps *PeerStore) findKeys(prefix string) []string {
	v, err := ps.client.Keys(prefix).Result()
	if err != nil {
		log.Errorf("Failed to query for key prefix: %s", err.Error())
	}
	return v
}

// UpdatePeer will sync any new peer data with the backing store
func (ps *PeerStore) UpdatePeer(ih model.InfoHash, p *model.Peer) error {
	err := ps.client.HSet(peerKey(ih, p.PeerID), map[string]interface{}{
		"speed_up":         p.SpeedUP,
		"speed_dn":         p.SpeedDN,
		"speed_up_max":     p.SpeedUPMax,
		"speed_dn_max":     p.SpeedDNMax,
		"total_uploaded":   p.Uploaded,
		"total_downloaded": p.Downloaded,
		"total_left":       p.Left,
		"total_announces":  p.Announces,
		"total_time":       p.TotalTime,
		"last_announce":    util.TimeToString(p.AnnounceLast),
		"first_announce":   util.TimeToString(p.AnnounceFirst),
		"updated_on":       util.TimeToString(p.UpdatedOn),
	}).Err()
	if err != nil {
		return errors.Wrap(err, "Failed to UpdatePeer")
	}
	return nil
}

// DeletePeer will remove a user from a torrents swarm
func (ps *PeerStore) DeletePeer(ih model.InfoHash, p *model.Peer) error {
	return ps.client.Del(peerKey(ih, p.PeerID)).Err()
}

func (ps *PeerStore) GetPeer(_ model.InfoHash, _ model.PeerID) (*model.Peer, error) {
	panic("implement me")
}

// GetPeers will fetch peers for a torrents active swarm up to N users
func (ps *PeerStore) GetPeers(ih model.InfoHash, limit int) (model.Swarm, error) {
	var peers []*model.Peer
	for i, key := range ps.findKeys(torrentPeerPrefix(ih)) {
		if i == limit {
			break
		}
		v, err := ps.client.HGetAll(key).Result()
		if err != nil {
			return nil, errors.Wrap(err, "Error trying to GetPeers")
		}
		p := &model.Peer{
			UserPeerID:    util.StringToUInt32(v["user_peer_id"], 0),
			SpeedUP:       util.StringToUInt32(v["speed_up"], 0),
			SpeedDN:       util.StringToUInt32(v["speed_dn"], 0),
			SpeedUPMax:    util.StringToUInt32(v["speed_dn_max"], 0),
			SpeedDNMax:    util.StringToUInt32(v["speed_up_max"], 0),
			Uploaded:      util.StringToUInt32(v["total_uploaded"], 0),
			Downloaded:    util.StringToUInt32(v["total_downloaded"], 0),
			Left:          util.StringToUInt32(v["total_left"], 0),
			Announces:     util.StringToUInt32(v["total_announces"], 0),
			TotalTime:     util.StringToUInt32(v["total_time"], 0),
			IP:            net.ParseIP(v["addr_ip"]),
			Port:          util.StringToUInt16(v["addr_port"], 0),
			AnnounceLast:  util.StringToTime(v["last_announce"]),
			AnnounceFirst: util.StringToTime(v["first_announce"]),
			PeerID:        model.PeerIDFromString(v["peer_id"]),
			Location:      geo.LatLongFromString(v["location"]),
			UserID:        util.StringToUInt32(v["user_id"], 0),
			CreatedOn:     util.StringToTime(v["created_on"]),
			UpdatedOn:     util.StringToTime(v["updated_on"]),
		}
		peers = append(peers, p)
	}
	return peers, nil
}

// GetScrape returns scrape data for the torrent provided
func (ps *PeerStore) GetScrape(_ model.InfoHash) {
	panic("implement me")
}

// Close will close the underlying redis client and clear in-memory caches
func (ps *PeerStore) Close() error {
	return ps.client.Close()
}

func newRedisConfig(c *config.StoreConfig) *redis.Options {
	database, err := strconv.ParseInt(c.Database, 10, 32)
	if err != nil {
		log.Panicf("Failed to parse redis database integer: %s", c.Database)
	}
	return &redis.Options{
		Addr:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Password: c.Password,
		DB:       int(database),
		OnConnect: func(conn *redis.Conn) error {
			if err := conn.ClientSetName(clientName).Err(); err != nil {
				log.Fatalf("Could not setname, bailing: %s", err)
			}
			return nil
		},
	}
}

type torrentDriver struct{}

// NewTorrentStore initialize a TorrentStore implementation using the redis backing store
func (td torrentDriver) NewTorrentStore(cfg interface{}) (store.TorrentStore, error) {
	c, ok := cfg.(*config.StoreConfig)
	if !ok {
		return nil, consts.ErrInvalidConfig
	}
	client := redis.NewClient(newRedisConfig(c))
	return &TorrentStore{
		client: client,
	}, nil
}

type peerDriver struct{}

// NewPeerStore initialize a NewPeerStore implementation using the redis backing store
func (pd peerDriver) NewPeerStore(cfg interface{}) (store.PeerStore, error) {
	c, ok := cfg.(*config.StoreConfig)
	if !ok {
		return nil, consts.ErrInvalidConfig
	}
	client := redis.NewClient(newRedisConfig(c))
	return &PeerStore{
		client: client,
	}, nil
}

func init() {
	store.AddPeerDriver(driverName, peerDriver{})
	store.AddTorrentDriver(driverName, torrentDriver{})
}
