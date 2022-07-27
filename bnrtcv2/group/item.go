package group

import (
	"math/rand"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/buffer"
	"github.com/guabee/bnrtc/util"
)

type GroupId = string
type MemberId = deviceid.DeviceId
type MemberMeta = device_info.DeviceInfo

var EmptyMemberMeta MemberMeta = MemberMeta{Id: deviceid.EmptyDeviceId}

type MemberData struct {
	Meta     *MemberMeta
	UserData []byte
}

func IsEmptyMemberMeta(meta *MemberMeta) bool {
	return meta.Id.Equal(EmptyMemberMeta.Id)
}

func NewMemberData(meta *MemberMeta, data []byte) *MemberData {
	return &MemberData{
		Meta:     meta,
		UserData: data,
	}
}
func MemberDataFromBuffer(buffer *buffer.Buffer) *MemberData {
	info, err := buffer.PullWithError()
	if err != nil {
		return nil
	}
	meta := device_info.FromBytes(info)
	if meta == nil || IsEmptyMemberMeta(meta) {
		return nil
	}

	return &MemberData{
		Meta:     meta,
		UserData: buffer.Data(),
	}
}

func (m *MemberData) Id() MemberId {
	if m.Meta == nil {
		return EmptyMemberMeta.Id
	}
	return m.Meta.Id
}

func (m *MemberData) IsEmpty() bool {
	return IsEmptyMemberMeta(m.Meta)
}

func (m *MemberData) GetMeta() *MemberMeta {
	return m.Meta
}

func (m *MemberData) GetUserData() []byte {
	return m.UserData
}

func (m *MemberData) GetBuffer() *buffer.Buffer {
	buf := buffer.FromBytes(m.UserData)
	buf.Put(m.Meta.GetBytes())
	return buf
}

func (m *MemberData) GetBytes() []byte {
	return m.GetBuffer().Data()
}

type MemberInfo struct {
	Seq       uint64
	Version   uint32
	Data      *MemberData
	timestamp time.Time
}

func MemberInfoFromBuffer(buffer *buffer.Buffer) *MemberInfo {
	seq := buffer.PullU64()
	version := buffer.PullU32()
	data := MemberDataFromBuffer(buffer)
	if data == nil {
		return nil
	}
	return &MemberInfo{
		Seq:       seq,
		Version:   version,
		Data:      data,
		timestamp: time.Now(),
	}
}

func (m *MemberInfo) Id() MemberId {
	return m.Data.Id()
}

func (m *MemberInfo) GetBuffer() *buffer.Buffer {
	buf := m.Data.GetBuffer()
	buf.PutU32(m.Version)
	buf.PutU64(m.Seq)
	return buf
}

func (m *MemberInfo) GetBytes() []byte {
	return m.GetBuffer().Data()
}

type GroupItemOption func(*GroupItem)

func WithGroupRepublishTime(t time.Duration) GroupItemOption {
	return func(i *GroupItem) {
		if t > 0 {
			i.RepublishInterval = t
		}
	}
}

func WithGroupDataExpireTime(t time.Duration) GroupItemOption {
	return func(i *GroupItem) {
		if t > 0 {
			i.DataExpireInterval = t
		}
	}
}

type GroupItem struct {
	Id                 GroupId
	Leader             *MemberInfo
	Members            *util.SafeMap // map[string]*MemberInfo
	LocalInfo          *MemberInfo
	RepublishInterval  time.Duration
	DataExpireInterval time.Duration
	UserDataHandler    func(groupId GroupId, buf *buffer.Buffer)
	isAcked            bool
	isPending          bool
	Version            uint32
	Nonce              uint64
}

func newGroupItem(groupId GroupId, data *MemberData, options ...GroupItemOption) *GroupItem {
	item := &GroupItem{
		Id: groupId,
		LocalInfo: &MemberInfo{
			Data:      data,
			timestamp: time.Now(),
		},
		Members:   util.NewSafeMap(),
		isPending: true,
	}
	for _, option := range options {
		option(item)
	}
	item.Renonce()
	return item
}

func (item *GroupItem) Renonce() {
	item.Nonce = rand.Uint64()
}

func (item *GroupItem) Reset() {
	item.Leader = nil
	item.Members.Clear()
	item.isPending = true
	item.Renonce()
}

func (item *GroupItem) ToGroupInfo() *GroupInfo {
	if item.LocalInfo == nil && item.Leader == nil {
		return nil
	}

	var leader *MemberInfo
	if item.Leader != nil {
		leader = item.Leader
	} else {
		leader = item.LocalInfo
	}

	info := &GroupInfo{
		Id:        item.Id,
		Leader:    leader.Data,
		Timestamp: time.Now().Unix(),
		Signature: []byte(""),
		Members:   []*MemberData{},
		Nonce:     item.Nonce,
	}
	for _, _member := range item.Members.Items() {
		member := _member.(*MemberInfo)
		info.Members = append(info.Members, member.Data)
	}

	if !info.Validate() {
		return nil
	}

	return info
}

func (item *GroupItem) SetPending(pending bool) {
	item.isPending = pending
}

func (item *GroupItem) IsPending() bool {
	return item.isPending
}

func (item *GroupItem) SetLeader(data *MemberData) {
	if data == nil {
		item.Leader = nil
		item.isPending = true
	} else {
		if item.Leader == nil {
			item.Leader = &MemberInfo{
				Data:      data,
				timestamp: time.Now(),
			}
		} else {
			item.Leader.Data = data
		}
	}
	item.Renonce()
}

func (item *GroupItem) GetLeader() *MemberInfo {
	return item.Leader
}

func (item *GroupItem) GetLeaderId() MemberId {
	if item.Leader == nil {
		return EmptyMemberMeta.Id
	}
	return item.Leader.Id()
}

func (item *GroupItem) IsLeader(id MemberId) bool {
	if id.Equal(EmptyMemberMeta.Id) {
		return false
	}
	return id.Equal(item.GetLeaderId())
}

func (item *GroupItem) UpdateLeaderTime() {
	if item.Leader != nil {
		item.Leader.timestamp = time.Now()
	}
}

func (item *GroupItem) AddMember(data *MemberData, version uint32) {
	if data == nil {
		return
	}

	item.Members.Set(data.Meta.Id.String(), &MemberInfo{
		Data:      data,
		timestamp: time.Now(),
		Version:   version,
	})
	item.Renonce()
}

func (item *GroupItem) UpdateMemberTime(id MemberId) {
	member := item.GetMember(id)
	if member != nil {
		member.timestamp = time.Now()
	}
}

func (item *GroupItem) GetMemberVersion(id MemberId) uint32 {
	member := item.GetMember(id)
	if member != nil {
		return member.Version
	}
	return 0
}

func (item *GroupItem) UpdateMemberVersion(id MemberId, version uint32) {
	member := item.GetMember(id)
	if member != nil {
		member.Version = version
	}
}

func (item *GroupItem) DeleteMember(id MemberId) {
	item.Members.Delete(id.String())
	item.Renonce()
}

func (item *GroupItem) GetMember(id MemberId) *MemberInfo {
	member, found := item.Members.Get(id.String())
	if found {
		return member.(*MemberInfo)
	}
	return nil
}

func (item *GroupItem) HasMember(id MemberId) bool {
	_, found := item.Members.Get(id.String())
	return found
}

func (item *GroupItem) GetMembers() []*MemberInfo {
	members := make([]*MemberInfo, 0)
	for _, member := range item.Members.Items() {
		members = append(members, member.(*MemberInfo))
	}

	return members
}

func (item *GroupItem) UpdateMessageSeq(id MemberId, seq uint64) bool {
	info := (*MemberInfo)(nil)
	if item.IsLeader(id) {
		info = item.GetLeader()
	} else {
		info = item.GetMember(id)
	}

	if info == nil {
		return true
	}

	if info.Seq >= seq {
		return false
	}

	info.Seq = seq
	return true
}

func (item *GroupItem) GetMessageSeq(id MemberId) uint64 {
	info := (*MemberInfo)(nil)
	if item.IsLeader(id) {
		info = item.GetLeader()
	} else {
		info = item.GetMember(id)
	}

	if info == nil {
		return 0
	}

	return info.Seq
}
