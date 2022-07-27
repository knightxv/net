package deviceid_test

import (
	"bytes"
	"net"
	"testing"

	"github.com/guabee/bnrtc/bnrtcv2/deviceid"
	"github.com/guabee/bnrtc/bnrtcv2/rpc"
	"github.com/guabee/bnrtc/util"
)

func TestDeviceID(t *testing.T) {
	t.Log("TestDeviceID")

	groupID := []byte{192, 168, 0, 1}
	groupID2 := []byte{192, 168, 0, 2}
	randomID := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e}
	id1 := deviceid.NewDeviceId(true, groupID, nil)
	id2 := deviceid.NewDeviceId(true, groupID, nil)
	id3 := deviceid.NewDeviceId(false, groupID2, randomID)
	id4 := id3[:15]
	id5 := deviceid.DeviceId(nil)
	id1Str := id1.String()

	if !deviceid.FromString(id1Str).Equal(id1) {
		t.Fatalf("id1 should equal to the string version of itself")
	}

	// IsValid
	if !id1.IsValid() || !id2.IsValid() || !id3.IsValid() || id4.IsValid() || id5.IsValid() {
		t.Error("id IsValid check failed")
	}

	// IsEmpty
	if id1.IsEmpty() || id2.IsEmpty() || id3.IsEmpty() || id4.IsEmpty() || !id5.IsEmpty() {
		t.Error("id IsEmpty check failed")
	}

	// IsTopnetwork
	if !id1.IsTopNetwork() || !id2.IsTopNetwork() || id3.IsTopNetwork() || id4.IsTopNetwork() || id5.IsTopNetwork() {
		t.Error("id IsTopnetwork check failed")
	}

	// ToTopNetwork
	if !id1.ToTopNetwork().IsTopNetwork() || !id2.ToTopNetwork().IsTopNetwork() || !id3.ToTopNetwork().IsTopNetwork() || !id4.ToTopNetwork().IsEmpty() || !id5.ToTopNetwork().IsEmpty() {
		t.Error("id ToTopnetwork check failed")
	}

	// GroupID
	if !bytes.Equal(id1.GroupId(), groupID) {
		t.Error("id GroupID check failed")
	}

	// Random Value
	if !bytes.Equal(id3.NodeIdValue(), randomID) {
		t.Error("id randomID check failed")
	}

	// SameGroup
	if !id1.InSameGroup(id2) || id1.InSameGroup(id3) || id1.InSameGroup(id4) || id1.InSameGroup(id5) {
		t.Error("id InSameGroup check failed")
	}

	// Equal
	if !id1.Equal(id1) || id1.Equal(id2) || id1.Equal(id3) || id1.Equal(id4) || id1.Equal(id5) {
		t.Fatalf("id Equal check failed")
	}

	// Distance
	d1 := id1.GetDistance(id2)
	d2 := id1.GetDistance(id3)
	d3 := id1.GetDistance(id4)
	t.Logf("d1: %d, d2: %d, d3: %d", d1, d2, d3)

}

func TestDataKey(t *testing.T) {
	t.Log("TestDataKey")

	data1 := []byte{192, 168, 0, 1}
	data2 := []byte{192, 168, 0, 2}
	keyNull := deviceid.DataKey(nil)
	key1 := deviceid.KeyGen(data1)
	key2 := deviceid.KeyGen(data2)
	key3 := deviceid.KeyGen(data1)

	// Equal
	if !key1.Equal(key3) || key1.Equal(key2) || key1.Equal(keyNull) {
		t.Fatalf("key Equal check failed")
	}

	// IsEmpty
	if key1.IsEmpty() || key2.IsEmpty() || !keyNull.IsEmpty() {
		t.Error("key IsEmpty check failed")
	}

	// Verify
	if !key1.Verify(data1) || !key2.Verify(data2) || key1.Verify(data2) || keyNull.Verify(data1) {
		t.Error("key Verify check failed")
	}
}

func TestConfigDeviceManager(t *testing.T) {
	groupID := []byte{192, 168, 0, 1}
	devID := deviceid.NewDeviceId(true, groupID, nil)
	managerConfig := deviceid.NewDeviceIdManager(deviceid.WithDeviceId(devID), deviceid.WithPubSub(util.NewPubSub()))

	if !managerConfig.GetDeviceId(true).Equal(devID) {
		t.Error("manager get device id check failed")
	}

	_, _ = managerConfig.PubSub.SubscribeFunc("managerConfig", rpc.DeviceIdChangeTopic, func(msg util.PubSubMsgType) {
		id := msg.(deviceid.DeviceId)
		if !id.Equal(devID) {
			t.Error("manager OnDeviceIdChange check failed")
		}
	})
}

func TestAutoDeviceManager(t *testing.T) {
	done := make(chan bool)
	autoManager := deviceid.NewDeviceIdManager(deviceid.WithPubSub(util.NewPubSub()))
	var ID deviceid.DeviceId
	go func() {
		id := autoManager.GetDeviceId(true)

		if ID.IsEmpty() {
			ID = id
			t.Logf("autoManager id %s", ID)
		} else {
			if !ID.Equal(id) {
				t.Error("autoManager id check failed")
			}
			done <- true
		}
	}()

	_, _ = autoManager.PubSub.SubscribeFunc("autoManager", rpc.DeviceIdChangeTopic, func(msg util.PubSubMsgType) {
		id := msg.(deviceid.DeviceId)
		if ID.IsEmpty() {
			ID = id
			t.Logf("autoManager id %s", ID)
		} else {
			if !ID.Equal(id) {
				t.Error("autoManager id check failed")
			}
			done <- true
		}
	})

	autoManager.RPCCenter.Regist(rpc.GetExternalIpRpcName, func(params ...util.RPCParamType) util.RPCResultType {
		return net.ParseIP("192.168.0.1")
	})

	<-done
}

func TestBitwiseInversion(t *testing.T) {
	t.Log("TestBitwiseInversion")

	groupID := []byte{192, 168, 0, 1}
	devID := deviceid.NewDeviceId(true, groupID, nil)
	{
		inversionDevID := devID.BitwiseInversion(2)
		if devID.HighestDifferBit(inversionDevID) != 2 {
			t.Error("inversionDevID must compare devID in point 3")
		}
	}
	{
		inversionDevID := devID.BitwiseInversion(deviceid.GroupIdLength - 1)
		if devID.HighestDifferBit(inversionDevID) != deviceid.GroupIdLength-1 {
			t.Error("inversionDevID must compare devID in point 3")
		}
	}
}
