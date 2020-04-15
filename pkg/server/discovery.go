package server

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/grandcat/zeroconf"
	log "github.com/sirupsen/logrus"
)

const domain = "local."
const srvName = "_cryptochat._tcp"
const interval = 3 * time.Second
const browseTime = 500 * time.Millisecond

var roomRegex = regexp.MustCompile(`^room=(.+)$`)

// RoomMember represents a member of a room
type RoomMember struct {
	UUID uuid.UUID
	Addr net.TCPAddr
}

// Discovery represents a CryptoChat discovery server / client
type Discovery struct {
	id uuid.UUID

	roomsLock  sync.RWMutex
	rooms      map[string][]RoomMember
	membership []string

	server *zeroconf.Server
	quit   chan struct{}
}

// NewDiscovery creates a new discovery server / client
func NewDiscovery(id uuid.UUID) Discovery {
	return Discovery{
		id:         id,
		rooms:      make(map[string][]RoomMember),
		membership: []string{},
	}
}

func (d *Discovery) addEntry(e *zeroconf.ServiceEntry) {
	id, err := uuid.Parse(e.Instance)
	if err != nil {
		log.WithField("uuid", e.Instance).Debug("Failed to parse discovered UUID")
		return
	}

	if id == d.id {
		return
	}

	member := RoomMember{
		UUID: id,
		Addr: net.TCPAddr{
			IP:   e.AddrIPv4[0],
			Port: e.Port,
		},
	}

	for _, r := range e.Text {
		m := roomRegex.FindStringSubmatch(r)
		if len(m) == 0 {
			continue
		}

		room := m[1]

		d.roomsLock.Lock()
		if _, ok := d.rooms[room]; !ok {
			d.rooms[room] = []RoomMember{}
		}

		members := d.rooms[room]
		found := false
		for i, m := range members {
			if m.UUID == member.UUID {
				members[i] = member
				found = true
				break
			}
		}
		if !found {
			d.rooms[room] = append(d.rooms[room], member)
		}
		d.roomsLock.Unlock()
	}
}

// Start starts discovery server and client
func (d *Discovery) Start(apiPort int) error {
	var err error

	d.server, err = zeroconf.Register(d.id.String(), srvName, domain, apiPort, []string{}, nil)
	if err != nil {
		return fmt.Errorf("failed to create DNS-SD server: %w", err)
	}

	t := time.NewTicker(interval)

	for {
		select {
		case <-t.C:
			resolver, err := zeroconf.NewResolver()
			if err != nil {
				return fmt.Errorf("failed to create DNS-SD resolver: %w", err)
			}

			entries := make(chan *zeroconf.ServiceEntry)
			go func() {
				for e := range entries {
					d.addEntry(e)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), browseTime)
			if err := resolver.Browse(ctx, srvName, domain, entries); err != nil {
				cancel()
				return fmt.Errorf("failed to start browsing for DNS-SD services: %w", err)
			}

			<-ctx.Done()
			cancel()
		case <-d.quit:
			t.Stop()
			return nil
		}
	}
}

// Close shuts down the discover server / client
func (d *Discovery) Close() error {
	close(d.quit)
	d.server.Shutdown()

	return nil
}

func (d *Discovery) updateTXTs() {
	d.roomsLock.RLock()
	defer d.roomsLock.RUnlock()

	txts := make([]string, len(d.membership))
	for i, r := range d.membership {
		txts[i] = "room=" + r
	}
	d.server.SetText(txts)
}

// AddRoom adds a room to the list of rooms this user is in
func (d *Discovery) AddRoom(room string) bool {
	d.roomsLock.Lock()

	for _, r := range d.membership {
		if r == room {
			d.roomsLock.Unlock()
			return false
		}
	}

	d.membership = append(d.membership, room)
	d.roomsLock.Unlock()

	d.updateTXTs()
	return true
}

// RemoveRoom removes a room from the list of rooms this user is in
func (d *Discovery) RemoveRoom(room string) bool {
	d.roomsLock.Lock()

	for i, r := range d.membership {
		if r == room {
			e := len(d.membership) - 1
			d.membership[e], d.membership[i] = d.membership[i], d.membership[e]
			d.membership = d.membership[:e]
			d.roomsLock.Unlock()

			d.updateTXTs()
			return true
		}
	}

	d.roomsLock.Unlock()
	return false
}

// GetRooms retrieves a map of rooms and their members
func (d *Discovery) GetRooms() map[string][]RoomMember {
	d.roomsLock.RLock()
	defer d.roomsLock.RUnlock()

	rooms := make(map[string][]RoomMember)
	for r, ms := range d.rooms {
		members := make([]RoomMember, len(ms))
		for i, m := range ms {
			members[i] = m
		}

		rooms[r] = members
	}

	return rooms
}

// IsMember checks if this user is a member of a room
func (d *Discovery) IsMember(room string) bool {
	d.roomsLock.RLock()
	defer d.roomsLock.RUnlock()

	for _, r := range d.membership {
		if r == room {
			return true
		}
	}

	return false
}
