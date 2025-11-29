package handlers

import (
	"testing"
)

func TestExtractHashtags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTags []string
	}{
		{
			name:     "single hashtag",
			input:    "This is a note #important",
			wantTags: []string{"important"},
		},
		{
			name:     "multiple hashtags",
			input:    "Meeting notes #work #important #project",
			wantTags: []string{"important", "project", "work"}, // sorted
		},
		{
			name:     "duplicate hashtags",
			input:    "Test #tag1 #tag2 #tag1",
			wantTags: []string{"tag1", "tag2"},
		},
		{
			name:     "hashtags in middle",
			input:    "Start #middle text continues here",
			wantTags: []string{"middle"},
		},
		{
			name:     "no hashtags",
			input:    "Just regular text",
			wantTags: []string{},
		},
		{
			name:     "empty string",
			input:    "",
			wantTags: []string{},
		},
		{
			name:     "only hashtags",
			input:    "#tag1 #tag2 #tag3",
			wantTags: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "hashtags with numbers",
			input:    "Note #year2024 #v1",
			wantTags: []string{"v1", "year2024"}, // sorted
		},
		{
			name:     "case insensitive tags",
			input:    "Test #Important #URGENT #important",
			wantTags: []string{"important", "urgent"}, // deduplicated and lowercase
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTags := extractHashtags(tt.input)

			if len(gotTags) != len(tt.wantTags) {
				t.Errorf("extractHashtags() got %d tags, want %d tags. Got: %v, Want: %v",
					len(gotTags), len(tt.wantTags), gotTags, tt.wantTags)
				return
			}

			// Check that all expected tags are present in order (since we sort)
			for i, wantTag := range tt.wantTags {
				if gotTags[i] != wantTag {
					t.Errorf("extractHashtags() tag[%d] = %v, want %v", i, gotTags[i], wantTag)
				}
			}
		})
	}
}
