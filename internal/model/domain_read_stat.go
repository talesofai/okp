package model

// DomainReadStat stores one UTC day's successful knowledge-read requests for a
// domain. It intentionally contains no visitor or request identifiers.
type DomainReadStat struct {
	Domain string `gorm:"primaryKey;type:text" json:"domain"`
	Date   string `gorm:"primaryKey;type:date" json:"date"`
	Reads  int64  `gorm:"not null;default:0" json:"reads"`
}

func (DomainReadStat) TableName() string {
	return "domain_read_stats"
}
