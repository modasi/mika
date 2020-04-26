package model

import (
	"mika/geo"
	"net"
	"time"
)

type PeerID [20]byte

func PeerIDFromString(s string) PeerID {
	var buf [20]byte
	copy(buf[:], s)
	return buf
}

// Peer represents a single unique peer in a swarm
type Peer struct {
	UserPeerId uint32 `db:"user_peer_id" redis:"user_peer_id" json:"user_peer_id"`
	// Current speed up, bytes/sec
	SpeedUP uint32 `db:"speed_up" redis:"speed_up" json:"speed_up"`
	// Current speed dn, bytes/sec
	SpeedDN uint32 `db:"speed_dn" redis:"speed_dn" json:"speed_dn"`
	// Max recorded up speed, bytes/sec
	SpeedUPMax uint32 `db:"speed_up_max"  redis:"speed_up_max" json:"speed_up_max"`
	// Max recorded dn speed, bytes/sec
	SpeedDNMax uint32 `db:"speed_dn_max" redis:"speed_dn_max" json:"speed_dn_max"`
	// Total amount uploaded as reported by client
	Uploaded uint32 `db:"total_uploaded" redis:"total_uploaded" json:"total_uploaded"`
	// Total amount downloaded as reported by client
	Downloaded uint32 `db:"total_downloaded" redis:"total_downloaded" json:"total_downloaded"`
	// Clients reported bytes left of the download
	Left uint32 `db:"total_left" redis:"total_left" json:"total_left"`
	// Total number of announces the peer has made
	Announces uint32 `db:"total_announces" redis:"total_announces" json:"total_announces"`
	// Total active swarm participation time
	TotalTime uint32 `db:"total_time" redis:"total_time" json:"total_time"`
	// Clients IPv4 Address detected automatically, does not use client supplied value
	IP net.IP `db:"addr_ip" redis:"addr_ip" json:"addr_ip"`
	// Clients reported port
	Port uint16 `db:"addr_port" redis:"addr_port" json:"addr_port"`
	// Last announce timestamp
	AnnounceLast int32 `redis:"last_announce" json:"last_announce"`
	// First announce timestamp
	AnnounceFirst int32 `redis:"first_announce" json:"first_announce"`
	// Peer id, reported by client. Must have white-listed prefix
	PeerId    PeerID      `db:"peer_id" redis:"peer_id" json:"peer_id"`
	Location  geo.LatLong `db:"location" redis:"location" json:"location"`
	UserId    uint32      `db:"user_id" redis:"user_id" json:"user_id"`
	CreatedOn time.Time   `db:"created_on" redis:"created_on" json:"created_on"`
	UpdatedOn time.Time   `db:"updated_on" redis:"updated_on" json:"updated_on"`
}

// IsNew checks if the peer is making its first announce request
func (peer *Peer) IsNew() bool {
	return peer.AnnounceLast == 0
}