package nfo

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

func ParseMovie(f string) (*Movie, error) {
	raw, err := os.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("read nfo file %s failed: %w", f, err)
	}
	return ParseMovieWithData(raw)
}

func ParseMovieWithData(data []byte) (*Movie, error) {
	movie := &Movie{}
	if err := xml.Unmarshal(data, movie); err != nil {
		return nil, fmt.Errorf("unmarshal nfo xml failed: %w", err)
	}
	return movie, nil
}

func WriteMovie(w io.Writer, movie *Movie) error {
	xmlData, err := xml.MarshalIndent(movie, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal nfo xml failed: %w", err)
	}

	xmlWithHeader := []byte(xml.Header + string(xmlData))
	if _, err := w.Write(xmlWithHeader); err != nil {
		return fmt.Errorf("write nfo xml failed: %w", err)
	}
	return nil
}

func WriteMovieToFile(f string, m *Movie) error {
	file, err := os.OpenFile(f, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open nfo file %s failed: %w", f, err)
	}
	defer func() {
		_ = file.Close()
	}()
	return WriteMovie(file, m)
}
