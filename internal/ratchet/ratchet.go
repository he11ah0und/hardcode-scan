// Package ratchet compares a fresh scan against an allowlist of known
// hardcoded strings and reports new and stale entries.
package ratchet

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// LoadAllow reads an allowlist file into a set of keys. A missing file
// yields an empty set (everything in the scan is "new"). Empty lines are
// ignored.
func LoadAllow(path string) (map[string]struct{}, error) {
	set := map[string]struct{}{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return set, nil
		}
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		set[line] = struct{}{}
	}
	return set, nil
}

// Compare splits scanKeys into keys missing from the allowlist (new) and
// allowlist entries missing from the scan (stale). Both slices are sorted.
func Compare(scanKeys []string, allow map[string]struct{}) (newKeys, stale []string) {
	scanned := make(map[string]struct{}, len(scanKeys))
	for _, k := range scanKeys {
		scanned[k] = struct{}{}
		if _, ok := allow[k]; !ok {
			newKeys = append(newKeys, k)
		}
	}
	for k := range allow {
		if _, ok := scanned[k]; !ok {
			stale = append(stale, k)
		}
	}
	sort.Strings(newKeys)
	sort.Strings(stale)
	return newKeys, stale
}

// WriteAllow overwrites the allowlist with the sorted, deduplicated keys.
func WriteAllow(path string, keys []string) error {
	sorted := make([]string, len(keys))
	copy(sorted, keys)
	sort.Strings(sorted)
	var b strings.Builder
	prev := ""
	for i, k := range sorted {
		if i > 0 && k == prev {
			continue
		}
		prev = k
		fmt.Fprintln(&b, k)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
