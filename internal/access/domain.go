package access

import (
	"fmt"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
	"gorm.io/gorm"
)

const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

// IsValidVisibility validates the only two visibility values accepted by the API.
func IsValidVisibility(visibility string) bool {
	return visibility == VisibilityPublic || visibility == VisibilityPrivate
}

func isAdmin(db *gorm.DB, userID string) bool {
	if db == nil || userID == "" {
		return false
	}
	var user model.User
	if err := db.Select("role").Where("uuid = ?", userID).First(&user).Error; err != nil {
		return false
	}
	return user.Role == "admin"
}

func domainVisibility(db *gorm.DB, domain string) (string, error) {
	if db == nil {
		return "", fmt.Errorf("database is not initialized")
	}
	var meta model.DomainMeta
	err := db.Select("visibility").Where("domain = ?", domain).First(&meta).Error
	if err == gorm.ErrRecordNotFound {
		return VisibilityPublic, nil
	}
	if err != nil {
		return "", err
	}
	if meta.Visibility == "" {
		return VisibilityPublic, nil
	}
	return meta.Visibility, nil
}

func membershipRole(db *gorm.DB, userID, domain string) (string, bool) {
	if db == nil || userID == "" {
		return "", false
	}
	var member model.DomainMember
	if err := db.Select("role").Where("domain = ? AND user_id = ?", domain, userID).First(&member).Error; err != nil {
		return "", false
	}
	return member.Role, true
}

// IsDomainHost only considers explicit ownership. A global admin is not a host.
func IsDomainHost(userID, domain string) bool {
	role, ok := membershipRole(store.DB, userID, domain)
	return ok && role == "host"
}

// CanReadDomain applies the public/private boundary. Global admins have no
// implicit access to private domains.
func CanReadDomain(userID, domain string) bool {
	visibility, err := domainVisibility(store.DB, domain)
	if err != nil {
		return false
	}
	if visibility != VisibilityPrivate {
		return true
	}
	role, ok := membershipRole(store.DB, userID, domain)
	return ok && (role == "host" || role == "writer" || role == "reader")
}

// CanWriteDomain grants public-domain writes to global admins and grants
// private-domain writes only through an explicit host/writer membership.
func CanWriteDomain(userID, domain string) bool {
	visibility, err := domainVisibility(store.DB, domain)
	if err != nil {
		return false
	}
	role, _ := membershipRole(store.DB, userID, domain)
	if role == "host" || role == "writer" {
		return true
	}
	return visibility != VisibilityPrivate && isAdmin(store.DB, userID)
}

// CanManageDomain grants domain administration to the explicit host. Global
// admins may manage public domains, but never receive an override on private ones.
func CanManageDomain(userID, domain string) bool {
	visibility, err := domainVisibility(store.DB, domain)
	if err != nil {
		return false
	}
	role, _ := membershipRole(store.DB, userID, domain)
	if role == "host" {
		return true
	}
	return visibility != VisibilityPrivate && isAdmin(store.DB, userID)
}

// GetUserDomains returns explicit memberships, including private domains the
// user can access.
func GetUserDomains(userID string) []model.DomainMember {
	if userID == "" || store.DB == nil {
		return nil
	}
	var members []model.DomainMember
	store.DB.Where("user_id = ?", userID).Order("domain ASC").Find(&members)
	return members
}

// ScopeReadableConcepts filters a concepts query to public domains plus private
// domains where the caller has an explicit membership.
func ScopeReadableConcepts(userID string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(`
			NOT EXISTS (
				SELECT 1 FROM domain_meta dm
				WHERE dm.domain = concepts.domain AND dm.visibility = 'private'
			)
			OR EXISTS (
				SELECT 1 FROM domain_members dmbr
				WHERE dmbr.domain = concepts.domain AND dmbr.user_id = ?
				AND dmbr.role IN ('host', 'writer', 'reader')
			)
		`, userID)
	}
}

// ScopeWritableConcepts filters maintenance writes to domains where the caller
// is an explicit host/writer, plus public domains for global admins.
func ScopeWritableConcepts(userID string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where(`
			EXISTS (
				SELECT 1 FROM domain_members dmbr
				WHERE dmbr.domain = concepts.domain AND dmbr.user_id = ?
				AND dmbr.role IN ('host', 'writer')
			)
			OR (
				NOT EXISTS (
					SELECT 1 FROM domain_meta dm
					WHERE dm.domain = concepts.domain AND dm.visibility = 'private'
				)
				AND EXISTS (
					SELECT 1 FROM users u
					WHERE u.uuid = ? AND u.role = 'admin'
				)
			)
		`, userID, userID)
	}
}
