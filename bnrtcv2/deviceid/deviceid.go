package deviceid

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/big"

	"github.com/guabee/bnrtc/util"
)

const (
	DeviceIdLength          = 20
	TopNetworkOffset        = 0
	GroupIdOffset           = 1
	GroupIdLength           = 4
	NodeIdOffset            = 5
	NodeIdLength            = 15
	DeviceIdCalculateOffset = GroupIdOffset
	DeviceIdCalculateLength = GroupIdLength
	DeviceIdTopFlags        = 0x08
)

type DeviceId []uint8

var EmptyDeviceId DeviceId = nil
var ZeroDeviceId DeviceId = make(DeviceId, DeviceIdLength)

func NewDeviceId(topNetwork bool, groupId []byte, nodeid []byte) DeviceId {
	if groupId == nil || bytes.Equal(groupId, []byte{0, 0, 0, 0}) {
		groupId = make([]byte, GroupIdLength)
		util.RandomIt(groupId)
	}

	id := make(DeviceId, DeviceIdLength)
	if topNetwork {
		id[TopNetworkOffset] = DeviceIdTopFlags
	}
	copy(id[GroupIdOffset:], groupId[:])
	if nodeid != nil {
		if len(nodeid) != NodeIdLength {
			util.RandomIt(id[NodeIdOffset:])
		}
		copy(id[NodeIdOffset:], nodeid)
	} else {
		util.RandomIt(id[NodeIdOffset:])
	}
	return id
}

func FromString(devid string) DeviceId {
	if devid == "" {
		return EmptyDeviceId
	}

	id := base58Decode(devid)
	if len(id) != DeviceIdLength {
		return EmptyDeviceId
	}
	return id
}

func (t DeviceId) String() string {
	if t.IsEmpty() {
		return ""
	}
	return base58Encode(t)
}

func (t DeviceId) Copy() DeviceId {
	id := make(DeviceId, DeviceIdLength)
	copy(id, t)
	return id
}

func (t DeviceId) IsValid() bool {
	return len(t) == DeviceIdLength
}

func (t DeviceId) IsEmpty() bool {
	return t.Equal(EmptyDeviceId)
}

func (d DeviceId) IsTopNetwork() bool {
	if !d.IsValid() {
		return false
	}
	return d[TopNetworkOffset]&DeviceIdTopFlags == DeviceIdTopFlags
}

func (d DeviceId) ToTopNetwork() DeviceId {
	if !d.IsValid() {
		return EmptyDeviceId
	}

	id := make(DeviceId, DeviceIdLength)
	copy(id, d)
	id[TopNetworkOffset] |= DeviceIdTopFlags
	return id
}

func (d DeviceId) GroupId() []byte {
	if !d.IsValid() {
		return nil
	}
	return d[GroupIdOffset : GroupIdOffset+GroupIdLength]
}

func (d DeviceId) CalculateId() []byte {
	if !d.IsValid() {
		return nil
	}
	return d[DeviceIdCalculateOffset : DeviceIdCalculateOffset+DeviceIdCalculateLength]
}

func (d DeviceId) GroupIdValue() int {
	if !d.IsValid() {
		return 0
	}
	buf := bytes.NewBuffer(d[GroupIdOffset : GroupIdOffset+GroupIdLength])
	var value int32
	_ = binary.Read(buf, binary.BigEndian, &value)
	return int(value)
}

func (d DeviceId) NodeIdValue() []byte {
	if !d.IsValid() {
		return nil
	}
	return d[NodeIdOffset:]
}

func (d DeviceId) NodeId() string {
	if !d.IsValid() {
		return ""
	}
	return base58Encode(d[NodeIdOffset:])
}

func (d DeviceId) InSameGroup(another DeviceId) bool {
	if !another.IsValid() {
		return false
	}

	return d.SameGroupId(another.GroupId())
}

func (d DeviceId) SameGroupId(groupId []byte) bool {
	if !d.IsValid() {
		return false
	}

	return bytes.Equal(d.GroupId(), groupId)
}

func (d DeviceId) SameNodeId(id DeviceId) bool {
	return bytes.Equal(d.NodeIdValue(), id.NodeIdValue())
}

func (d DeviceId) Equal(another DeviceId) bool {
	return d.Compare(another) == 0
}

func (d DeviceId) Less(another DeviceId) bool {
	return d.Compare(another) < 0
}

func (d DeviceId) Compare(another DeviceId) int {
	return bytes.Compare(d, another)
}

func (d DeviceId) GetDistance(another DeviceId) *big.Int {
	if !d.IsValid() || !another.IsValid() {
		return nil
	}

	res := make(DeviceId, DeviceIdLength)
	for i := 0; i < len(d); i++ {
		res[i] = d[i] ^ another[i]
	}

	return big.NewInt(0).SetBytes(res)
}

func (d DeviceId) XOR(another DeviceId) DeviceId {
	if !d.IsValid() || !another.IsValid() {
		return nil
	}

	res := make(DeviceId, DeviceIdLength)
	for i := 0; i < len(d); i++ {
		res[i] = d[i] ^ another[i]
	}

	return res
}

func (d DeviceId) HighestDifferBit(another DeviceId) int {
	if !d.IsValid() || !another.IsValid() {
		return DeviceIdCalculateLength*8 - 1
	}

	calculateId := d.CalculateId()
	calculateId2 := another.CalculateId()
	for j := 0; j < DeviceIdCalculateLength; j++ {
		xor := calculateId[j] ^ calculateId2[j]
		for i := 0; i < 8; i++ {
			if util.HasBit(xor, uint(i)) {
				byteIndex := j * 8
				bitIndex := i
				return byteIndex + bitIndex
			}
		}
	}

	return DeviceIdCalculateLength*8 - 1
}

func (d DeviceId) BitwiseInversion(bitPoint int) DeviceId {
	if !d.IsValid() {
		return nil
	}
	if bitPoint > DeviceIdCalculateLength*8 {
		return nil
	}
	inverDeviceId := d.Copy()
	inverIndex := bitPoint / 8
	inverDeviceId[DeviceIdCalculateOffset+inverIndex] ^= uint8(math.Pow(2, float64(7-bitPoint%8)))
	return inverDeviceId
}
