package hasher

import (
	"crypto/md5"
	"encoding/hex"
)

func ToMD5(in string) string {
	h := md5.New()
	_, _ = h.Write([]byte(in))
	return hex.EncodeToString(h.Sum(nil))
}
