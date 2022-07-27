package dweb

import (
	"testing"
)

func TestDwebEncode(t *testing.T) {
	buf := []byte{0, 0, 0, 233, 0, 200, 0,
		9, 123, 34, 97, 34, 58, 34,
		98, 34, 125, 1, 2, 3}
	var res HttpProxyResponse
	_ = decodeDwebResponce(buf, &res)
	if res.ReqId != 233 {
		t.Errorf("decode result `ReqId` no same with web client")
	}
	if res.StatusCode != 200 {
		t.Errorf("decode result `StatusCode` no same with web client")
	}
	encBytes, _ := encodeDwebResponce(&res)
	if string(buf) != string(encBytes) {
		t.Errorf("encode decode result no same")
	}
	print(encBytes)
}
func TestValidDwebHost(t *testing.T) {
	type DportAddress struct {
		Dport      string
		HexAddress string
		Address    string
	}
	testValidMap := []DportAddress{
		{"www.baidu.com", "326574386a476231346641667a51424e674c5248637756784c643450696631.6d6e3a7777772e62616964752e636f6d", "2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn"},
	}
	for _, v := range testValidMap {
		host := v.HexAddress + ".localhost:19020"
		address, dport, err := ValidDwebHost(host)
		if err != nil {
			t.Fail()
			return
		}
		if dport != v.Dport {
			t.Fail()
			return
		}
		if address != v.Address {
			t.Fail()
			return
		}
	}
	testUnValidMap := []DportAddress{
		{"www.baidu.com", "326574386a476231346641667a51424e674c5248637756784c643450696631.6d6e7777772e62616964752e636f6d", "2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn"},
		{"abc", "123", ""},
		{"v!1", "326574386a476231346641667a51424e674c5248637756784c643450696631.6d6e", "2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn"},
		{"v@1", "326574386a476231346641667a51424e674c5248637756784c643450696631.6d6e", "2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn"},
		{"v~1", "326574386a476231346641667a51424e674c5248637756784c643450696631.6d6e", "2et8jGb14fAfzQBNgLRHcwVxLd4Pif1mn"},
	}
	for _, v := range testUnValidMap {
		host := v.Dport + "." + v.HexAddress + ".localhost:19020"
		_, _, err := ValidDwebHost(host)
		if err == nil {
			t.Fail()
			return
		}
	}

}
