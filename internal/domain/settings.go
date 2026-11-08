package domain

import (
	"time"

	"github.com/Andrew-Sem/gc/specs/contract"
)

const (
	MaxPace  = 168
	MaxLimit = 100
)

type Settings struct {
	Pace  int
	Limit int
	saved time.Time
}

var DefaultSettings = Settings{Pace: 1, Limit: 12}

func RepositorySettings(facts []contract.Fact, repository string) Settings {
	settings := DefaultSettings
	for _, f := range facts {
		if changed, ok := f.(contract.RepositorySettingsChanged); ok && changed.Repository == repository {
			settings = Settings{Pace: changed.Pace, Limit: changed.Limit, saved: changed.At}
		}
	}
	return settings
}

func (s Settings) Same(pace, limit int) bool {
	return s.Pace == pace && s.Limit == limit
}

func (s Settings) Due(wakeUp time.Time) bool {
	if s.saved.IsZero() {
		return true
	}
	hours := int(wakeUp.Truncate(time.Hour).Sub(s.saved.Truncate(time.Hour)) / time.Hour)
	return hours%s.Pace == 0
}
