package urlutil

import (
	"net/url"
)

// BuildNoteViewURL builds a web application URL for viewing a note.
// Returns a URL like: {baseURL}/note?id={noteID}
func BuildNoteViewURL(baseURL, noteID string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = "/note"
	q := u.Query()
	q.Set("id", noteID)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// BuildWikiViewURL builds a web application URL for viewing a wiki page.
// Returns a URL like: {baseURL}/wiki?guild_id={guildID}&title={title}
// The title parameter is automatically URL-encoded.
func BuildWikiViewURL(baseURL, guildID, title string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = "/wiki"
	q := u.Query()
	q.Set("guild_id", guildID)
	q.Set("title", title)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
