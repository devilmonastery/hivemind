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
		{
			name:     "hashtags with hyphens",
			input:    "Test #my-tag #multi-word-tag #single",
			wantTags: []string{"multi-word-tag", "my-tag", "single"}, // sorted
		},
		{
			name:     "hashtags with underscores and hyphens",
			input:    "Mix #snake_case #kebab-case #camelCase",
			wantTags: []string{"camelcase", "kebab-case", "snake_case"}, // sorted, lowercase
		},
		{
			name:     "hashtags ending with punctuation",
			input:    "Sentence ends with #tag. Another #tag2, and #tag3!",
			wantTags: []string{"tag", "tag2", "tag3"}, // punctuation not included
		},
		{
			name:     "hashtag at start and end",
			input:    "#start middle text #end",
			wantTags: []string{"end", "start"},
		},
		{
			name:     "multiple hyphens in tag",
			input:    "Test #my-multi-word-tag-here",
			wantTags: []string{"my-multi-word-tag-here"},
		},
		{
			name:     "hashtag with leading hyphen (edge case)",
			input:    "Test #-tag",
			wantTags: []string{"-tag"}, // regex allows leading hyphen
		},
		{
			name:     "hashtag with trailing hyphen (edge case)",
			input:    "Test #tag-",
			wantTags: []string{"tag-"}, // regex allows trailing hyphen
		},
		{
			name:     "complex hashtags",
			input:    "Notes #project-alpha #v2_0 #Q4-2024 #follow-up",
			wantTags: []string{"follow-up", "project-alpha", "q4-2024", "v2_0"}, // sorted
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
