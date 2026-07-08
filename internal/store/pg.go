package store

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/talesofai/okp/internal/config"
	"github.com/talesofai/okp/internal/model"
	"gorm.io/driver/postgres"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// DB 全局数据库实例
var DB *gorm.DB

// IsSQLite 当前是否为 SQLite 模式
var IsSQLite bool

// Init 初始化数据库连接并执行自动迁移。
func Init() {
	level := gormLogger.Warn
	if config.C.LogLevel == "debug" {
		level = gormLogger.Info
	}

	gormCfg := &gorm.Config{
		Logger:                 gormLogger.Default.LogMode(level),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}

	var err error
	IsSQLite = config.C.DatabaseType == "sqlite"

	if IsSQLite {
		path := config.C.SQLitePath
		// 确保目录存在
		dir := filepath.Dir(path)
		if dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				slog.Error("创建 SQLite 目录失败", "error", err)
				panic(err)
			}
		}
		slog.Info("使用 SQLite", "path", path)
		DB, err = gorm.Open(sqlite.Open(path), gormCfg)
	} else {
		slog.Info("使用 PostgreSQL")
		DB, err = gorm.Open(postgres.Open(config.C.DatabaseURL), gormCfg)
	}

	if err != nil {
		slog.Error("数据库连接失败", "error", err)
		panic(err)
	}

	slog.Info("数据库已连接")

	// 自动迁移 + 清理废弃字段
	if err := DB.AutoMigrate(
		&model.Concept{},
		&model.Link{},
		&model.Revision{},
		&model.User{},
		&model.DomainMeta{},
	); err != nil {
		slog.Error("数据库迁移失败", "error", err)
		panic(err)
	}

	// 移除 status 列（废弃）
	if DB.Migrator().HasColumn(&model.Concept{}, "status") {
		if err := DB.Migrator().DropColumn(&model.Concept{}, "status"); err != nil {
			slog.Warn("删除 status 列失败", "error", err)
		}
	}

	if IsSQLite {
		// SQLite: 创建基础索引
		_ = DB.Exec("CREATE INDEX IF NOT EXISTS idx_concepts_domain_type ON concepts (domain, type)").Error
		_ = DB.Exec("CREATE INDEX IF NOT EXISTS idx_concepts_title ON concepts (title)").Error
		_ = DB.Exec("CREATE INDEX IF NOT EXISTS idx_links_from ON links (from_id)").Error
		_ = DB.Exec("CREATE INDEX IF NOT EXISTS idx_links_to ON links (to_id)").Error
	} else {
		// PostgreSQL: 确保 UTF-8 编码
		_ = DB.Exec("SET client_encoding = 'UTF8'").Error

		// PostgreSQL: 创建扩展和索引
		_ = DB.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error
		_ = DB.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`).Error
		// pg_trgm trigram 索引：对中文按字符三元组工作，支持相似度搜索和 ILIKE
		_ = DB.Exec(`CREATE INDEX IF NOT EXISTS idx_concepts_title_trgm ON concepts USING gin (title gin_trgm_ops)`).Error
		_ = DB.Exec(`CREATE INDEX IF NOT EXISTS idx_concepts_tags_gin ON concepts USING gin (tags)`).Error
		// 复合索引：domain + type（领域过滤查询优化）
		_ = DB.Exec(`CREATE INDEX IF NOT EXISTS idx_concepts_domain_type ON concepts (domain, type)`).Error
		// 状态 + 领域索引：加速 domains 列表、领域统计查询
		// frontmatter GIN 全列索引（支持所有 key 的 @> 包含查询）
		_ = DB.Exec(`CREATE INDEX IF NOT EXISTS idx_concepts_frontmatter_gin ON concepts USING gin (frontmatter)`).Error
	}

	slog.Info("数据库迁移完成")
}
