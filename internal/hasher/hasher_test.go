package hasher

import (
	"bytes"
	"crypto/md5"  //nolint:gosec // reference for cross-check tests
	"crypto/sha1" //nolint:gosec // reference for cross-check tests
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToMD5_KnownVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "d41d8cd98f00b204e9800998ecf8427e"},
		{"hello", "hello", "5d41402abc4b2a76b9719d911017c592"},
		{"unicode", "你好", "7eca689f0d3389d9dea66ae112e5cfd7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ToMD5(tt.in)
			assert.Equal(t, tt.want, got)
			assert.Len(t, got, 32)
		})
	}
}

func TestToMD5Bytes_KnownVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"empty_slice", []byte{}, "d41d8cd98f00b204e9800998ecf8427e"},
		{"hello", []byte("hello"), "5d41402abc4b2a76b9719d911017c592"},
		{"unicode_utf8", []byte("你好"), "7eca689f0d3389d9dea66ae112e5cfd7"},
		{"binary", []byte{0x00, 0xff, 0x7f}, "ff4068e5fb45f28e025e7c1004b3e65a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ToMD5Bytes(tt.in)
			assert.Equal(t, tt.want, got)
			assert.Len(t, got, 32)
		})
	}
}

func TestToMD5Bytes_NilSlice(t *testing.T) {
	t.Parallel()
	// nil slice behaves like empty input for hash writers
	assert.Equal(t, ToMD5(""), ToMD5Bytes(nil))
	assert.Equal(t, "d41d8cd98f00b204e9800998ecf8427e", ToMD5Bytes(nil))
}

func TestToSha1_KnownVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"hello", "hello", "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"},
		{"unicode", "你好", "440ee0853ad1e99f962b63e459ef992d7c211722"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ToSha1(tt.in)
			assert.Equal(t, tt.want, got)
			assert.Len(t, got, 40)
		})
	}
}

func TestToSha1Bytes_KnownVectors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"empty_slice", []byte{}, "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"hello", []byte("hello"), "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"},
		{"unicode_utf8", []byte("你好"), "440ee0853ad1e99f962b63e459ef992d7c211722"},
		{"binary", []byte{0x00, 0xff, 0x7f}, "6157fd21706ec7ac8b87cc54caaf4936ad41cce7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ToSha1Bytes(tt.in)
			assert.Equal(t, tt.want, got)
			assert.Len(t, got, 40)
		})
	}
}

func TestToSha1Bytes_NilSlice(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ToSha1(""), ToSha1Bytes(nil))
	assert.Equal(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709", ToSha1Bytes(nil))
}

func TestStringAndBytesAPIsAgree(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"",
		"a",
		"hello 世界",
		string([]byte{0xff, 0xfe, 0x00}), // not valid UTF-8
	}
	for _, s := range inputs {
		b := []byte(s)
		assert.Equal(t, ToMD5Bytes(b), ToMD5(s), "md5: %q", s)
		assert.Equal(t, ToSha1Bytes(b), ToSha1(s), "sha1: %q", s)
	}
}

func TestVeryLongInput_MatchesStdlib(t *testing.T) {
	t.Parallel()

	chunk := bytes.Repeat([]byte("abc"), 70_000) // 210_000 bytes
	require.Len(t, chunk, 210_000)

	wantMD5 := md5.Sum(chunk) //nolint:gosec // reference digest
	assert.Equal(t, hex.EncodeToString(wantMD5[:]), ToMD5Bytes(chunk))
	assert.Equal(t, hex.EncodeToString(wantMD5[:]), ToMD5(string(chunk)))

	wantSHA1 := sha1.Sum(chunk) //nolint:gosec // reference digest
	assert.Equal(t, hex.EncodeToString(wantSHA1[:]), ToSha1Bytes(chunk))
	assert.Equal(t, hex.EncodeToString(wantSHA1[:]), ToSha1(string(chunk)))
}

func TestDistinctInputsProduceDistinctDigests(t *testing.T) {
	t.Parallel()
	require.NotEqual(t, ToMD5("foo"), ToMD5("bar"))
	require.NotEqual(t, ToSha1("foo"), ToSha1("bar"))
}
