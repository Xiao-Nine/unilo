package search

import (
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"unilo/pkg/apperror"
)

const snippetRadius = 60

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Search(query string, sourceType string, limit int) (SearchResponse, error) {
	return s.SearchWithOptions(SearchOptions{Query: query, SourceType: sourceType, Limit: limit})
}

func (s *Service) SearchWithOptions(opts SearchOptions) (SearchResponse, error) {
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return SearchResponse{}, apperror.BadRequest("q is required")
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	sourceType := opts.SourceType
	if opts.ChannelID != nil && (strings.TrimSpace(sourceType) == "" || strings.TrimSpace(sourceType) == "all") {
		sourceType = "messages"
	}
	sourceTypes, err := parseSourceType(sourceType)
	if err != nil {
		return SearchResponse{}, err
	}
	if opts.ChannelID != nil {
		if len(sourceTypes) == 0 {
			sourceTypes = []string{"message"}
		}
		for _, sourceType := range sourceTypes {
			if sourceType != "message" {
				return SearchResponse{}, apperror.BadRequest("channel_id only supports message search")
			}
		}
	}

	docs, err := s.repo.SearchWithOptions(query, sourceTypes, opts.ChannelID, limit)
	if err != nil {
		return SearchResponse{}, apperror.Internal(err)
	}
	items := make([]SearchResult, 0, len(docs))
	for _, doc := range docs {
		items = append(items, SearchResult{Type: doc.SourceType, ID: doc.SourceID, Title: doc.Title, Snippet: makeSnippet(doc.Content, query), CreatedAt: doc.CreatedAt})
	}
	return SearchResponse{Items: items}, nil
}

func (s *Service) UpsertDocument(tx *gorm.DB, sourceType string, sourceID string, title string, content string, metadata map[string]any) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	return s.repo.UpsertDocument(tx, IndexDocument{SourceType: sourceType, SourceID: sourceID, Title: title, Content: content, Metadata: metadata})
}

func (s *Service) DeleteDocument(tx *gorm.DB, sourceType string, sourceID string) error {
	return s.repo.DeleteDocument(tx, sourceType, sourceID)
}

func parseSourceType(raw string) ([]string, error) {
	switch strings.TrimSpace(raw) {
	case "", "all":
		return nil, nil
	case "messages":
		return []string{"message"}, nil
	case "drops":
		return []string{"drop"}, nil
	case "files":
		return []string{"file"}, nil
	default:
		return nil, apperror.BadRequest("type is invalid")
	}
}

func makeSnippet(content string, query string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	contentRunes := []rune(content)
	lowerContentRunes := []rune(strings.ToLower(content))
	lowerQueryRunes := []rune(strings.ToLower(strings.TrimSpace(query)))
	idx := runeIndex(lowerContentRunes, lowerQueryRunes)
	if idx < 0 {
		return abbreviate(contentRunes, 0, min(len(contentRunes), snippetRadius*2))
	}
	start := max(idx-snippetRadius, 0)
	end := min(idx+len(lowerQueryRunes)+snippetRadius, len(contentRunes))
	return abbreviate(contentRunes, start, end)
}

func runeIndex(haystack []rune, needle []rune) int {
	if len(needle) == 0 || len(needle) > len(haystack) {
		return -1
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		matched := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

func abbreviate(runes []rune, start int, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start > end {
		start = end
	}
	prefix := ""
	if start > 0 {
		prefix = "..."
	}
	suffix := ""
	if end < len(runes) {
		suffix = "..."
	}
	out := string(runes[start:end])
	if !utf8.ValidString(out) {
		out = strings.ToValidUTF8(out, "")
	}
	return prefix + out + suffix
}
