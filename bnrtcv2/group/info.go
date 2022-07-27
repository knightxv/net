package group

import (
	"encoding/json"
)

type GroupInfo struct {
	Id        GroupId
	Leader    *MemberData
	Members   []*MemberData
	Timestamp int64
	Signature []byte
	Nonce     uint64
}

func GroupInfoFromBytes(bytes []byte) *GroupInfo {
	info := &GroupInfo{}
	err := json.Unmarshal(bytes, info)
	if err != nil {
		return nil
	}

	if !info.Validate() {
		return nil
	}

	return info
}

func (info *GroupInfo) GetBytes() []byte {
	if info == nil {
		return nil
	}

	if !info.Validate() {
		return nil
	}

	bytes, err := json.Marshal(info)
	if err != nil {
		return nil
	}

	return bytes
}

func (Info *GroupInfo) Validate() bool {
	if Info == nil {
		return false
	}

	if Info.Leader == nil || Info.Leader.Meta == nil {
		return false
	}

	for _, member := range Info.Members {
		if member.Meta == nil {
			return false
		}
	}

	return true
}

func (Info *GroupInfo) GetLeader() *MemberData {
	return Info.Leader
}

func (Info *GroupInfo) GetLeaderId() MemberId {
	return Info.Leader.Id()
}

func (Info *GroupInfo) String() string {
	return string(Info.GetBytes())
}
