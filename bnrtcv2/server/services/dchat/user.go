package dchat

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/guabee/bnrtc/bnrtcv2/store"
	"github.com/guabee/bnrtc/util"
)

type User struct {
	Address   string              `json:"address"`
	EndPoints map[string]struct{} `json:"endpoints"`
	Friends   map[string]struct{} `json:"friends"`
	LastTime  time.Time           `json:"last_time"`
	storeKey  string              `json:"-"`
	store     store.IStore        `json:"-"`
}

func NewUser(address string, store store.IStore) *User {
	return &User{
		Address:   address,
		EndPoints: make(map[string]struct{}),
		Friends:   make(map[string]struct{}),
		LastTime:  time.Now(),
		storeKey:  fmt.Sprintf("%s%s", DchatServiceStoreOnlineUserPrefix, address),
		store:     store,
	}
}

func (u User) IsExpired() bool {
	return time.Since(u.LastTime) > DchatServiceAddressExpireInterval
}

func (u *User) updateLastTime() {
	u.LastTime = time.Now()
}

func (u *User) IsEmpty() bool {
	return len(u.EndPoints) != 0
}

func (u *User) HasEndPoint(devid string) bool {
	_, ok := u.EndPoints[devid]
	return ok
}

func (u *User) AddEndPoint(devid string) {
	_, ok := u.EndPoints[devid]
	if !ok {
		u.EndPoints[devid] = struct{}{}
	}
	u.updateLastTime()
	u.save()
}

func (u *User) DelEndPoint(devid string) {
	delete(u.EndPoints, devid)
	u.updateLastTime()
	if len(u.EndPoints) == 0 {
		u.unsave()
	} else {
		u.save()
	}
}

func (u *User) ClearEndPoints() {
	u.EndPoints = nil
	u.unsave()
}

func (u *User) AddFriends(friends []string) {
	for _, friend := range friends {
		u.Friends[friend] = struct{}{}
	}
	u.updateLastTime()
	u.save()
}

func (u User) GetFriends() map[string]struct{} {
	return u.Friends
}

func (u *User) DelFriends(friends []string) {
	for _, friend := range friends {
		delete(u.Friends, friend)
	}
	u.updateLastTime()
	u.save()
}

func (u User) data() ([]byte, error) {
	return json.Marshal(u)
}

func (u User) save() {
	data, err := u.data()
	if err == nil {
		_ = u.store.PutOne([]byte(u.storeKey), data)
	}
}

func (u User) unsave() {
	_ = u.store.DeleteOne([]byte(u.storeKey))
}

type UserManager struct {
	users          *util.SafeLinkMap
	store          store.IStore
	offlineHandler func(address string, friends map[string]struct{}) `json:"-"`
}

func NewUserManager(store store.IStore, offlineHandler func(address string, friends map[string]struct{})) *UserManager {
	return &UserManager{
		users:          util.NewLinkMap(),
		store:          store,
		offlineHandler: offlineHandler,
	}
}

func (um *UserManager) HasUser(address string) bool {
	return um.users.Has(address)
}

func (um *UserManager) GetUser(address string) *User {
	_user, ok := um.users.Get(address)
	if !ok {
		return nil
	}
	return _user.(*User)
}

func (um *UserManager) SetUser(address string, user *User) {
	um.users.Add(address, user)
}

func (um *UserManager) _deletUser(user *User) {
	um.users.Delete(user.Address)
	if um.offlineHandler != nil {
		um.offlineHandler(user.Address, user.GetFriends())
	}
}

func (um *UserManager) ClearUser(address string) {
	user := um.GetUser(address)
	if user != nil {
		user.ClearEndPoints()
		um._deletUser(user)
	}
}

func (um *UserManager) AddUser(address string, devid string) *User {
	user := um.GetUser(address)
	if user == nil {
		user = NewUser(address, um.store)
		um.users.Add(address, user)
	}

	user.AddEndPoint(devid)
	return user
}

func (um *UserManager) DelUser(address string, devid string) {
	user := um.GetUser(address)
	if user != nil {
		user.DelEndPoint(devid)
		if !user.IsEmpty() {
			um._deletUser(user)
		}
	}
}

func (um *UserManager) AddFriends(address string, devid string, friends []string) {
	user := um.AddUser(address, devid)
	user.AddFriends(friends)
}

func (um *UserManager) DelFriends(address string, devid string, friends []string) {
	user := um.AddUser(address, devid)
	user.DelFriends(friends)
}
