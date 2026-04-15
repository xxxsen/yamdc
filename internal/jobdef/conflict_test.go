package jobdef

import (
	"strings"
	"testing"

	"github.com/xxxsen/yamdc/internal/number"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildConflictKey_EmptyNumberText(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", BuildConflictKey("", ".mp4", "anything.mp4"))
}

func TestBuildConflictKey_WhitespaceOnlyNumberText(t *testing.T) {
	t.Parallel()
	cases := []string{"   ", "\t\n", " \t "}
	for _, s := range cases {
		assert.Equal(t, "", BuildConflictKey(s, ".mp4", "x.mp4"), "numberText=%q", s)
	}
}

func TestBuildConflictKey_ValidNumberWithExplicitExt(t *testing.T) {
	t.Parallel()
	parsed, err := number.Parse("ssis-001")
	require.NoError(t, err)
	wantBase := strings.ToUpper(parsed.GenerateFileName())
	assert.Equal(t, wantBase+".mp4", BuildConflictKey("ssis-001", ".mp4", ""))
	assert.Equal(t, wantBase+".mp4", BuildConflictKey("ssis-001", ".MP4", ""))
	assert.Equal(t, wantBase+".mkv", BuildConflictKey("  ssis-001  ", "  .MKV  ", ""))
}

func TestBuildConflictKey_ValidNumberExtFromFileName(t *testing.T) {
	t.Parallel()
	parsed, err := number.Parse("abc-123")
	require.NoError(t, err)
	wantBase := strings.ToUpper(parsed.GenerateFileName())
	assert.Equal(t, wantBase+".mp4", BuildConflictKey("abc-123", "", "SSIS-001.mp4"))
	assert.Equal(t, wantBase+".mkv", BuildConflictKey("abc-123", "", `/path/to/foo/bar.ABC-123.MKV`))
}

func TestBuildConflictKey_EmptyExtEmptyFileName(t *testing.T) {
	t.Parallel()
	parsed, err := number.Parse("n001")
	require.NoError(t, err)
	want := strings.ToUpper(parsed.GenerateFileName())
	assert.Equal(t, want, BuildConflictKey("n001", "", ""))
}

func TestBuildConflictKey_EmptyExtFileNameWithoutExt(t *testing.T) {
	t.Parallel()
	parsed, err := number.Parse("xyz-999")
	require.NoError(t, err)
	want := strings.ToUpper(parsed.GenerateFileName())
	assert.Equal(t, want, BuildConflictKey("xyz-999", "", "no_extension_here"))
	assert.Equal(t, want, BuildConflictKey("xyz-999", "", ""))
}

func TestBuildConflictKey_ExtWithLeadingTrailingSpaces(t *testing.T) {
	t.Parallel()
	parsed, err := number.Parse("fc2-ppv-1234567")
	require.NoError(t, err)
	wantBase := strings.ToUpper(parsed.GenerateFileName())
	assert.Equal(t, wantBase+".mp4", BuildConflictKey("fc2-ppv-1234567", "  .mp4  ", ""))
}

func TestBuildConflictKey_UnparseableNumberText_UppercasesRaw(t *testing.T) {
	t.Parallel()
	// number.Parse rejects strings containing '.'
	assert.Equal(t, "FOO.BAR.mp4", BuildConflictKey("foo.bar", ".mp4", ""))
	assert.Equal(t, "A.B.C", BuildConflictKey("a.b.c", "", ""))
}

func TestBuildConflictKey_ParseSuccess_CanonicalBaseDiffersFromRawUpper(t *testing.T) {
	t.Parallel()
	numberText := "k0009-c_cd1-4k"
	parsed, err := number.Parse(numberText)
	require.NoError(t, err)
	canonical := strings.ToUpper(parsed.GenerateFileName())
	rawUpper := strings.ToUpper(strings.TrimSpace(numberText))
	assert.NotEqual(t, rawUpper, canonical, "sanity: suffix normalization should change base")

	assert.Equal(t, canonical+".mp4", BuildConflictKey(numberText, ".mp4", ""))
}

func TestBuildConflictKey_ExplicitExtOverridesFileNameExt(t *testing.T) {
	t.Parallel()
	parsed, err := number.Parse("ssis-002")
	require.NoError(t, err)
	wantBase := strings.ToUpper(parsed.GenerateFileName())
	assert.Equal(t, wantBase+".wmv", BuildConflictKey("ssis-002", ".wmv", "ignored.mp4"))
}
