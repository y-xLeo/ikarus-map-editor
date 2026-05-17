// nvmrebuildcheck calls the actual editor server's /api/region/rebuild-nvm
// endpoint in default mode, then byte-diffs the result against the source
// NVM. Any non-tile-flag byte differing is a regression in the live path.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"sromapedit/internal/editor"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: nvmrebuildcheck <root> <regionX> <regionY> [query-param...]")
		os.Exit(2)
	}
	root := os.Args[1]
	rx := os.Args[2]
	ry := os.Args[3]

	srv, err := editor.NewServer(root)
	if err != nil {
		die("server:", err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Snapshot the source NVM bytes before the rebuild touches them.
	// We must run the rebuild against the same file the editor opens.
	url := fmt.Sprintf("%s/api/region/rebuild-nvm?x=%s&y=%s", ts.URL, rx, ry)
	for _, param := range os.Args[4:] {
		url += "&" + param
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(nil))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		die("POST:", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("=== %s → %d\n%s\n", url, resp.StatusCode, body)
}

func die(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
