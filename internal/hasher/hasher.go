package hasher

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
)

func ToMD5(in string) string {
	return ToMD5Bytes([]byte(in))
}

func ToMD5Bytes(in []byte) string {
	h := md5.New()
	_, _ = h.Write(in)
	return hex.EncodeToString(h.Sum(nil))
}

func ToSha1(in string) string {
	return ToSha1Bytes([]byte(in))
}

func ToSha1Bytes(in []byte) string {
	h := sha1.New()
	_, _ = h.Write(in)
	return hex.EncodeToString(h.Sum(nil))
}
