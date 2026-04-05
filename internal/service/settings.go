package service

import (
	"fmt"

	"github.com/Elexation/onyx/internal/domain"
)

type SettingsRepo interface {
	Get(key string) (string, bool, error)
	Set(key, value string) error
	GetAll() (map[string]string, error)
}

type SettingsService struct {
	repo SettingsRepo
}

func NewSettingsService(repo SettingsRepo) *SettingsService {
	return &SettingsService{repo: repo}
}

func (s *SettingsService) Get(key string) (string, error) {
	value, found, err := s.repo.Get(key)
	if err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	if found {
		return value, nil
	}
	if def, ok := domain.Defaults[key]; ok {
		return def, nil
	}
	return "", nil
}

func (s *SettingsService) Set(key, value string) error {
	return s.repo.Set(key, value)
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
