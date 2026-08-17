package service

import (
	"fmt"
	"strings"

	"github.com/talesofai/okp/internal/access"
	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/store"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetDomainMeta 获取 domain 的 README + schema
func GetDomainMeta(domain string) (*model.DomainMeta, error) {
	var meta model.DomainMeta
	if err := store.DB.Where("domain = ?", domain).First(&meta).Error; err != nil {
		return nil, err
	}
	if meta.CreatedAt.IsZero() {
		var first model.Concept
		err := store.DB.Select("created_at").Where("domain = ?", domain).Order("created_at ASC").First(&first).Error
		switch {
		case err == nil && !first.CreatedAt.IsZero():
			meta.CreatedAt = first.CreatedAt
		case err == gorm.ErrRecordNotFound:
			meta.CreatedAt = meta.UpdatedAt
		case err != nil:
			return nil, err
		}
	}
	return &meta, nil
}

// DomainExists reports whether a domain has metadata or at least one concept.
func DomainExists(domain string) (bool, error) {
	var count int64
	if err := store.DB.Model(&model.DomainMeta{}).Where("domain = ?", domain).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := store.DB.Model(&model.Concept{}).Where("domain = ?", domain).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// PutDomainMeta writes or updates a domain README. When creating a domain, it
// grants the authenticated creator the domain's single host membership in the
// same transaction. Existing domains retain their original host.
func PutDomainMeta(domain, readme, visibility, creatorID string) (*model.DomainMeta, bool, error) {
	schema, err := parseReadmeSchema(readme)
	if err != nil {
		return nil, false, fmt.Errorf("README schema 解析失败: %w", err)
	}
	if !access.IsValidVisibility(visibility) {
		return nil, false, fmt.Errorf("visibility must be 'public' or 'private'")
	}
	if creatorID == "" {
		return nil, false, fmt.Errorf("creator user id is required")
	}

	meta := model.DomainMeta{
		Domain:     domain,
		Readme:     readme,
		Schema:     model.JSONMap{"fields": schema.Fields},
		Visibility: visibility,
	}
	created := false

	err = store.DB.Transaction(func(tx *gorm.DB) error {
		var existing model.DomainMeta
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("domain = ?", domain).First(&existing).Error
		switch {
		case err == nil:
			return tx.Model(&existing).Updates(map[string]any{
				"readme":     readme,
				"schema":     meta.Schema,
				"visibility": visibility,
			}).Error
		case err != gorm.ErrRecordNotFound:
			return err
		}

		created = true
		if err := tx.Create(&meta).Error; err != nil {
			return err
		}
		return tx.Create(&model.DomainMember{
			Domain: domain,
			UserID: creatorID,
			Role:   "host",
		}).Error
	})
	if err != nil {
		return nil, false, err
	}
	if !created {
		if err := store.DB.Where("domain = ?", domain).First(&meta).Error; err != nil {
			return nil, false, err
		}
	}
	return &meta, created, nil
}

// DeleteDomain removes a domain and all data owned by it in one transaction.
func DeleteDomain(domain string) error {
	return store.DB.Transaction(func(tx *gorm.DB) error {
		var meta model.DomainMeta
		metaErr := tx.Where("domain = ?", domain).First(&meta).Error
		if metaErr != nil && metaErr != gorm.ErrRecordNotFound {
			return metaErr
		}

		var conceptCount int64
		if err := tx.Model(&model.Concept{}).Where("domain = ?", domain).Count(&conceptCount).Error; err != nil {
			return err
		}
		if metaErr == gorm.ErrRecordNotFound && conceptCount == 0 {
			return gorm.ErrRecordNotFound
		}

		if conceptCount > 0 {
			conceptIDs := tx.Model(&model.Concept{}).Select("id").Where("domain = ?", domain)
			if err := tx.Where(`
				from_id IN (?) OR to_id IN (?)
				OR substr(from_id, 1, length(?) + 1) = ? || '/'
				OR substr(to_id, 1, length(?) + 1) = ? || '/'
			`, conceptIDs, conceptIDs, domain, domain, domain, domain).Delete(&model.Link{}).Error; err != nil {
				return err
			}
			if err := tx.Where("concept_id IN (?)", conceptIDs).Delete(&model.Revision{}).Error; err != nil {
				return err
			}
			if err := tx.Where("domain = ?", domain).Delete(&model.Concept{}).Error; err != nil {
				return err
			}
		} else if err := tx.Where(`
			substr(from_id, 1, length(?) + 1) = ? || '/'
			OR substr(to_id, 1, length(?) + 1) = ? || '/'
		`, domain, domain, domain, domain).Delete(&model.Link{}).Error; err != nil {
			return err
		}
		if err := tx.Where("domain = ?", domain).Delete(&model.DomainInvite{}).Error; err != nil {
			return err
		}
		if err := tx.Where("domain = ?", domain).Delete(&model.DomainMember{}).Error; err != nil {
			return err
		}
		if metaErr == nil {
			return tx.Delete(&meta).Error
		}
		return nil
	})
}

// ValidateFrontmatter 按 domain schema 校验 concept 的 frontmatter
// 返回校验错误列表（空表示通过）
func ValidateFrontmatter(domain string, frontmatter map[string]any) []string {
	var meta model.DomainMeta
	if err := store.DB.Where("domain = ?", domain).First(&meta).Error; err != nil {
		return nil // 无 schema，放行
	}

	rawFields, ok := meta.Schema["fields"]
	if !ok || rawFields == nil {
		return nil
	}

	// 重新序列化再解析到 FrontmatterSchema
	b, _ := yaml.Marshal(rawFields)
	var schema model.FrontmatterSchema
	if err := yaml.Unmarshal(b, &schema); err != nil {
		return nil
	}

	var errs []string
	for fieldName, def := range schema.Fields {
		val, exists := frontmatter[fieldName]
		if def.Required && (!exists || val == nil || val == "") {
			errs = append(errs, fmt.Sprintf("frontmatter.%s 是必填字段 (%s)", fieldName, def.Description))
			continue
		}
		if !exists || val == nil {
			continue
		}
		// enum 校验
		if len(def.Enum) > 0 {
			strVal := fmt.Sprintf("%v", val)
			valid := false
			for _, e := range def.Enum {
				if strVal == e {
					valid = true
					break
				}
			}
			if !valid {
				errs = append(errs, fmt.Sprintf("frontmatter.%s 值 %q 不在允许范围 %v 内", fieldName, strVal, def.Enum))
			}
		}
	}
	return errs
}

// parseReadmeSchema 从 README markdown 解析 YAML frontmatter 中的 schema
// 支持 --- ... --- 块
func parseReadmeSchema(readme string) (model.FrontmatterSchema, error) {
	var schema model.FrontmatterSchema
	if !strings.HasPrefix(strings.TrimSpace(readme), "---") {
		return schema, nil // 无 frontmatter，返回空 schema
	}

	// 提取 --- 和 --- 之间的 YAML
	parts := strings.SplitN(strings.TrimSpace(readme), "---", 3)
	if len(parts) < 3 {
		return schema, nil
	}
	yamlBlock := parts[1]

	if err := yaml.Unmarshal([]byte(yamlBlock), &schema); err != nil {
		return schema, err
	}
	return schema, nil
}
