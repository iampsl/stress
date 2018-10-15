package head

import (
	"crypto/hmac"
	"crypto/rc4"
	"crypto/sha1"
	"stress/global"
)

//Encode head encode
type Encode struct {
	c *rc4.Cipher
}

//NewEncode create encode
func NewEncode(key string) *Encode {
	h := sha1.New()
	_, err := h.Write([]byte(key))
	if err != nil {
		global.AppLog.PrintlnError(err)
		return nil
	}
	mac := hmac.New(sha1.New, []byte{0xC2, 0xB3, 0x72, 0x3C, 0xC6, 0xAE, 0xD9, 0xB5, 0x34, 0x3C, 0x53, 0xEE, 0x2F, 0x43, 0x67, 0xCE})
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
	return &Encode{c: pc}
}

//Do encode data
func (e *Encode) Do(dst []byte, src []byte) {
	e.c.XORKeyStream(dst, src)
}
