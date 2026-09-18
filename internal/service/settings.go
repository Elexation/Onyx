package service

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/Elexation/onyx/internal/domain"
)

type SettingsRepo interface {
	Get(key string) (string, bool, error)
	Set(key, value string) error
	GetAll() (map[string]string, error)
}

type SettingsService struct {
	repo   SettingsRepo
	events EventRecorder
	// Get sits on hot paths (one call per upload create), so resolved values
	// are cached; every write invalidates the touched key.
	mu    sync.RWMutex
	cache map[string]string
	// Bumped by every invalidate so a Get whose repo read straddled a write
	// discards its now-stale result instead of latching it for the process
	// lifetime (nothing else ever evicts a cached key).
	gen uint64
}

func NewSettingsService(repo SettingsRepo) *SettingsService {
	return &SettingsService{repo: repo, cache: make(map[string]string)}
}

func (s *SettingsService) SetEvents(r EventRecorder) {
	s.events = r
}

func (s *SettingsService) Get(key string) (string, error) {
	s.mu.RLock()
	cached, ok := s.cache[key]
	gen := s.gen
	s.mu.RUnlock()
	if ok {
		return cached, nil
	}
	value, found, err := s.repo.Get(key)
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	if !found {
		value = domain.Defaults[key]
	}
	s.mu.Lock()
	if s.gen == gen {
		s.cache[key] = value
	}
	s.mu.Unlock()
	return value, nil
}

func (s *SettingsService) Set(key, value string) error {
	if err := s.repo.Set(key, value); err != nil {
		return err
	}
	s.invalidate(key)
	return nil
}

func (s *SettingsService) invalidate(key string) {
	s.mu.Lock()
	delete(s.cache, key)
	s.gen++
	s.mu.Unlock()
}

func (s *SettingsService) Update(updates map[string]string) (saved []string, errors map[string]string) {
	saved = make([]string, 0)
	errors = make(map[string]string)
	for key, value := range updates {
		if err := validateSetting(key, value); err != nil {
			errors[key] = err.Error()
			continue
		}
		if err := s.repo.Set(key, value); err != nil {
			errors[key] = "failed to save"
			continue
		}
		s.invalidate(key)
		saved = append(saved, key)
	}
	if len(saved) > 0 {
		recordIf(s.events, "settings.changed", SettingsChangedPayload{Keys: saved})
	}
	return saved, errors
}

func validateSetting(key, value string) error {
	if _, ok := domain.Defaults[key]; !ok {
		return fmt.Errorf("unknown setting")
	}

	switch key {
	case domain.SettingTrashEnabled, domain.SettingVersionsEnabled, domain.SettingSharesEnabled, domain.SettingServerTLSEnabled:
		if value != "true" && value != "false" {
			return fmt.Errorf("must be true or false")
		}

	case domain.SettingTrashPurgeAge, domain.SettingVersionsMaxAge:
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration format")
		}
		if d < 0 {
			return fmt.Errorf("must be 0 or positive")
		}
		if d > 8760*time.Hour {
			return fmt.Errorf("cannot exceed 8760 hours")
		}

	case domain.SettingSessionLifetime:
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration format")
		}
		if d < time.Hour {
			return fmt.Errorf("must be at least 1 hour")
		}
		if d > 720*time.Hour {
			return fmt.Errorf("cannot exceed 720 hours")
		}

	case domain.SettingVersionsMaxCount:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		if n < 0 {
			return fmt.Errorf("must be 0 or positive")
		}
		if n > 100 {
			return fmt.Errorf("cannot exceed 100")
		}

	case domain.SettingTrashMaxSize:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		if n < 0 {
			return fmt.Errorf("must be 0 or positive")
		}
		if n > 102400*1024*1024 {
			return fmt.Errorf("cannot exceed 102400 MB")
		}

	case domain.SettingVersionsMaxStorageBytes, domain.SettingVersionsMaxFileSize:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		if n < 0 {
			return fmt.Errorf("must be 0 or positive")
		}
		if n > 20480*1024*1024 {
			return fmt.Errorf("cannot exceed 20480 MB")
		}

	case domain.SettingUploadMaxSize:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		if n < 0 {
			return fmt.Errorf("must be 0 or positive")
		}
		if n > 102400*1024*1024 {
			return fmt.Errorf("cannot exceed 102400 MB")
		}

	case domain.SettingPlaybackDefaultQualityCeiling:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		switch n {
		case 0, 480, 720, 1080, 1440, 2160:
		default:
			return fmt.Errorf("must be one of 0, 480, 720, 1080, 1440, 2160")
		}

	case domain.SettingServerListenPort:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		if n < 1024 || n > 65535 {
			return fmt.Errorf("must be between 1024 and 65535")
		}

	}

	return nil
}

func (s *SettingsService) GetAll() (map[string]string, error) {
	stored, err := s.repo.GetAll()
	if err != nil {
		return nil, err
	}

	merged := make(map[string]string, len(domain.Defaults))
	for k, v := range domain.Defaults {
		merged[k] = v
	}
	for k, v := range stored {
		merged[k] = v
	}
	return merged, nil
}
