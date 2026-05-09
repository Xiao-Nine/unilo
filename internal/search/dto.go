package search

import (
	"time"

	"github.com/google/uuid"
)

type SearchOptions struct {
	Query      string
	SourceType string
	Limit      int
	ChannelID  *uuid.UUID
}

type SearchResponse struct {
	Items []SearchResult `json:"items"`
}

type SearchResult struct {
	Type      string    `json:"type"`
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Snippet   string    `json:"snippet"`
	CreatedAt time.Time `json:"created_at"`
}

type IndexDocument struct {
	SourceType string
	SourceID   string
	Title      string
	Content    string
	Metadata   map[string]any
}
