package service

import (
	"fmt"
	"time"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
)

const maxActivityDays = 365

// DomainActivityPoint is one UTC calendar-day activity bucket.
type DomainActivityPoint struct {
	Date     string `json:"date"`
	Created  int64  `json:"created"`
	Updated  int64  `json:"updated"`
	Activity int64  `json:"activity"`
}

// DomainActivity is the time series used by the portal's activity chart.
type DomainActivity struct {
	Domain string                `json:"domain"`
	Days   int                   `json:"days"`
	From   string                `json:"from"`
	To     string                `json:"to"`
	Points []DomainActivityPoint `json:"points"`
}

type conceptActivityDates struct {
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// GetDomainActivity groups current concepts by created_at and their latest
// updated_at. The range includes today and is bounded for predictable queries.
func GetDomainActivity(domain string, days int) (DomainActivity, error) {
	if domain == "" {
		return DomainActivity{}, fmt.Errorf("domain is required")
	}
	if days < 1 || days > maxActivityDays {
		return DomainActivity{}, fmt.Errorf("days must be between 1 and %d", maxActivityDays)
	}
	return getDomainActivityAt(domain, days, time.Now().UTC())
}

func getDomainActivityAt(domain string, days int, now time.Time) (DomainActivity, error) {
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, -(days - 1))
	end := today.AddDate(0, 0, 1)

	points := make([]DomainActivityPoint, days)
	for i := range points {
		points[i].Date = start.AddDate(0, 0, i).Format("2006-01-02")
	}

	var rows []conceptActivityDates
	if err := store.DB.Model(&model.Concept{}).
		Select("created_at, updated_at").
		Where("domain = ? AND ((created_at >= ? AND created_at < ?) OR (updated_at >= ? AND updated_at < ?))", domain, start, end, start, end).
		Find(&rows).Error; err != nil {
		return DomainActivity{}, err
	}

	bucket := func(value time.Time) int {
		value = value.UTC()
		if value.Before(start) || !value.Before(end) {
			return -1
		}
		return int(value.Sub(start) / (24 * time.Hour))
	}
	for _, row := range rows {
		if i := bucket(row.CreatedAt); i >= 0 {
			points[i].Created++
		}
		// A freshly inserted row has created_at == updated_at and counts once,
		// as a creation rather than as a second update.
		if row.UpdatedAt.After(row.CreatedAt) {
			if i := bucket(row.UpdatedAt); i >= 0 {
				points[i].Updated++
			}
		}
	}
	for i := range points {
		points[i].Activity = points[i].Created + points[i].Updated
	}

	return DomainActivity{
		Domain: domain,
		Days:   days,
		From:   points[0].Date,
		To:     points[len(points)-1].Date,
		Points: points,
	}, nil
}
