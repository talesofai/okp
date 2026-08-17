package service

import (
	"fmt"
	"time"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RecordDomainRead increments the UTC daily aggregate for a successful
// domain-scoped knowledge read. It deliberately stores no visitor identity.
func RecordDomainRead(domain string) error {
	return recordDomainReadAt(domain, time.Now().UTC())
}

func recordDomainReadAt(domain string, at time.Time) error {
	if domain == "" {
		return fmt.Errorf("domain is required")
	}

	stat := model.DomainReadStat{
		Domain: domain,
		Date:   at.UTC().Format("2006-01-02"),
		Reads:  1,
	}
	return store.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "domain"}, {Name: "date"}},
		DoUpdates: clause.Assignments(map[string]any{
			"reads": gorm.Expr("domain_read_stats.reads + ?", 1),
		}),
	}).Create(&stat).Error
}
