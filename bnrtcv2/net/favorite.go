package net

import (
	"context"
	"sync"

	"github.com/guabee/bnrtc/bnrtcv2/transport"
)

type Favorite struct {
	Address string
	Weight  int
}

func NewFavorite(address string, weight int) *Favorite {
	return &Favorite{
		Address: address,
		Weight:  weight,
	}
}

type FavoriteManager struct {
	DirectChannelManager *DirectChannelManager
	StatisManager        *StatisManager
	Favorites            map[string]*Favorite
	mt                   sync.Mutex
}

func NewFavoriteManager(ctx context.Context, device IDevice, connecter IConnectionManager) *FavoriteManager {
	statisManager := NewStatisManager(ctx)
	return &FavoriteManager{
		StatisManager: statisManager,
		DirectChannelManager: NewDirectChannelManager(ctx, &DirectChannelManagerOption{
			StatisManager:     statisManager,
			ConnectionManager: connecter,
			Device:            device,
		}),
		Favorites: make(map[string]*Favorite),
	}
}

func (m *FavoriteManager) AddFavorite(address string) {
	if address != "" {
		m.mt.Lock()
		m.Favorites[address] = NewFavorite(address, 0)
		m.mt.Unlock()
	}
}

func (m *FavoriteManager) DelFavorite(address string) {
	m.mt.Lock()
	delete(m.Favorites, address)
	m.mt.Unlock()
}

func (m *FavoriteManager) HasFavorite(address string) bool {
	m.mt.Lock()
	_, ok := m.Favorites[address]
	m.mt.Unlock()
	return ok
}

func (m *FavoriteManager) GetFavorites() []string {
	addresses := make([]string, 0)
	m.mt.Lock()
	for address := range m.Favorites {
		addresses = append(addresses, address)
	}
	m.mt.Unlock()
	return addresses
}

func (m *FavoriteManager) RecordSend(devid string, size uint64) {
	m.StatisManager.RecordSend(devid, size)
}

func (m *FavoriteManager) RecordRecv(devid string, size uint64) {
	m.StatisManager.RecordRecv(devid, size)
}

func (m *FavoriteManager) CheckChannel(ta *transport.TransportAddress, address string) error {
	return m.DirectChannelManager.CheckChannel(ta, m.HasFavorite(address))
}
