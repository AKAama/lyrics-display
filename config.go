package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type config struct {
	ShowEmoji bool   `json:"show_emoji"`
	Emoji     string `json:"emoji"`
	OffsetMS  int    `json:"offset_ms"`
}

func defaultConfig() config {
	return config{
		ShowEmoji: true,
		Emoji:     defaultEmoji,
		OffsetMS:  int(defaultOffset / time.Millisecond),
	}
}

type configStore struct {
	path string
	mu   sync.Mutex
}

func newConfigStore() (*configStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	return &configStore{
		path: filepath.Join(configDir, appName, "config.json"),
	}, nil
}

func (s *configStore) pathString() string {
	return s.path
}

func (s *configStore) ensureDir() error {
	return os.MkdirAll(filepath.Dir(s.path), 0o755)
}

func (s *configStore) load() (config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := defaultConfig()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}

	cleaned, err := stripJSONComments(data)
	if err != nil {
		return cfg, err
	}

	if err := json.Unmarshal(cleaned, &cfg); err != nil {
		return cfg, err
	}

	cfg.normalize()
	return cfg, nil
}

func (s *configStore) save(cfg config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg.normalize()

	if err := s.ensureDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	data = append(data, '\n')
	return os.WriteFile(s.path, data, 0o644)
}

func (c *config) normalize() {
	if strings.TrimSpace(c.Emoji) == "" {
		c.Emoji = defaultEmoji
	}
	if c.OffsetMS < -5000 {
		c.OffsetMS = -5000
	}
	if c.OffsetMS > 5000 {
		c.OffsetMS = 5000
	}
}

func (c config) titlePrefix() string {
	if !c.ShowEmoji {
		return ""
	}
	return strings.TrimSpace(c.Emoji) + " "
}

func (c config) offsetDuration() time.Duration {
	return time.Duration(c.OffsetMS) * time.Millisecond
}

func stripJSONComments(input []byte) ([]byte, error) {
	var out bytes.Buffer

	inString := false
	escaped := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(input); i++ {
		ch := input[i]

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				out.WriteByte(ch)
			}
			continue
		}

		if inBlockComment {
			if ch == '*' && i+1 < len(input) && input[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if inString {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}

		if ch == '/' && i+1 < len(input) {
			next := input[i+1]
			if next == '/' {
				inLineComment = true
				i++
				continue
			}
			if next == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		out.WriteByte(ch)
	}

	if inBlockComment {
		return nil, errors.New("unterminated block comment in config file")
	}

	return out.Bytes(), nil
}
