-- okp 数据库初始化脚本
-- 用法: psql -d okp -f migrations/001_init.sql
-- 注意：GORM AutoMigrate 会在 API 启动时自动创建表结构，
-- 本脚本主要用于创建扩展和性能索引（部分 expression index 无法通过 GORM 自动创建）。

-- 扩展
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================
-- concepts 主干表
-- ============================================================
-- GORM 自动创建基础表结构，以下为补充索引：

-- trigram 索引：title 子串搜索（实体名精确命中）
CREATE INDEX IF NOT EXISTS idx_concepts_title_trgm
    ON concepts USING gin (title gin_trgm_ops);

-- tags GIN 索引：标签过滤
CREATE INDEX IF NOT EXISTS idx_concepts_tags_gin
    ON concepts USING gin (tags);

-- frontmatter jsonb 索引：scenario 等扩展键查询
CREATE INDEX IF NOT EXISTS idx_concepts_frontmatter
    ON concepts USING gin (frontmatter jsonb_path_ops);

-- domain + type + status 复合索引（结构导航，最常用）
CREATE INDEX IF NOT EXISTS idx_concepts_domain_type_status
    ON concepts (domain, type, status);

-- 活跃度时间窗口查询
CREATE INDEX IF NOT EXISTS idx_concepts_domain_created_at ON concepts (domain, created_at);
CREATE INDEX IF NOT EXISTS idx_concepts_domain_updated_at ON concepts (domain, updated_at);

-- ============================================================
-- links 关系表
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_links_from ON links (from_id);
CREATE INDEX IF NOT EXISTS idx_links_to ON links (to_id);

-- ============================================================
-- revisions 历史表
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_revisions_concept_rev
    ON revisions (concept_id, rev);

-- ============================================================
-- domain metadata
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_domain_meta_visibility
    ON domain_meta (visibility);

-- ============================================================
-- domain read statistics
-- ============================================================

-- Daily aggregate of successful domain-scoped knowledge GET requests.
-- No visitor, IP, or request identifiers are stored.
CREATE TABLE IF NOT EXISTS domain_read_stats (
    domain TEXT NOT NULL,
    date DATE NOT NULL,
    reads BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (domain, date)
);

-- ============================================================
-- 全文检索升级（可选，后续按需执行）
-- ============================================================

-- pgroonga（推荐：CJK 原生 n-gram，不依赖词典）
-- 安装: https://pgroonga.github.io/install/
-- CREATE EXTENSION IF NOT EXISTS pgroonga;
-- CREATE INDEX IF NOT EXISTS idx_concepts_pgroonga
--     ON concepts USING pgroonga (title, description, body);

-- pgvector（可选：语义 rerank）
-- CREATE EXTENSION IF NOT EXISTS vector;
-- ALTER TABLE concepts ADD COLUMN IF NOT EXISTS embedding vector(1536);
-- CREATE INDEX IF NOT EXISTS idx_concepts_embedding
--     ON concepts USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- ============================================================
-- 定期维护（建议 cron）
-- ============================================================

-- VACUUM ANALYZE concepts;   -- 每天
-- REINDEX TABLE concepts;    -- 每周
