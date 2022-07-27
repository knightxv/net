package net

import (
	"context"

	"github.com/guabee/bnrtc/util"
)

type Statis struct {
	CountSent     uint64
	CountReceived uint64
	BytesSent     uint64
	BytesReceived uint64
}

type StatisInfo struct {
	IsSend   bool
	DeviceId string
	Bytes    uint64
}

type StatisManager struct {
	statisMap    *util.SafeMap //map[string]*Statis
	onChangeFunc func(deviceId string, statis *Statis, bytes uint64, isSend bool)
	infoChan     chan *StatisInfo
}

func NewStatisManager(ctx context.Context) *StatisManager {
	m := &StatisManager{
		statisMap: util.NewSafeMap(), // make(map[string]*Statis),
		infoChan:  make(chan *StatisInfo, 1024),
	}

	go m.handleStatisInfo(ctx)
	return m
}

func (m *StatisManager) handleStatisInfo(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case info := <-m.infoChan:
			if info.IsSend {
				m.recordSend(info.DeviceId, info.Bytes)
			} else {
				m.recordRecv(info.DeviceId, info.Bytes)
			}
		}
	}
}

func (m *StatisManager) getStatis(deviceid string, createIfNotExist bool) *Statis {
	statis, found := m.statisMap.Get(deviceid)
	if !found {
		if createIfNotExist {
			statis = &Statis{}
			m.statisMap.Set(deviceid, statis)
		} else {
			return nil
		}
	}
	return statis.(*Statis)
}

func (m *StatisManager) RecordSend(deviceId string, bytes uint64) {
	if deviceId == "" {
		return
	}
	m.infoChan <- &StatisInfo{
		IsSend:   true,
		DeviceId: deviceId,
		Bytes:    bytes,
	}
}

func (m *StatisManager) RecordRecv(deviceId string, bytes uint64) {
	if deviceId == "" {
		return
	}
	m.infoChan <- &StatisInfo{
		IsSend:   false,
		DeviceId: deviceId,
		Bytes:    bytes,
	}
}

func (m *StatisManager) recordSend(deviceId string, bytes uint64) {
	s := m.getStatis(deviceId, true)
	s.CountSent++
	s.BytesSent += bytes
	m.onchange(deviceId, s, bytes, true)
}

func (m *StatisManager) recordRecv(deviceId string, bytes uint64) {
	s := m.getStatis(deviceId, true)
	s.CountReceived++
	s.BytesReceived += bytes
	m.onchange(deviceId, s, bytes, false)
}

func (m *StatisManager) GetSendCount(deviceId string) uint64 {
	s := m.GetStatis(deviceId)
	if s == nil {
		return 0
	}
	return s.CountSent
}
func (m *StatisManager) GetRecvCount(deviceId string) uint64 {
	s := m.GetStatis(deviceId)
	if s == nil {
		return 0
	}
	return s.CountReceived
}

func (m *StatisManager) GetSendBytes(deviceId string) uint64 {
	s := m.GetStatis(deviceId)
	if s == nil {
		return 0
	}
	return s.BytesSent
}

func (m *StatisManager) GetRecvBytes(deviceId string) uint64 {
	s := m.GetStatis(deviceId)
	if s == nil {
		return 0
	}
	return s.BytesReceived
}

func (m *StatisManager) GetStatis(deviceId string) *Statis {
	return m.getStatis(deviceId, false)
}

func (m *StatisManager) OnChange(handler func(deviceId string, statis *Statis, bytes uint64, isSend bool)) {
	m.onChangeFunc = handler
}

func (m *StatisManager) onchange(deviceId string, s *Statis, bytes uint64, isSend bool) {
	if m.onChangeFunc != nil {
		m.onChangeFunc(deviceId, s, bytes, isSend)
	}
}

func (m *StatisManager) Clear(deviceId string) {
	m.statisMap.Delete(deviceId)
}

func (m *StatisManager) ClearAll() {
	m.statisMap.Clear()
}
