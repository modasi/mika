package http

import (
	"bytes"
	"github.com/chihaya/bencode"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"mika/model"
	"mika/tracker"
	"mika/util"
	"net"
	"time"
)

// BitTorrentHandler is the public HTTP interface for the tracker handling announces and
// scrape requests
type BitTorrentHandler struct {
	t *tracker.Tracker
}

// Represents an announce received from the bittorrent client
//
// TODO use gin binding func?
type announceRequest struct {
	Compact bool `form:"compact"` // Force compact always?

	// The total amount downloaded (since the client sent the 'started' event to the tracker) in
	// base ten ASCII. While not explicitly stated in the official specification, the consensus is that
	// this should be the total number of bytes downloaded.
	Downloaded uint32 `form:"downloaded" binding:"required"`

	// The number of bytes this peer still has to download, encoded in base ten ascii.
	// Note that this can't be computed from downloaded and the file length since it
	// might be a resume, and there's a chance that some of the downloaded data failed an
	// integrity check and had to be re-downloaded.
	Left uint32 `form:"left" binding:"required"`

	// The total amount uploaded (since the client sent the 'started' event to the tracker) in base ten
	// ASCII. While not explicitly stated in the official specification, the consensus is that this should
	// be the total number of bytes uploaded.
	Uploaded uint32 `form:"uploaded" binding:"required"`

	Corrupt uint32 `form:"corrupt"`

	// This is an optional key which maps to started, completed, or stopped (or empty,
	// which is the same as not being present). If not present, this is one of the
	// announcements done at regular intervals. An announcement using started is sent
	// when a download first begins, and one using completed is sent when the download
	// is complete. No completed is sent if the file was complete when started. Downloaders
	// send an announcement using stopped when they cease downloading.
	Event announceType `form:"event" binding:"required"`

	//  Optional. The true IP address of the client machine, in dotted quad format or rfc3513
	// defined hexed IPv6 address. Notes: In general this parameter is not necessary as the address
	// of the client can be determined from the IP address from which the HTTP request came.
	// The parameter is only needed in the case where the IP address that the request came in on
	// is not the IP address of the client. This happens if the client is communicating to the
	// tracker through a proxy (or a transparent web proxy/cache.) It also is necessary when both the
	// client and the tracker are on the same local side of a NAT gateway. The reason for this is that
	// otherwise the tracker would give out the internal (RFC1918) address of the client, which is not
	// routable. Therefore the client must explicitly state its (external, routable) IP address to be
	// given out to external peers. Various trackers treat this parameter differently. Some only honor
	// it only if the IP address that the request came in on is in RFC1918 space. Others honor it
	// unconditionally, while others ignore it completely. In case of IPv6 address (e.g.: 2001:db8:1:2::100)
	// it indicates only that client can communicate via IPv6.
	IP net.IP `form:"ip" binding:"required"`

	// urlencoded 20-byte SHA1 hash of the value of the info key from the Metainfo file. Note that the
	// value will be a bencoded dictionary, given the definition of the info key above.
	InfoHash model.InfoHash `form:"info_hash" binding:"required"`

	// Optional. Number of peers that the client would like to receive from the tracker. This value is
	// permitted to be zero. If omitted, typically defaults to 50 peers.
	NumWant uint `form:"numwant" `

	// Required for private tracker use. Authentication key to authenticate requests
	Passkey string

	// urlencoded 20-byte string used as a unique ID for the client, generated by the client at startup.
	// This is allowed to be any value, and may be binary data. There are currently no guidelines for
	// generating this peer ID. However, one may rightly presume that it must at least be unique for
	// your local machine, thus should probably incorporate things like process ID and perhaps a timestamp
	// recorded at startup. See peer_id below for common client encodings of this field.
	PeerID model.PeerID `form:"peer_id" binding:"required"`

	// The port number that the client is listening on. Ports reserved for BitTorrent are typically
	// 6881-6889. Clients may choose to give up if it cannot establish a port within this range.
	Port uint16 `binding:"required"`

	// Optional. If a previous announce contained a tracker id, it should be set here.
	TrackerID string `form:"tracker_id"`
}

type announceResponse struct {
	//  (optional) Minimum announce interval. If present clients must not reannounce more frequently than this.
	MinInterval int `bencode:"min interval"`

	Complete int `bencode:"complete"`

	Incomplete int `bencode:"incomplete"`
	// Interval in seconds that the client should wait between sending regular requests to the tracker
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
	//  A string that the client should send back on its next announcements. If absent and a previous
	//  announce sent a tracker id, do not discard the old value; keep using it.
	TrackerID []byte
	// ( optional ) Similar to failure reason, but the response still gets processed normally. The warning message is
	// shown just like an error.
	Warning string `bencode:"warning message"`
}

func getUint64Key(q *query, key announceParam, def uint64) uint64 {
	left, err := q.Uint64(key)
	if err != nil {
		return def
	}
	return util.UMax64(0, left)
}

func getUint32Key(q *query, key announceParam, def uint32) uint32 {
	left, err := q.Uint32key(key)
	if err != nil {
		return def
	}
	return util.UMax32(0, left)
}

func getUint16Key(q *query, key announceParam, def uint16) uint16 {
	left, err := q.Uint16(key)
	if err != nil {
		return def
	}
	return util.UMax16(0, left)
}

func getUintKey(q *query, key announceParam, def uint) uint {
	left, err := q.Uint(key)
	if err != nil {
		return def
	}
	return util.UMax(0, left)
}

// Parse the query string into an announceRequest struct
func newAnnounce(c *gin.Context) (*announceRequest, trackerErrCode) {
	q, err := queryStringParser(c.Request.URL.RawQuery)
	if err != nil {
		return nil, msgMalformedRequest
	}
	infoHash, exists := q.Params[paramInfoHash]
	if !exists {
		return nil, msgInvalidInfoHash
	}
	peerID, exists := q.Params[paramPeerID]
	if !exists {
		return nil, msgInvalidPeerID
	}
	ipv4, err := getIP(q, c)
	if err != nil {

		return nil, msgMalformedRequest
	}
	port := getUint16Key(q, paramPort, 0)
	if port < 1024 || port > 65535 {
		// Don't allow privileged ports which require root to bind to on unix
		return nil, msgInvalidPort
	}
	left := getUint32Key(q, paramLeft, 0)
	downloaded := getUint32Key(q, paramDownloaded, 0)
	uploaded := getUint32Key(q, paramUploaded, 0)
	corrupt := getUint32Key(q, paramCorrupt, 0)
	event := parseAnnounceType(q.Params[paramNumWant])
	numWant := getUintKey(q, "numwant", 30)
	return &announceRequest{
		Compact:    true, // Ignored and always set to true
		Corrupt:    corrupt,
		Downloaded: downloaded,
		Event:      event,
		IP:         ipv4,
		InfoHash:   model.InfoHashFromString(infoHash),
		Left:       left,
		NumWant:    numWant,
		PeerID:     model.PeerIDFromString(peerID),
		Port:       port,
		Uploaded:   uploaded,
	}, msgOk
}

// The meaty bits.
func (h *BitTorrentHandler) announce(c *gin.Context) {
	// Check that the user is valid before parsing anything
	usr, valid := preFlightChecks(c, h.t)
	if !valid {
		return
	}
	// Parse the announce into an announceRequest
	req, code := newAnnounce(c)
	if code != msgOk {
		oops(c, code)
		return
	}
	// Get & Validate the torrent associated with the info_hash supplies
	tor, err := h.t.Torrents.GetTorrent(req.InfoHash)
	if err != nil || tor.IsDeleted {
		oops(c, msgInvalidInfoHash)
		return
	}
	// If disabled and reason is set, the reason is returned to the client
	// This is mostly useful for when a torrent has been "trumped" by another torrent so it
	// should be downloaded instead
	//
	// TODO send this as a "warning message" field of a normal announce response instead?
	if !tor.IsEnabled && tor.Reason != "" {
		c.String(int(msgInvalidInfoHash), responseError(tor.Reason))
		return
	}

	// Peer / Swarm stuff
	peer, err := h.t.Peers.GetPeer(tor.InfoHash, req.PeerID)
	if err != nil {
		// Create a new peer for the swarm
		peer = model.NewPeer(usr.UserID, req.PeerID, req.IP, req.Port)
		if err := h.t.Peers.AddPeer(tor.InfoHash, peer); err != nil {
			log.Errorf("Failed to insert peer into swarm: %s", err.Error())
			oops(c, msgGenericError)
			return
		}
	}
	// TODO use a channel to send deltas instead of locking in-request?
	// Maybe use sync/atomic, but needs testing?
	peer.Lock()
	peer.Uploaded = req.Uploaded
	peer.Downloaded = req.Downloaded
	peer.Announces++
	peer.Left = req.Left
	peer.UpdatedOn = time.Now()
	peer.Unlock()
	switch req.Event {
	case COMPLETED:
		// TODO does a complete event get sent for a torrent when the user only downloads a specific file from the torrent
		// Do we force left=0 for this? Or trust the client?
		tor.TotalCompleted++
	case STOPPED:
		if err := h.t.Peers.DeletePeer(tor.InfoHash, peer); err != nil {
			log.Errorf("Could not remove peer from swarm: %s", err.Error())
			oops(c, msgGenericError)
			return
		}
	}
	peers, err := h.t.Peers.GetPeers(tor.InfoHash, h.t.MaxPeers)
	if err != nil {
		log.Errorf("Could not read peers from swarm: %s", err.Error())
		oops(c, msgGenericError)
		return
	}
	seeders, leechers := peers.Counts()
	dict := bencode.Dict{
		"complete":     seeders,
		"incomplete":   leechers,
		"interval":     h.t.AnnInterval,
		"min interval": h.t.AnnIntervalMin,
	}
	// NOTE we ONLY support compact response formats (binary format) by design even though its
	// technically breaking the protocol specs.
	// There is no reason to support the older less efficient model for private needs
	if peers != nil {
		dict["peers"] = makeCompactPeers(peers, peer.PeerID)
	} else {
		dict["peers"] = []byte{}
	}
	var outBytes bytes.Buffer
	if err := bencode.NewEncoder(&outBytes).Encode(dict); err != nil {
		oops(c, msgGenericError)
		return
	}
	c.String(int(msgOk), outBytes.String())
}

// Generate a compact peer field array containing the byte representations
// of a peers IP+Port appended to each other
func makeCompactPeers(peers model.Swarm, skipID model.PeerID) []byte {
	var buf bytes.Buffer
	for _, peer := range peers {
		if peer.PeerID == skipID {
			// Skip the peers own peer_id
			continue
		}
		buf.Write(peer.IP.To4())
		buf.Write([]byte{byte(peer.Port >> 8), byte(peer.Port & 0xff)})
	}
	return buf.Bytes()
}
