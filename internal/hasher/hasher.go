package hasher

import (
	"crypto/md5"  //nolint:gosec // used for content fingerprinting, not security
	"crypto/sha1" //nolint:gosec // used for content fingerprinting, not security
	"encoding/hex"
)

func ToMD5(in string) string {
	return ToMD5Bytes([]byte(in))
}

func ToMD5Bytes(in []byte) string {
	h := md5.New() //nolint:gosec // used for content fingerprinting, not security
	_, _ = h.Write(in)
	return hex.EncodeToString(h.Sum(nil))
}

func ToSha1(in string) string {
	return ToSha1Bytes([]byte(in))
}

func ToSha1Bytes(in []byte) string {
	h := sha1.New() //nolint:gosec // used for content fingerprinting, not security
	_, _ = h.Write(in)
	return hex.EncodeToString(h.Sum(nil))
}
