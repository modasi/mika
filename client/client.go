package client

import (
	"fmt"
	"github.com/leighmacdonald/mika/consts"
	"github.com/leighmacdonald/mika/store"
	"github.com/leighmacdonald/mika/tracker"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"time"
)

// Client is the API client implementation
type Client struct {
	*AuthedClient
}

// New initializes an API client for the specified host
func New(host string, authKey string) *Client {
	ac := NewAuthedClient(authKey, host)
	return &Client{ac}
}

// TorrentDelete will delete the torrent matching the info_hash provided
func (c *Client) TorrentDelete(ih store.InfoHash) error {
	_, err := c.Exec(Opts{
		Method: "DELETE",
		Path:   fmt.Sprintf("/torrent/%s", ih.String()),
	})
	return err
}

// TorrentAdd add a new info_hash and associated name to be tracked
func (c *Client) TorrentAdd(ih store.InfoHash, name string) error {
	resp, err := c.Exec(Opts{
		Method: "POST",
		Path:   "/torrent",
		JSON: tracker.TorrentAddRequest{
			InfoHash: ih.String(),
			Name:     name,
		},
	})
	if err != nil && resp != nil {
		if resp.StatusCode == 409 {
			return consts.ErrDuplicate
		}
	}
	return err
}

// UserDelete deletes the user matching the passkey provided
func (c *Client) UserDelete(passkey string) error {
	_, err := c.Exec(Opts{
		Method: "DELETE",
		Path:   fmt.Sprintf("/user/pk/%s", passkey),
	})
	return err
}

// UserAdd creates a new user with the passkey provided
func (c *Client) UserAdd(user store.User) error {
	if !user.Valid() {
		return errors.New("Invalid user model")
	}
	_, err := c.Exec(Opts{
		Method: "POST",
		Path:   "/user",
		JSON:   user,
	})
	return err
}

// Ping tests communication between the API server and the client
func (c *Client) Ping() error {
	const msg = "hello world"
	var pong tracker.PingResponse
	t0 := time.Now()
	_, err := c.Exec(Opts{
		Method: "POST",
		Path:   "/ping",
		JSON:   tracker.PingRequest{Ping: msg},
		Recv:   &pong,
	})
	if err != nil {
		return err
	}
	if pong.Pong != msg {
		return errors.New("invalid response to message")
	}
	log.Debugf("Ping successful: %s", time.Since(t0).String())
	return nil
}
