package nfo

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	_ = p
	return 0, errors.New("write failed")
}

func TestReadWrite(t *testing.T) {
	m := &Movie{
		XMLName:       xml.Name{},
		Plot:          "hello world, this is a test",
		Dateadded:     "2022-01-02",
		Title:         "hello world",
		OriginalTitle: "hello world",
		SortTitle:     "hello world",
		Set:           "aaaa",
		Rating:        111,
		Release:       "2021-01-05",
		ReleaseDate:   "2021-01-05",
		Premiered:     "2021-01-05",
		Runtime:       60,
		Year:          2021,
		Tags:          []string{"t_a", "t_b", "t_c", "t_d"},
		Studio:        "hello_studio",
		Maker:         "hello_maker",
		Genres:        []string{"t_x", "t_y", "t_z"},
		Art:           Art{Poster: "art_poster.jpg", Fanart: []string{"art_fanart_1", "art_fanart_2", "art_fanart_3"}},
		Director:      "hello_director",
		Actors:        []Actor{{Name: "act_a", Role: "main", Thumb: "act_a.jpg"}},
		Poster:        "poster.jpg",
		Thumb:         "thumb.jpg",
		Label:         "hello_label",
		ID:            "2022-01111",
		Cover:         "cover.jpg",
		Fanart:        "fanart.jpg",
		ScrapeInfo: ScrapeInfo{
			Source: "abc",
			Date:   "2021-03-05",
		},
	}
	buf := bytes.NewBuffer(nil)
	err := WriteMovie(buf, m)
	assert.NoError(t, err)
	newM, err := ParseMovieWithData(buf.Bytes())
	assert.NoError(t, err)
	newM.XMLName = m.XMLName
	assert.Equal(t, m, newM)
}

func TestParseMovieWithDataInvalidXML(t *testing.T) {
	t.Parallel()
	_, err := ParseMovieWithData([]byte("not valid xml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal nfo xml failed")
}

func TestWriteMovieWriterError(t *testing.T) {
	t.Parallel()
	m := &Movie{Title: "t"}
	err := WriteMovie(failingWriter{}, m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write nfo xml failed")
}

func TestParseMovieFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "movie.nfo")
	m := &Movie{Title: "from disk", ID: "id-1"}
	buf := bytes.NewBuffer(nil)
	require.NoError(t, WriteMovie(buf, m))
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o600))

	got, err := ParseMovie(path)
	require.NoError(t, err)
	got.XMLName = m.XMLName
	assert.Equal(t, m, got)
}

func TestParseMovieReadError(t *testing.T) {
	t.Parallel()
	_, err := ParseMovie(filepath.Join(t.TempDir(), "does-not-exist.nfo"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read nfo file")
}

func TestWriteMovieToFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.nfo")
	m := &Movie{Title: "round", Plot: "p", ID: "x"}
	require.NoError(t, WriteMovieToFile(path, m))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	got, err := ParseMovieWithData(raw)
	require.NoError(t, err)
	got.XMLName = m.XMLName
	assert.Equal(t, m, got)
}

func TestWriteMovieToFileOpenError(t *testing.T) {
	t.Parallel()
	// Opening a directory for truncate+write-only should fail on common Unix setups.
	err := WriteMovieToFile(t.TempDir(), &Movie{Title: "nope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open nfo file")
}
