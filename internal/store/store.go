package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound 表示目标记录不存在。
var ErrNotFound = errors.New("记录不存在")

// Store 封装 SQLite 连接与全部数据操作。
type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS prompts (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  title       TEXT    NOT NULL DEFAULT '',
  content     TEXT    NOT NULL DEFAULT '',
  negative    TEXT    NOT NULL DEFAULT '',
  tags        TEXT    NOT NULL DEFAULT '',
  source      TEXT    NOT NULL DEFAULT '',
  enabled     INTEGER NOT NULL DEFAULT 1,
  favorite    INTEGER NOT NULL DEFAULT 0,
  preview_url TEXT    NOT NULL DEFAULT '',
  note        TEXT    NOT NULL DEFAULT '',
  sort_order  INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT    NOT NULL DEFAULT '',
  updated_at  TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_prompts_sort    ON prompts(sort_order, id);
CREATE INDEX IF NOT EXISTS idx_prompts_updated ON prompts(updated_at);
CREATE INDEX IF NOT EXISTS idx_prompts_enabled ON prompts(enabled);
CREATE INDEX IF NOT EXISTS idx_prompts_fav     ON prompts(favorite);
CREATE TABLE IF NOT EXISTS prompt_tags (
  prompt_id INTEGER NOT NULL,
  tag       TEXT    NOT NULL,
  PRIMARY KEY (prompt_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_prompt_tags_tag ON prompt_tags(tag);
CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`

// Open 打开（必要时创建）数据库并完成建表。
func Open(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("数据库路径为空")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	// SQLite 是单写者：串行化到 1 条连接，彻底避免 "database is locked"。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	// 按分号切分，逐条执行：modernc 驱动对多语句 Exec 的支持不保证。
	for _, stmt := range strings.Split(schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("建表失败: %w", err)
		}
	}
	return nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error { return s.db.Close() }

// Ping 检查数据库可用性。
func (s *Store) Ping() error { return s.db.Ping() }

func scanPrompt(sc interface{ Scan(...any) error }) (Prompt, error) {
	var (
		p    Prompt
		tags string
		en   int
		fav  int
	)
	err := sc.Scan(&p.ID, &p.Title, &p.Content, &p.Negative, &tags, &p.Source,
		&en, &fav, &p.PreviewURL, &p.Note, &p.SortOrder, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return p, err
	}
	p.Enabled = en != 0
	p.Favorite = fav != 0
	p.Tags = ParseTags(tags)
	return p, nil
}

// List 按过滤条件分页查询，返回结果与命中总数。
func (s *Store) List(f Filter) ([]Prompt, int, error) {
	where, args := f.buildWhere()

	var total int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM prompts p "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []Prompt{}, 0, nil
	}

	query := "SELECT " + promptColumns + " FROM prompts p " + where +
		" ORDER BY " + f.orderClause() + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), f.limit(), f.offset())

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]Prompt, 0, f.limit())
	for rows.Next() {
		p, err := scanPrompt(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Get 按 ID 取出一条记录。
func (s *Store) Get(id int64) (*Prompt, error) {
	row := s.db.QueryRow("SELECT "+promptColumns+" FROM prompts p WHERE p.id = ?", id)
	p, err := scanPrompt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Create 新建一条记录。
func (s *Store) Create(in PromptInput) (*Prompt, error) {
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return nil, errors.New("提示词内容不能为空")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = deriveTitle(content)
	}
	tags := NormalizeTags(in.Tags)
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	favorite := false
	if in.Favorite != nil {
		favorite = *in.Favorite
	}
	now := nowRFC3339()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	sortOrder := 0
	if in.SortOrder != nil {
		sortOrder = *in.SortOrder
	} else if err := tx.QueryRow("SELECT COALESCE(MAX(sort_order), 0) + 1 FROM prompts").Scan(&sortOrder); err != nil {
		return nil, err
	}

	res, err := tx.Exec(`INSERT INTO prompts
		(title, content, negative, tags, source, enabled, favorite, preview_url, note, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		title, content, in.Negative, JoinTags(tags), strings.TrimSpace(in.Source),
		boolToInt(enabled), boolToInt(favorite), strings.TrimSpace(in.PreviewURL),
		in.Note, sortOrder, now, now)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if err := rebuildTags(tx, id, tags); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Get(id)
}

// Update 全量更新一条记录。
func (s *Store) Update(id int64, in PromptInput) (*Prompt, error) {
	if _, err := s.Get(id); err != nil {
		return nil, err
	}
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return nil, errors.New("提示词内容不能为空")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = deriveTitle(content)
	}
	tags := NormalizeTags(in.Tags)
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	favorite := false
	if in.Favorite != nil {
		favorite = *in.Favorite
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if in.SortOrder != nil {
		if _, err := tx.Exec("UPDATE prompts SET sort_order = ? WHERE id = ?", *in.SortOrder, id); err != nil {
			return nil, err
		}
	}

	_, err = tx.Exec(`UPDATE prompts SET
		title = ?, content = ?, negative = ?, tags = ?, source = ?,
		enabled = ?, favorite = ?, preview_url = ?, note = ?, updated_at = ?
		WHERE id = ?`,
		title, content, in.Negative, JoinTags(tags), strings.TrimSpace(in.Source),
		boolToInt(enabled), boolToInt(favorite), strings.TrimSpace(in.PreviewURL),
		in.Note, nowRFC3339(), id)
	if err != nil {
		return nil, err
	}
	if err := rebuildTags(tx, id, tags); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Get(id)
}

// Delete 删除一条记录。
func (s *Store) Delete(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec("DELETE FROM prompts WHERE id = ?", id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec("DELETE FROM prompt_tags WHERE prompt_id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

// SetFavorite 设置收藏状态。
func (s *Store) SetFavorite(id int64, favorite bool) (*Prompt, error) {
	res, err := s.db.Exec("UPDATE prompts SET favorite = ? WHERE id = ?", boolToInt(favorite), id)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		if _, err := s.Get(id); err != nil {
			return nil, err
		}
	}
	return s.Get(id)
}

// SetEnabled 启用 / 停用一条记录。
func (s *Store) SetEnabled(id int64, enabled bool) (*Prompt, error) {
	res, err := s.db.Exec("UPDATE prompts SET enabled = ? WHERE id = ?", boolToInt(enabled), id)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		if _, err := s.Get(id); err != nil {
			return nil, err
		}
	}
	return s.Get(id)
}

// Tags 返回全部标签及其出现次数。
func (s *Store) Tags() ([]TagCount, error) {
	rows, err := s.db.Query(`SELECT pt.tag, COUNT(*) AS c
		FROM prompt_tags pt JOIN prompts p ON p.id = pt.prompt_id
		GROUP BY pt.tag ORDER BY c DESC, pt.tag ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TagCount{}
	for rows.Next() {
		var t TagCount
		if err := rows.Scan(&t.Tag, &t.Count); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Stats 返回站点概览。
func (s *Store) Stats() (Stats, error) {
	var st Stats
	if err := s.db.QueryRow(`SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN enabled = 1 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN enabled = 0 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(favorite), 0),
		COALESCE(MAX(updated_at), '')
		FROM prompts`).Scan(&st.Total, &st.Enabled, &st.Disabled, &st.Favorite, &st.UpdatedAt); err != nil {
		return st, err
	}
	if err := s.db.QueryRow("SELECT COUNT(DISTINCT tag) FROM prompt_tags").Scan(&st.TagCount); err != nil {
		return st, err
	}
	rows, err := s.db.Query(`SELECT source, COUNT(*) AS c FROM prompts
		GROUP BY source ORDER BY c DESC, source ASC`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	st.Sources = []TagCount{}
	for rows.Next() {
		var sc TagCount
		if err := rows.Scan(&sc.Tag, &sc.Count); err != nil {
			return st, err
		}
		st.Sources = append(st.Sources, sc)
	}
	return st, rows.Err()
}

// All 返回全部记录（导出用）。
func (s *Store) All() ([]Prompt, error) {
	rows, err := s.db.Query("SELECT " + promptColumns + " FROM prompts p ORDER BY p.sort_order ASC, p.id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Prompt{}
	for rows.Next() {
		p, err := scanPrompt(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Count 返回记录总数。
func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM prompts").Scan(&n)
	return n, err
}

// Import 批量导入记录；replace 为 true 时先清空。
func (s *Store) Import(recs []ImportRecord, replace bool) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if replace {
		if _, err := tx.Exec("DELETE FROM prompts"); err != nil {
			return 0, err
		}
		if _, err := tx.Exec("DELETE FROM prompt_tags"); err != nil {
			return 0, err
		}
		if _, err := tx.Exec("DELETE FROM sqlite_sequence WHERE name = 'prompts'"); err != nil {
			return 0, err
		}
	}

	var nextOrder int
	if err := tx.QueryRow("SELECT COALESCE(MAX(sort_order), 0) FROM prompts").Scan(&nextOrder); err != nil {
		return 0, err
	}

	inserted := 0
	for _, r := range recs {
		content := strings.TrimSpace(r.Content)
		if content == "" {
			continue
		}
		title := strings.TrimSpace(r.Title)
		if title == "" {
			title = deriveTitle(content)
		}
		tags := NormalizeTags(r.Tags)
		enabled := true
		if r.Enabled != nil {
			enabled = *r.Enabled
		}
		favorite := false
		if r.Favorite != nil {
			favorite = *r.Favorite
		}
		created := normalizeTime(r.CreatedAt)
		updated := normalizeTime(r.UpdatedAt)
		if updated == "" {
			updated = created
		}
		if created == "" {
			created = updated
		}

		nextOrder++
		order := nextOrder
		if r.SortOrder != nil {
			order = *r.SortOrder
		}

		res, err := tx.Exec(`INSERT INTO prompts
			(title, content, negative, tags, source, enabled, favorite, preview_url, note, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			title, content, r.Negative, JoinTags(tags), strings.TrimSpace(r.Source),
			boolToInt(enabled), boolToInt(favorite), strings.TrimSpace(r.PreviewURL),
			r.Note, order, created, updated)
		if err != nil {
			return inserted, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return inserted, err
		}
		if err := rebuildTags(tx, id, tags); err != nil {
			return inserted, err
		}
		inserted++
	}
	if err := tx.Commit(); err != nil {
		return inserted, err
	}
	return inserted, nil
}

// SeedIfEmpty 仅在数据库为空时写入种子数据。
func (s *Store) SeedIfEmpty(recs []ImportRecord) (int, error) {
	n, err := s.Count()
	if err != nil {
		return 0, err
	}
	if n > 0 || len(recs) == 0 {
		return 0, nil
	}
	return s.Import(recs, false)
}

// Setting 读取一个配置项。
func (s *Store) Setting(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

// SetSetting 写入一个配置项。
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// AuthEpoch 返回当前鉴权纪元，密码变更后自增即可让旧会话全部失效。
func (s *Store) AuthEpoch() (int64, error) {
	v, ok, err := s.Setting("auth_epoch")
	if err != nil || !ok || v == "" {
		return 0, err
	}
	var epoch int64
	if _, err := fmt.Sscanf(v, "%d", &epoch); err != nil {
		return 0, nil
	}
	return epoch, nil
}

// BumpAuthEpoch 让当前所有会话失效。
func (s *Store) BumpAuthEpoch() (int64, error) {
	cur, err := s.AuthEpoch()
	if err != nil {
		return 0, err
	}
	next := cur + 1
	if err := s.SetSetting("auth_epoch", fmt.Sprintf("%d", next)); err != nil {
		return 0, err
	}
	return next, nil
}

func rebuildTags(tx *sql.Tx, id int64, tags []string) error {
	if _, err := tx.Exec("DELETE FROM prompt_tags WHERE prompt_id = ?", id); err != nil {
		return err
	}
	for _, t := range tags {
		if _, err := tx.Exec("INSERT OR IGNORE INTO prompt_tags (prompt_id, tag) VALUES (?, ?)", id, t); err != nil {
			return err
		}
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nowRFC3339() string { return time.Now().Format(time.RFC3339) }

// deriveTitle 在标题为空时从正文截取一个可读标题。
func deriveTitle(content string) string {
	line := strings.TrimSpace(strings.SplitN(content, "\n", 2)[0])
	line = strings.TrimSpace(strings.Trim(line, "#"))
	runes := []rune(line)
	if len(runes) > 24 {
		return string(runes[:24]) + "…"
	}
	if line == "" {
		return "未命名提示词"
	}
	return line
}

// normalizeTime 把多种时间写法统一成 RFC3339。
func normalizeTime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999 -0700",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format(time.RFC3339)
		}
	}
	return ""
}
