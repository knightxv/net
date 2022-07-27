package group

import (
	"github.com/guabee/bnrtc/buffer"
)

func (manager *GroupManager) sendToLeader(mt messageType, item *GroupItem, buf *buffer.Buffer) {
	leader := item.GetLeader()
	if leader == nil {
		manager.logger.Debugf("group %s sendToLeader, leader is nil", item.Id)
		return
	}

	manager.logger.Debugf("group %s sendToLeader %s", item.Id, leader.Id())
	err := manager.sendMessage(mt, item, leader.Data.Meta, buf)
	if err != nil {
		manager.logger.Debugf("group %s sendToLeader %s send message error %s", item.Id, leader.Data.Meta.Id, err.Error())
	}
}

func (manager *GroupManager) memberMessageHandler(groupItem *GroupItem, mt messageType, groupId GroupId, leaderId MemberId, buf *buffer.Buffer) {
	if mt != messageType_pong {
		manager.logger.Debugf("group %s handle member message from %s Type %s", groupId, leaderId, mt)
	}

	groupItem.UpdateLeaderTime()
	switch mt {
	case messageType_pong:
		// do nothing
	case messageType_memberInfo_ack:
		version := buf.PullU32()
		if version == groupItem.Version {
			manager.logger.Debugf("memberInfo %s is acked", leaderId)
			groupItem.SetPending(false)
			groupItem.isAcked = true
		}
	case messageType_groupInfo:
		info := GroupInfoFromBytes(buf.Data())
		if info == nil || !info.GetLeaderId().Equal(leaderId) {
			manager.logger.Errorf("groupInfo message from %s(%s) not leader", leaderId, groupId)
			return
		}
		manager.logger.Debugf("got groupInfo %s leader %s", groupId, leaderId)
		groupItem.SetLeader(info.GetLeader())
		manager.setLocalGroupInfo(info.Id, info)
	case messageType_userdata:
		manager.handerUserData(groupItem, leaderId, buf)
	case messageType_close:
		manager.logger.Debugf("got group %s close message from %s", groupId, leaderId)
		groupItem.SetLeader(nil)
		manager.updateGroupInfo(nil, groupItem)
	default:
		manager.logger.Debugf("messageType %s error ", mt)
	}
}
