package medialib

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (s *Service) IsSaveConfigured() bool {
	return strings.TrimSpace(s.saveDir) != ""
}

func (s *Service) ResolveSavePath(raw string) (string, string, error) {
	return s.resolveRootPath(s.saveDir, raw)
}

func (s *Service) ListSaveItems() ([]Item, error) {
	if !s.IsSaveConfigured() {
		return nil, fmt.Errorf("save dir is not configured")
	}
	if _, err := os.Stat(s.saveDir); err != nil {
		if os.IsNotExist(err) {
			return []Item{}, nil
		}
		return nil, err
	}
	itemDirs, err := s.listRootItemDirs(s.saveDir)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(itemDirs))
	for _, absPath := range itemDirs {
		item, ok, err := s.inspectRootDir(s.saveDir, absPath)
		if err != nil {
			return nil, err
		}
		if ok {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt == items[j].UpdatedAt {
			return firstNonEmpty(items[i].Title, items[i].Name) < firstNonEmpty(items[j].Title, items[j].Name)
		}
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
	return items, nil
}

func (s *Service) GetSaveDetail(raw string) (*Detail, error) {
	relPath, absPath, err := s.ResolveSavePath(raw)
	if err != nil {
		return nil, err
	}
	return s.readRootDetail(s.saveDir, relPath, absPath)
}

func (s *Service) UpdateSaveItem(raw string, meta Meta) (*Detail, error) {
	relPath, absPath, err := s.ResolveSavePath(raw)
	if err != nil {
		return nil, err
	}
	detail, err := s.readRootDetail(s.saveDir, relPath, absPath)
	if err != nil {
		return nil, err
	}
	return s.updateRootItem(s.saveDir, detail, absPath, meta)
}

func (s *Service) ReplaceSaveAsset(raw string, variantKey string, kind string, originalName string, data []byte) (*Detail, error) {
	relPath, absPath, err := s.ResolveSavePath(raw)
	if err != nil {
		return nil, err
	}
	detail, err := s.readRootDetail(s.saveDir, relPath, absPath)
	if err != nil {
		return nil, err
	}
	return s.replaceRootArtwork(s.saveDir, detail, absPath, variantKey, kind, originalName, data)
}

func (s *Service) CropSavePoster(raw string, variantKey string, x int, y int, width int, height int) (*Detail, error) {
	relPath, absPath, err := s.ResolveSavePath(raw)
	if err != nil {
		return nil, err
	}
	detail, err := s.readRootDetail(s.saveDir, relPath, absPath)
	if err != nil {
		return nil, err
	}
	return s.cropRootPosterFromCover(s.saveDir, detail, absPath, variantKey, x, y, width, height)
}

func (s *Service) DeleteSaveFile(raw string) (*Detail, error) {
	relPath, absPath, err := s.ResolveSavePath(raw)
	if err != nil {
		return nil, err
	}
	itemAbsPath := filepath.Dir(filepath.Dir(absPath))
	itemRelPath, err := filepath.Rel(s.saveDir, itemAbsPath)
	if err != nil {
		return nil, err
	}
	return s.deleteRootFile(s.saveDir, filepath.ToSlash(itemRelPath), relPath)
}
