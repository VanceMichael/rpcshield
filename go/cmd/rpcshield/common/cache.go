package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

const _maxCacheAge = 7 * 24 * time.Hour

// CacheOption is the option for cache file path.
type CacheOption string

const (
	// CacheOptionAuto is the option to use the cached file in the workspace root if it
	// exists and is newer than 1 week.
	CacheOptionAuto CacheOption = "auto"
	// CacheOptionRefresh is the option to force refresh the cached file.
	CacheOptionRefresh CacheOption = "refresh"
)

// CheckValid checks if the cache option is either a valid option or a path to a file
// that exists.
func (c CacheOption) CheckValid() error {
	if c == CacheOptionAuto || c == CacheOptionRefresh {
		return nil
	}

	if _, err := os.Stat(string(c)); err != nil {
		return fmt.Errorf("valid values are `auto` or `refresh`, or a path to a file: %w", err)
	}

	return nil
}

// FetchFromCache fetches the cache from the cache file via the given option:
//
//   - CacheOptionAuto: if the cache file exists and is newer than 1 week, it will be
//     decoded as a JSON file. Otherwise, the cache will be recreated via the `recreate`
//     function and stored in the default path.
//
//   - CacheOptionRefresh: the cache will be recreated via the `recreate` function and
//     stored in the default path.
//
// Otherwise, the cache will be decoded directly from the given cache option as a JSON file.
func FetchFromCache[T any](opt CacheOption, defaultPath string, recreate func() (T, error), logger *zap.SugaredLogger) (T, error) {
	// If the option is not auto or refresh, decode the cache directly from the given
	// cache option as a JSON file.
	if opt != CacheOptionAuto && opt != CacheOptionRefresh {
		logger.Infow("Using given cache file", "path", string(opt), "age", fileAgeStr(string(opt)))
		return decode[T](string(opt))
	}

	// For "auto", try to use cache if fresh; otherwise, fall through to recreate.
	if opt == CacheOptionAuto {
		info, err := os.Stat(defaultPath)
		if err == nil && time.Since(info.ModTime()) < _maxCacheAge {
			logger.Infow("Using default cached file", "path", defaultPath, "age", fileAgeStr(defaultPath), "maxAge", _maxCacheAge)
			return decode[T](defaultPath)
		}

		logger.Infow("Default cache file is too old or missing, will recreate", "path", defaultPath, "age", fileAgeStr(defaultPath), "maxAge", _maxCacheAge)
	}

	t, err := recreate()
	if err != nil {
		return t, fmt.Errorf("recreate cache: %w", err)
	}
	if err := writeToCache[T](defaultPath, t); err != nil {
		return t, fmt.Errorf("write cache file: %w", err)
	}
	logger.Infow("Refreshed cache file", "path", defaultPath)
	return t, nil
}

func decode[T any](path string) (T, error) {
	var t T

	f, err := os.Open(path)
	if err != nil {
		return t, fmt.Errorf("open cache file: %w", err)
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&t); err != nil {
		return t, fmt.Errorf("decode cache file: %w", err)
	}

	return t, nil
}

func writeToCache[T any](path string, t T) error {
	// Ensure the directory exists before opening the file.
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create cache file: %w", err)
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(t)
}

// fileAgeStr returns the age of the file in a human-readable format. If the file does not
// exist, it returns "<invalid>".
func fileAgeStr(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "<invalid>"
	}
	return time.Since(info.ModTime()).String()
}
