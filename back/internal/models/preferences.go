package models

import "time"

type ThemeMode string

const (
	ThemeDark   ThemeMode = "dark"
	ThemeLight  ThemeMode = "light"
	ThemeSystem ThemeMode = "system"
)

func IsThemeMode(mode ThemeMode) bool {
	switch mode {
	case ThemeDark, ThemeLight, ThemeSystem:
		return true
	}
	return false
}

type Preferences struct {
	ThemeMode ThemeMode `json:"themeMode"`
	Accent    string    `json:"accent"`
	BlendA    string    `json:"blendA"`
	BlendB    string    `json:"blendB"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func DefaultPreferences() Preferences {
	return Preferences{
		ThemeMode: ThemeDark,
		Accent:    "#d4a72c",
		BlendA:    "#e01e37",
		BlendB:    "#d4a72c",
	}
}
