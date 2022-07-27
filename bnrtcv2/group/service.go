package group

import (
	"errors"
	"fmt"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/device_info"
	"github.com/guabee/bnrtc/bnrtcv2/message"
	"github.com/guabee/bnrtc/buffer"
)

func (manager *GroupManager) storeGroup(item *GroupItem) {
	if !manager.device.IsConnected() {
		manager.logger.Debugf("store group failed, not connected")
		return
	}

	manager.logger.Debugf("store group %s", item.Id)
	groupId := item.Id
	groupInfo, found, err := manager.retriveGroup(groupId, NoCache)
	if err != nil {
		manager.logger.Debugf("store group %s failed retriveGroup %s", groupId, err)
		return
	}
	if !found || groupInfo == nil {
		manager.updateGroupInfo(groupInfo, item)
	} else {
		leader := groupInfo.GetLeader()
		if leader == nil {
			manager.updateGroupInfo(groupInfo, item)
			return
		}
		manager.logger.Infof("set group %s leader %s", groupId, leader.Meta.Id)
		if item.IsLeader(manager.getLocalId()) {
			manager.logger.Debugf("I am group %s leader", groupId)
			// republish the latest
			item.SetLeader(item.LocalInfo.Data)
			manager.setLocalGroupInfo(groupInfo.Id, item.ToGroupInfo())
			manager.updateGroupInfo(groupInfo, item)
			item.SetPending(false)
		} else {
			item.SetLeader(leader)
			manager.setLocalGroupInfo(groupInfo.Id, groupInfo)
			manager.sendToLeader(messageType_memberInfo, item, nil)
		}
	}
}

func (manager *GroupManager) retriveGroup(groupId GroupId, nocache bool) (info *GroupInfo, found bool, err error) {
	if !manager.device.IsConnected() {
		return nil, false, errors.New("not connected")
	}

	bytes, found, err := manager.device.GetData(GetGroupKey(groupId), nocache)
	if err != nil || !found {
		return nil, found, err
	}
	info = GroupInfoFromBytes(bytes)
	if info == nil || info.Id != groupId {
		info = nil
	}
	return info, true, nil
}

func (manager *GroupManager) deleteGroup(item *GroupItem) {
	if !manager.device.IsConnected() {
		return
	}

	groupId := item.Id
	if item.IsLeader(manager.getLocalId()) {
		manager.logger.Infof("leader delete group %s", groupId)
		_ = manager.device.DeleteData(GetGroupKey(groupId))
		manager.sendToMembers(messageType_close, item, nil)
	} else {
		manager.logger.Infof("member delete group %s", groupId)
		manager.sendToLeader(messageType_close, item, nil)
	}
}

func (manager *GroupManager) run() {
	lastCheckInfoTime := time.Now()
	lastCheckLeaderTime := time.Now()
	checkInfo := time.NewTicker(manager.CheckInfoInterval)
	keepalive := time.NewTicker(manager.KeepAliveInterval)
	checkLeader := time.NewTicker(manager.CheckLeaderInterval)

	for {
		select {
		case <-checkInfo.C:
			if time.Since(lastCheckInfoTime) < manager.CheckInfoInterval/2 {
				continue
			}
			manager.updatePendingGroups()
			manager.checkGroupInfoExpired()
			manager.checkRepublish()
			lastCheckInfoTime = time.Now()
		case <-checkLeader.C:
			if time.Since(lastCheckLeaderTime) < manager.CheckLeaderInterval/2 {
				continue
			}
			manager.checkLeaderInfo()
			lastCheckLeaderTime = time.Now()
		case <-keepalive.C:
			manager.iterateGroupItems(func(item *GroupItem) {
				if !item.IsPending() && !item.IsLeader(manager.getLocalId()) {
					manager.sendToLeader(messageType_ping, item, nil)
				}
			})
		case msg := <-manager.msgChan:
			manager.handleRecvMessage(msg)
		case ev := <-manager.eventChan:
			manager.handEvent(ev)
		case <-manager.stopChan:
			close(manager.msgChan)
			checkInfo.Stop()
			keepalive.Stop()
			checkLeader.Stop()
			return
		}
	}
}

func (manager *GroupManager) handEvent(ev *GroupEventData) {
	manager.logger.Debugf("handle event %s", ev.event)
	switch ev.event {
	case GroupEventLocalInfoChange:
		oldInfo := ev.data.(*device_info.DeviceInfo)
		manager.handleLocalInfoChange(oldInfo, manager.getLocalInfo())
	case GroupEventAddGroup:
		groupId := ev.data.(GroupId)
		item := manager.getGroupItem(groupId)
		if item != nil {
			manager.storeGroup(item)
		}
	case GroupEventDeleteGroup:
		item := ev.data.(*GroupItem)
		if item != nil {
			manager.deleteGroup(item)
		}
	case GroupEventNetworkReady:
		manager.updatePendingGroups()
	}
}

func (manager *GroupManager) handleLocalInfoChange(oldInfo, newInfo *device_info.DeviceInfo) {
	manager.iterateGroupItems(func(item *GroupItem) {
		if oldInfo != nil {
			if !item.IsPending() {
				if item.IsLeader(oldInfo.Id) {
					manager.sendToMembers(messageType_close, item, nil)
				} else {
					manager.sendToLeader(messageType_close, item, nil)
				}
			}
		}
		item.LocalInfo.Data.Meta = newInfo
		manager.logger.Infof("update group %s localInfo %s", item.Id, newInfo)
		item.Reset()
	})
}

func (manager *GroupManager) updatePendingGroups() {
	manager.iterateGroupItems(func(item *GroupItem) {
		if item.IsPending() {
			manager.logger.Debugf("update pending group %s", item.Id)
			manager.storeGroup(item)
		} else if !item.IsLeader(manager.getLocalId()) {
			if !item.isAcked {
				manager.sendToLeader(messageType_memberInfo, item, nil)
			}
		}
	})
}

func (manager *GroupManager) checkGroupInfoExpired() {
	manager.iterateGroupItems(func(item *GroupItem) {
		if item.IsLeader(manager.getLocalId()) {
			isDeleted := false

			for _, _memberInfo := range item.Members.Items() {
				member := _memberInfo.(*MemberInfo)
				memberId := member.Data.Meta.Id
				if time.Since(member.timestamp) > manager.KeepAliveTimeout {
					manager.logger.Infof("group %s leader checkGroupInfoExpired member %s timeout", item.Id, memberId)
					item.Members.Delete(memberId.String())
					isDeleted = true
				}
			}
			if isDeleted {
				manager.updateGroupInfo(manager.getLocalGroupInfo(item.Id), item)
			}
		} else {
			if item.Leader != nil && time.Since(item.Leader.timestamp) > manager.KeepAliveTimeout+3*time.Second { // member timeout plus two second
				manager.logger.Infof("group %s member checkGroupInfoExpired leader %s timeout", item.Id, item.GetLeaderId())
				item.SetLeader(nil)
				item.Members.Delete(manager.getLocalId().String())
				manager.updateGroupInfo(manager.getLocalGroupInfo(item.Id), item)
			}
		}
	})
}

func (manager *GroupManager) checkRepublish() {
	manager.iterateGroupItems(func(item *GroupItem) {
		if !item.IsLeader(manager.getLocalId()) {
			return
		}

		groupInfo := manager.getLocalGroupInfo(item.Id)
		if groupInfo == nil {
			manager.updateGroupInfo(groupInfo, item)
		} else {
			if time.Since(time.Unix(groupInfo.Timestamp, 0)) > manager.RepublishInterval {
				manager.logger.Infof("group %s leader republish info", item.Id)
				manager.updateGroupInfo(groupInfo, item)
			}
		}
	})
}

func (manager *GroupManager) checkLeaderInfo() {
	manager.iterateGroupItems(func(item *GroupItem) {
		groupInfo := manager.getLocalGroupInfo(item.Id)
		if groupInfo == nil {
			return
		}

		if !item.IsLeader(manager.getLocalId()) {
			return
		}

		info, found, err := manager.retriveGroup(item.Id, NoCache)
		if err != nil {
			manager.logger.Infof("get group %s leader info failed, reset leader, %s", item.Id, err)
			item.Reset()
		} else {
			if !found || info == nil {
				manager.updateGroupInfo(nil, item)
			} else if !info.GetLeaderId().Equal(groupInfo.GetLeaderId()) {
				manager.logger.Infof("group %s leader changed %s->%s", item.Id, groupInfo.GetLeaderId(), info.GetLeaderId())
				item.Reset()
			}
		}
	})
}

func (manager *GroupManager) handleRecvMessage(msg *GroupMessage) {
	groupId := msg.Id
	memberId := msg.MemberId
	manager.logger.Debugf("handle message %s form %s", groupId, memberId)
	item := manager.getGroupItem(groupId)
	if item == nil {
		manager.logger.Errorf("handle message %s form %s failed no item", groupId, memberId)
		return
	}

	if !item.UpdateMessageSeq(memberId, msg.Seq) {
		manager.logger.Errorf("group %s message from Seq %d less local %d error", groupId, msg.Seq, item.GetMessageSeq(memberId))
		return
	}

	if item.IsLeader(manager.getLocalId()) {
		manager.leaderMessageHandler(item, msg.Type, groupId, memberId, msg.Buffer)
	} else {
		manager.memberMessageHandler(item, msg.Type, groupId, memberId, msg.Buffer)
	}
}

func (manager *GroupManager) sendMessage(mt messageType, groupItem *GroupItem, memberMeta *MemberMeta, buf *buffer.Buffer) error {
	memberId := memberMeta.Id

	if manager.getLocalId().Equal(memberId) {
		return fmt.Errorf("send %s to self", mt)
	}

	groupId := groupItem.Id
	if buf == nil {
		buf = buffer.NewBuffer(0, nil)
	}
	switch mt {
	case messageType_ping:
		info := manager.getLocalGroupInfo(groupId)
		if info == nil {
			return fmt.Errorf("not found group info for %s", groupId)
		}
		buf.PutU64(info.Nonce)
		// no data
	case messageType_pong:
		// no data
	case messageType_memberInfo:
		buf = groupItem.LocalInfo.Data.GetBuffer()
		buf.PutU32(groupItem.Version)
	case messageType_memberInfo_ack:
		version := groupItem.GetMemberVersion(memberId)
		buf.PutU32(version)
	case messageType_groupInfo:
		_info, found := manager.groupInfoMap.Get(groupId)
		if !found {
			return fmt.Errorf("not found group info %s", groupId)
		}
		groupInfo := _info.(*GroupInfo)
		buf = buffer.FromBytes(groupInfo.GetBytes())
	}

	groupMsg := manager.newGroupMessage(mt, groupId, buf)
	msg := message.NewMessage(getGroupMessagePort(), "", "", manager.getLocalId().String(), memberId.String(), false, groupMsg.GetBuffer())
	return manager.device.SendByInfo(memberMeta, msg.GetBuffer())
}
