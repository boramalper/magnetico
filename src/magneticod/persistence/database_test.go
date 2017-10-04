package persistence

import (
	"path"
	"testing"
)


// TestPathJoin tests the assumption we made in flushNewTorrents() function where we assumed path
// separator to be the `/` (slash), and not `\` (backslash) character (which is used by Windows).
//
// Golang seems to use slash character on both platforms but we need to check that slash character
// is used in all cases. As a rule of thumb in secure programming, always check ONLY for the valid
// case AND IGNORE THE REST (e.g. do not check for backslashes but check for slashes).
func TestPathJoin(t *testing.T) {
	if path.Join("a", "b", "c") != "a/b/c" {
		t.Errorf("path.Join uses a different character than `/` (slash) character as path separator!  (path: `%s`)",
			path.Join("a", "b", "c"))
	}
}
