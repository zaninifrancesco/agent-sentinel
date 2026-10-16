// Package catalog discovers local JSONL recordings without changing them.
package catalog

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/zaninifrancesco/agent-sentinel/internal/analysis"
	"github.com/zaninifrancesco/agent-sentinel/internal/recorder"
)

type Recording = analysis.Recording

type cached struct {
	Size      int64
	Modified  time.Time
	Digest    [32]byte
	Recording Recording
}

type Catalog struct {
	mu    sync.Mutex
	dir   string
	extra string
	cache map[string]cached
}

// New includes the standard archive and an explicitly chosen recording path.
func New(dir, extra string) *Catalog {
	return &Catalog{dir: dir, extra: extra, cache: map[string]cached{}}
}

func (c *Catalog) paths() ([]string, []string) {
	paths := map[string]bool{}
	warnings := []string{}
	entries, err := os.ReadDir(c.dir)
	if err != nil && !os.IsNotExist(err) {
		warnings = append(warnings, "Recording archive could not be read")
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && filepath.Ext(entry.Name()) == ".jsonl" {
			paths[filepath.Join(c.dir, entry.Name())] = true
		}
	}
	if c.extra != "" {
		paths[c.extra] = true
	}
	list := make([]string, 0, len(paths))
	for path := range paths {
		list = append(list, filepath.Clean(path))
	}
	sort.Strings(list)
	return list, warnings
}

// Snapshot caches body-free projections by file size and mtime. JSONL is the
// persistent source of truth, so a restart rebuilds this disposable index.
func (c *Catalog) Snapshot() ([]Recording, []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	paths, warnings := c.paths()
	seen := map[string]bool{}
	byID := map[string]cached{}
	conflicts := map[string]bool{}
	for _, path := range paths {
		seen[path] = true
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			warnings = append(warnings, filepath.Base(path)+": recording unavailable")
			delete(c.cache, path)
			continue
		}
		item, ok := c.cache[path]
		if !ok || item.Size != info.Size() || !item.Modified.Equal(info.ModTime()) {
			f, err := os.Open(path)
			if err != nil {
				warnings = append(warnings, filepath.Base(path)+": recording unreadable")
				continue
			}
			hash := sha256.New()
			rec, err := recorder.Load(io.TeeReader(f, hash))
			f.Close()
			if err != nil {
				warnings = append(warnings, filepath.Base(path)+": invalid recording: "+err.Error())
				delete(c.cache, path)
				continue
			}
			session, events, revision := rec.Snapshot()
			if session.ID == "" {
				warnings = append(warnings, filepath.Base(path)+": missing recording identity")
				continue
			}
			rows := analysis.Project(events)
			for i := range rows {
				rows[i] = analysis.Summary(rows[i])
				rows[i].Subtitle = ""
			}
			item = cached{Size: info.Size(), Modified: info.ModTime(), Recording: Recording{Session: session, Revision: revision, Rows: rows}}
			copy(item.Digest[:], hash.Sum(nil))
			c.cache[path] = item
		}
		id := item.Recording.Session.ID
		if previous, ok := byID[id]; ok && previous.Digest != item.Digest {
			conflicts[id] = true
		}
		byID[id] = item
	}
	for path := range c.cache {
		if !seen[path] {
			delete(c.cache, path)
		}
	}
	result := []Recording{}
	for id, item := range byID {
		if conflicts[id] {
			warnings = append(warnings, "Conflicting copies of recording "+id+" excluded")
			continue
		}
		result = append(result, item.Recording)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Session.ID < result[j].Session.ID })
	sort.Strings(warnings)
	return result, warnings
}

// Open accepts an identity from the catalog, never a client-supplied path.
func (c *Catalog) Open(id string) (*recorder.Recorder, error) {
	recordings, _ := c.Snapshot()
	found := false
	for _, item := range recordings {
		if item.Session.ID == id {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("recording not found or has conflicting copies")
	}
	c.mu.Lock()
	var path string
	for candidate, item := range c.cache {
		if item.Recording.Session.ID == id {
			path = candidate
			break
		}
	}
	c.mu.Unlock()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rec, err := recorder.Load(f)
	if err != nil {
		return nil, err
	}
	if rec.Session().ID != id {
		return nil, fmt.Errorf("recording changed; refresh the catalog")
	}
	return rec, nil
}
