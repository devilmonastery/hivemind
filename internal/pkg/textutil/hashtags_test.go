package textutil

import (
	"reflect"
	"testing"
)

func TestExtractHashtags(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "single hashtag",
			text: "This is a #test",
			want: []string{"test"},
		},
		{
			name: "multiple hashtags",
			text: "This is a #test with #multiple #tags",
			want: []string{"multiple", "tags", "test"},
		},
		{
			name: "hashtags with hyphens",
			text: "Check out #my-tag and #another-one",
			want: []string{"another-one", "my-tag"},
		},
		{
			name: "hashtags with underscores",
			text: "Using #snake_case and #mixed_Tags",
			want: []string{"mixed_tags", "snake_case"},
		},
		{
			name: "hashtags with numbers",
			text: "Version #v1 and #test123",
			want: []string{"test123", "v1"},
		},
		{
			name: "duplicate hashtags",
			text: "#test #test #TEST #Test",
			want: []string{"test"},
		},
		{
			name: "no hashtags",
			text: "Just plain text without any tags",
			want: []string{},
		},
		{
			name: "empty string",
			text: "",
			want: []string{},
		},
		{
			name: "hashtag at start",
			text: "#start of text",
			want: []string{"start"},
		},
		{
			name: "hashtag at end",
			text: "text at #end",
			want: []string{"end"},
		},
		{
			name: "complex markdown with hashtags",
			text: "# Heading\n\nSome content with #tag1 and #tag2.\n\n- List item with #tag3",
			want: []string{"tag1", "tag2", "tag3"},
		},
		{
			name: "discord channel mentions should be ignored",
			text: "Check out <#274539255248977920> and <#123456789> for more info",
			want: []string{},
		},
		{
			name: "mix of channel mentions and real hashtags",
			text: "See <#274539255248977920> for #updates and #announcements in <#123456789>",
			want: []string{"announcements", "updates"},
		},
		{
			name: "hashtag followed by greater-than should still work",
			text: "This #tag is > that one",
			want: []string{"tag"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractHashtags(tt.text)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractHashtags() = %v, want %v", got, tt.want)
			}
		})
	}
}
