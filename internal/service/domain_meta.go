package service

import (
	"fmt"
	"strings"

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
	return &meta, nil
}

// PutDomainMeta writes or updates a domain README. When creating a domain, it
// grants the authenticated creator the domain's single host membership in the
// same transaction. Existing domains retain their original host.
func PutDomainMeta(domain, readme, creatorID string) (*model.DomainMeta, bool, error) {
	schema, err := parseReadmeSchema(readme)
	if err != nil {
		return nil, false, fmt.Errorf("README schema 解析失败: %w", err)
	}
	if creatorID == "" {
		return nil, false, fmt.Errorf("creator user id is required")
	}

	meta := model.DomainMeta{
		Domain: domain,
		Readme: readme,
		Schema: model.JSONMap{"fields": schema.Fields},
	}
	created := false

	err = store.DB.Transaction(func(tx *gorm.DB) error {
		var existing model.DomainMeta
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("domain = ?", domain).First(&existing).Error
		switch {
		case err == nil:
			return tx.Model(&existing).Updates(map[string]any{
				"readme": readme,
				"schema": meta.Schema,
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
