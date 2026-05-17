// exportcheck spins up the server, runs the custom-object export pipeline
// against an already-imported custom object, and verifies the four output
// files round-trip through our parsers.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"time"

	"sromapedit/internal/editor"
	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: exportcheck <root> <custom-object-id> [collision-offset-x collision-offset-z]")
		os.Exit(2)
	}
	srv, err := editor.NewServer(os.Args[1])
	if err != nil {
		die("server:", err)
	}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	id := os.Args[2]
	q := url.Values{"id": []string{id}}
	if len(os.Args) >= 5 {
		q.Set("collisionOffsetX", os.Args[3])
		q.Set("collisionOffsetZ", os.Args[4])
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", ts.URL+"/api/custom-object/export?"+q.Encode(), bytes.NewReader(nil))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		die("POST export:", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("=== POST /api/custom-object/export?id=%s → %d\n%s\n\n", id, resp.StatusCode, raw)
	if resp.StatusCode != 200 {
		os.Exit(1)
	}

	var result editor.ExportResult
	if err := json.Unmarshal(raw, &result); err != nil {
		die("parse response:", err)
	}

	// Validate each generated file round-trips.
	root := os.Args[1]
	fmt.Println("=== Round-trip validation:")

	bmsPath := root + "\\Data\\" + result.Paths.BMS
	if bms, err := sromap.LoadBMS(bmsPath); err != nil {
		fmt.Printf("  BMS: FAIL — %v\n", err)
	} else {
		fmt.Printf("  BMS: %d vertices, %d indices, bbox %v..%v\n",
			len(bms.Vertices), len(bms.Indices), bms.BBoxMin, bms.BBoxMax)
	}
	bmtPath := root + "\\Data\\" + result.Paths.BMT
	if bmt, err := sromap.LoadBMT(bmtPath); err != nil {
		fmt.Printf("  BMT: FAIL — %v\n", err)
	} else {
		fmt.Printf("  BMT: %d materials, first=%+v\n", len(bmt.Materials), bmt.Materials[0])
	}
	bsrPath := root + "\\Data\\" + result.Paths.BSR
	if bsr, err := sromap.LoadBSR(bsrPath); err != nil {
		fmt.Printf("  BSR: FAIL — %v\n", err)
	} else {
		fmt.Printf("  BSR: name=%s, %d mat refs, %d mesh refs, collision=%q\n",
			bsr.Name, len(bsr.Materials), len(bsr.Meshes), bsr.CollisionMesh)
	}
	ddjPath := root + "\\Data\\" + result.Paths.DDJ
	if img, err := sromap.LoadDDJ(ddjPath); err != nil {
		fmt.Printf("  DDJ: FAIL — %v\n", err)
	} else {
		fmt.Printf("  DDJ: %dx%d %s\n", img.Width, img.Height, img.Format)
	}

	fmt.Printf("\n=> Allocated game objID: %d\n", result.GameObjID)
}

func die(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
