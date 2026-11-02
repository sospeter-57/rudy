package services

import (
	"context"
	"regexp"
	"strings"

	"rudy/internal/models"
	"rudy/internal/transaction"
)

type PreferencesService struct {
	db *transaction.DB
}

func NewPreferencesService(db *transaction.DB) *PreferencesService {
	return &PreferencesService{db: db}
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (s *PreferencesService) Get(ctx context.Context, userID int64) (models.Preferences, error) {
	return s.db.Repo().PreferencesByUser(ctx, userID)
}

func (s *PreferencesService) Update(ctx context.Context, userID int64, preferences models.Preferences) (models.Preferences, error) {
	if !models.IsThemeMode(preferences.ThemeMode) {
		return models.Preferences{}, models.Validation("theme mode must be dark, light or system")
	}
	preferences.Accent = strings.TrimSpace(preferences.Accent)
	preferences.BlendA = strings.TrimSpace(preferences.BlendA)
	preferences.BlendB = strings.TrimSpace(preferences.BlendB)
	for name, value := range map[string]string{
		"accent": preferences.Accent,
		"blendA": preferences.BlendA,
		"blendB": preferences.BlendB,
	} {
		if !hexColor.MatchString(value) {
			return models.Preferences{}, models.Validation("%s must be a six digit hex color", name)
		}
	}

	now := nowUTC()
	if err := s.db.Repo().UpsertPreferences(ctx, userID, preferences, now); err != nil {
		return models.Preferences{}, err
	}
	return s.db.Repo().PreferencesByUser(ctx, userID)
}
