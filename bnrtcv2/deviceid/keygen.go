package deviceid

import (
	"bytes"
	"crypto/sha256"
)

type DataKey []uint8

func DataKeyFromString(key string) DataKey {
	if key == "" {
		return nil
	}

	dataKey := base58Decode(key)
	if len(dataKey) != DeviceIdLength {
		return nil
	}
	return dataKey
}

func KeyGen(data []byte) DataKey {
	sum := sha256.Sum256(data)
	sum[TopNetworkOffset] |= DeviceIdTopFlags // to topnetwork key
	return sum[:DeviceIdLength]
}

func (d DataKey) IsEmpty() bool {
	return d == nil
}

func (d DataKey) Equal(o DataKey) bool {
	return bytes.Equal(d, o)
}

func (d DataKey) Verify(data []byte) bool {
	return d.Equal(KeyGen(data))
}

func (d DataKey) String() string {
	if d.IsEmpty() {
		return ""
	}
	return base58Encode(d)
}

func (d DataKey) ToDeviceId() DeviceId {
	return DeviceId(d)
}
