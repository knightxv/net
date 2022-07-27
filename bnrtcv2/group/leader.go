package group

import (
	"bytes"

	"github.com/guabee/bnrtc/bnrtcv2/dht/idht"
	"github.com/guabee/bnrtc/buffer"
)

func (manager *GroupManager) updateGroupInfo(oldGroupInfo *GroupInfo, item *GroupItem) {
	if item.LocalInfo.Data.IsEmpty() {
		manager.logger.Debugf("local info meta is empty, try later")
		return
	}

	IsLeader := item.IsLeader(manager.getLocalId())
	if IsLeader {
		manager.logger.Debugf("leader updateGroupInfo %s", item.Id)
	} else {
		manager.logger.Debugf("member updateGroupInfo %s", item.Id)
	}

	info := item.ToGroupInfo()
	if info == nil {
		manager.logger.Errorf("build GroupInfo failed")
		return
	}
	infoBytes := info.GetBytes()
	manager.logger.Debugf("updateGroupInfo %s %s", item.Id, info)
	err := manager.device.UpdateData(
		GetGroupKey(info.Id),
		oldGroupInfo.GetBytes(), infoBytes,
		&idht.DHTStoreOptions{
			TExpire:     manager.DataExpireInterval,
			NoReplicate: true,
		},
	)
	if err != nil {
		if IsLeader {
			manager.logger.Debugf("leader update group %s info failed, %s", info.Id, err)
		} else {
			manager.logger.Debugf("member update group %s info failed, %s", info.Id, err)
		}
		item.Reset()
	} else {
		// check by refetch it
		storeInfo, found, err := manager.retriveGroup(item.Id, NoCache)
		if err != nil {
			manager.logger.Debugf("updateGroupInfo %s retrive err %s", item.Id, err)
			return
		}

		if !found {
			manager.logger.Debugf("updateGroupInfo %s not found by retrive", item.Id)
			return
		}

		if !bytes.Equal(storeInfo.GetBytes(), infoBytes) {
			manager.logger.Debugf("updateGroupInfo %s not equal by retrive", item.Id)
			return
		}

		manager.setLocalGroupInfo(info.Id, info)
		if IsLeader {
			manager.sendToMembers(messageType_groupInfo, item, nil)
		}
	}
}

func (manager *GroupManager) leaderMessageHandler(groupItem *GroupItem, mt messageType, groupId GroupId, memberId MemberId, buf *buffer.Buffer) {
	if mt != messageType_ping {
		manager.logger.Debugf("leader handle message group %s member %s type %s", groupId, memberId, mt)
	}
	groupItem.UpdateMemberTime(memberId)
	switch mt {
	case messageType_ping:
		nonce := buf.PullU64()
		if groupItem.Nonce != nonce {
			// nonce mismatch, send groupInfo
			manager.sendToMemberById(messageType_groupInfo, groupItem, memberId, nil)
		} else {
			manager.sendToMemberById(messageType_pong, groupItem, memberId, nil)
		}
	case messageType_memberInfo:
		version := buf.PullU32()
		data := MemberDataFromBuffer(buf)
		if data == nil {
			manager.logger.Errorf("memberInfo message from %s(%s) error: invalid data", groupId, memberId)
			return
		}
		if !data.Id().Equal(memberId) {
			manager.logger.Errorf("memberInfo message from %s(%s<->%s) error", groupId, memberId, data.Id())
			return
		}

		manager.logger.Debugf("memberInfo message from %s(%s) version %d", groupId, memberId, version)
		groupItem.AddMember(data, version)
		manager.sendToMemberById(messageType_memberInfo_ack, groupItem, memberId, nil)
		manager.updateGroupInfo(manager.getLocalGroupInfo(groupId), groupItem)
	case messageType_userdata:
		manager.handerUserData(groupItem, memberId, buf)
	case messageType_close:
		manager.logger.Debugf("close message from %s(%s)", groupId, memberId)
		groupItem.DeleteMember(memberId)
		manager.updateGroupInfo(manager.getLocalGroupInfo(groupId), groupItem)
	default:
		manager.logger.Debugf("messageType %s error ", mt)
	}
}

func (manager *GroupManager) sendToMemberById(mt messageType, groupItem *GroupItem, memberId MemberId, buf *buffer.Buffer) {
	memberInfo := groupItem.GetMember(memberId)
	if memberInfo == nil {
		return
	}

	manager.sendToMember(mt, groupItem, memberInfo.Data.Meta, buf)
}

func (manager *GroupManager) sendToMember(mt messageType, groupItem *GroupItem, memberMeta *MemberMeta, buf *buffer.Buffer) {
	groupId := groupItem.Id
	memberId := memberMeta.Id
	manager.logger.Debugf("group %s leader send message %s to member %s", groupId, mt, memberId)
	err := manager.sendMessage(mt, groupItem, memberMeta, buf)
	if err != nil {
		manager.logger.Debugf("leader send message %s to member %s(%s) failed %s", mt, memberId, groupId, err)
	}
}

func (manager *GroupManager) sendToMembersWithFilter(mt messageType, item *GroupItem, filter func(id MemberId) bool, buf *buffer.Buffer) {
	for _, _member := range item.Members.Items() {
		member := _member.(*MemberInfo)
		if filter != nil && !filter(member.Data.Meta.Id) {
			continue
		}
		manager.sendToMember(mt, item, member.Data.Meta, buf.Clone())
	}
}

func (manager *GroupManager) sendToMembers(mt messageType, item *GroupItem, buf *buffer.Buffer) {
	manager.sendToMembersWithFilter(mt, item, nil, buf)
}
