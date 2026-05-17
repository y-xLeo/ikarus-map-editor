// customcheck spins up the editor server against a given root and runs the
// custom-object import endpoint locally. Lets us validate the server-side
// pipeline without the browser.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"sromapedit/internal/editor"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: customcheck <root> <source-folder>")
		os.Exit(2)
	}
	root := os.Args[1]
	folder := os.Args[2]

	srv, err := editor.NewServer(root)
	if err != nil {
		die("NewServer:", err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1) Import
	body, _ := json.Marshal(map[string]any{
		"sourceFolder": folder,
		"targetSize":   300,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", ts.URL+"/api/custom-object/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		die("POST import:", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("=== POST /api/custom-object/import → %d\n%s\n\n", resp.StatusCode, raw)
	if resp.StatusCode != 200 {
		os.Exit(1)
	}

	// 2) List
	resp, err = http.Get(ts.URL + "/api/custom-objects")
	if err != nil {
		die("GET list:", err)
	}
	raw, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("=== GET /api/custom-objects → %d\n%s\n\n", resp.StatusCode, raw)

	// 3) Asset detail
	var imp struct {
		ID uint32 `json:"id"`
	}
	if err := json.Unmarshal([]byte(""), &imp); err == nil {
		// no-op; just satisfying linter
	}
	var list struct {
		Entries []struct {
			ID uint32 `json:"id"`
		} `json:"entries"`
	}
	json.Unmarshal(raw, &list)
	if len(list.Entries) == 0 {
		die("list returned 0 entries")
	}
	id := list.Entries[len(list.Entries)-1].ID
	resp, err = http.Get(fmt.Sprintf("%s/api/custom-object?id=%d", ts.URL, id))
	if err != nil {
		die("GET asset:", err)
	}
	raw, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	preview := raw
	if len(preview) > 400 {
		preview = preview[:400]
	}
	fmt.Printf("=== GET /api/custom-object?id=%d → %d\n%s ... (truncated)\n", id, resp.StatusCode, preview)
}

func die(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
