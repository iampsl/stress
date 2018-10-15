package head

import (
	"crypto/hmac"
	"crypto/rc4"
	"crypto/sha1"
	"stress/global"
)

//Decode head decode
type Decode struct {
	c *rc4.Cipher
}

//NewDecode create Decode
func NewDecode(key string) *Decode {
	h := sha1.New()
	_, err := h.Write([]byte(key))
	if err != nil {
		global.AppLog.PrintlnError(err)
		return nil
	}
	mac := hmac.New(sha1.New, []byte{0xCC, 0x98, 0xAE, 0x04, 0xE8, 0x97, 0xEA, 0xCA, 0x12, 0xDD, 0xC0, 0x93, 0x42, 0x91, 0x53, 0x57})
	_, err = mac.Write(h.Sum(nil))
	if err != nil {
		global.AppLog.PrintlnError(err)
		return nil
	}
	pc, err := rc4.NewCipher(mac.Sum(nil))
	if err != nil {
		global.AppLog.PrintlnError(err)
		return nil
	}
	return &Decode{c: pc}
}

//Do decode data
func (e *Decode) Do(dst []byte, src []byte) {
	e.c.XORKeyStream(dst, src)
}
