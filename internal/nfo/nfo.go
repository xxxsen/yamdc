package nfo

import (
	"encoding/xml"
	"io"
	"os"
)

func ParseMovie(f string) (*Movie, error) {
	raw, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}
	return ParseMovieWithData(raw)
}

func ParseMovieWithData(data []byte) (*Movie, error) {
	movie := &Movie{}
	if err := xml.Unmarshal(data, movie); err != nil {
		return nil, err
	}
	return movie, nil
}

func WriteMovie(w io.Writer, movie *Movie) error {
	xmlData, err := xml.MarshalIndent(movie, "", "  ")
	if err != nil {
		return err
	}

	xmlWithHeader := []byte(xml.Header + string(xmlData))
	if _, err := w.Write(xmlWithHeader); err != nil {
		return err
	}
	return nil
}

func WriteMovieToFile(f string, m *Movie) error {
	file, err := os.OpenFile(f, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	return WriteMovie(file, m)
}
