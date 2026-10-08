package store

import "strings"

// Prompt 是数据库中一条提示词记录的对外形态。
type Prompt struct {
	ID         int64    `json:"id"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Negative   string   `json:"negative"`
	Tags       []string `json:"tags"`
	Source     string   `json:"source"`
	Enabled    bool     `json:"enabled"`
	Favorite   bool     `json:"favorite"`
	PreviewURL string   `json:"preview_url"`
	Note       string   `json:"note"`
	SortOrder  int      `json:"sort_order"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

// PromptInput 是写入接口（POST/PUT）接收的载荷。
type PromptInput struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Negative   string   `json:"negative"`
	Tags       []string `json:"tags"`
	Source     string   `json:"source"`
	Enabled    *bool    `json:"enabled"`
	Favorite   *bool    `json:"favorite"`
	PreviewURL string   `json:"preview_url"`
	Note       string   `json:"note"`
	SortOrder  *int     `json:"sort_order"`
}

// ImportRecord 是导入 / 种子数据使用的载荷，额外允许指定时间戳。
type ImportRecord struct {
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	Negative   string   `json:"negative"`
	Tags       []string `json:"tags"`
	Source     string   `json:"source"`
	Enabled    *bool    `json:"enabled"`
	Favorite   *bool    `json:"favorite"`
	PreviewURL string   `json:"preview_url"`
	Note       string   `json:"note"`
	SortOrder  *int     `json:"sort_order"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

// Filter 描述一次列表查询。
type Filter struct {
	Query    string
	Tags     []string
	Source   string
	Status   string // "" | "enabled" | "disabled"
	Favorite bool
	Sort     string
	Page     int
	Size     int
}

// TagCount 是标签聚合结果。
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// Stats 是站点概览统计。
type Stats struct {
	Total     int        `json:"total"`
	Enabled   int        `json:"enabled"`
	Disabled  int        `json:"disabled"`
	Favorite  int        `json:"favorite"`
	TagCount  int        `json:"tag_count"`
	Sources   []TagCount `json:"sources"`
	UpdatedAt string     `json:"updated_at"`
}

const promptColumns = `p.id, p.title, p.content, p.negative, p.tags, p.source,
	p.enabled, p.favorite, p.preview_url, p.note, p.sort_order, p.created_at, p.updated_at`

// NormalizeTags 清洗标签：去空白、去重（大小写不敏感）、保持原顺序。
func NormalizeTags(raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		for _, part := range SplitTags(item) {
			key := strings.ToLower(part)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, part)
		}
	}
	return out
}

// SplitTags 把一个标签串拆成多个标签。
func SplitTags(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case '、', ',', '，', ';', '；', '|', '/', '\n', '\r', '\t', ' ':
			return true
		}
		return false
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		f = strings.Trim(f, "#")
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// JoinTags 把标签切片序列化成存储形态。
func JoinTags(tags []string) string {
	return strings.Join(tags, "、")
}

// ParseTags 把存储形态还原成标签切片。
func ParseTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return SplitTags(s)
}

func (f Filter) orderClause() string {
	switch f.Sort {
	case "updated":
		return "p.updated_at DESC, p.id DESC"
	case "created":
		return "p.created_at DESC, p.id DESC"
	case "title":
		return "p.title ASC"
	case "id":
		return "p.id ASC"
	case "oldest":
		return "p.sort_order ASC, p.id ASC"
	default:
		return "p.sort_order DESC, p.id DESC"
	}
}

func (f Filter) limit() int {
	if f.Size <= 0 {
		return 24
	}
	if f.Size > 200 {
		return 200
	}
	return f.Size
}

func (f Filter) offset() int {
	if f.Page <= 1 {
		return 0
	}
	return (f.Page - 1) * f.limit()
}

const likeEscape = '\\'

func escapeLike(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		if r == '%' || r == '_' || r == likeEscape {
			b.WriteRune(likeEscape)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// buildWhere 生成 WHERE 子句与参数。
func (f Filter) buildWhere() (string, []any) {
	var conds []string
	var args []any

	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + escapeLike(q) + "%"
		conds = append(conds, `(p.title LIKE ? ESCAPE '\' OR p.content LIKE ? ESCAPE '\' OR p.negative LIKE ? ESCAPE '\' OR p.tags LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like, like)
	}
	for _, tag := range NormalizeTags(f.Tags) {
		conds = append(conds, "p.id IN (SELECT prompt_id FROM prompt_tags WHERE tag = ?)")
		args = append(args, tag)
	}
	if src := strings.TrimSpace(f.Source); src != "" {
		conds = append(conds, "p.source = ?")
		args = append(args, src)
	}
	switch f.Status {
	case "enabled":
		conds = append(conds, "p.enabled = 1")
	case "disabled":
		conds = append(conds, "p.enabled = 0")
	}
	if f.Favorite {
		conds = append(conds, "p.favorite = 1")
	}
	if len(conds) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}
