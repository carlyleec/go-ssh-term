package connections

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
)

func sampleRequest(t *testing.T, key string) ImportRequest {
	t.Helper()
	source, err := os.ReadFile("../../demo/ssh_config")
	if err != nil {
		t.Fatal(err)
	}
	request := ImportRequest{Config: string(source)}
	for _, name := range []string{"target-2", "target-1", "bastion"} {
		request.Selections = append(request.Selections, ImportSelection{Name: name, SSHKeyID: key})
	}
	return request
}
func connectionCount(t *testing.T, f fixture) int {
	t.Helper()
	var n int
	if err := f.db.QueryRow("SELECT count(*) FROM saved_connections").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
func TestImportConfirmAtomic(t *testing.T) {
	f := setup(t)
	request := sampleRequest(t, f.keys[0])
	w := f.request("POST", "/api/connections/import/confirm", request, 0)
	expect(t, w, 201)
	var body ConnectionsBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Connections) != 3 || body.Connections[0].Name != "bastion" {
		t.Fatal(body)
	}
	for _, row := range body.Connections[1:] {
		if row.JumpConnectionID == nil || *row.JumpConnectionID != body.Connections[0].ID {
			t.Fatal("jump not wired")
		}
	}
	expect(t, f.request("POST", "/api/connections/import/confirm", request, 0), 409)
	if connectionCount(t, f) != 3 {
		t.Fatal("retry wrote duplicates")
	}
}
func TestImportConfirmRollback(t *testing.T) {
	f := setup(t)
	// Fail the last insert after the bastion and first target have been inserted.
	if _, err := f.db.Exec(`CREATE TRIGGER fail_import BEFORE INSERT ON saved_connections WHEN NEW.name = 'target-2' BEGIN SELECT RAISE(ABORT, 'test'); END`); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("POST", "/api/connections/import/confirm", sampleRequest(t, f.keys[0]), 0), 503)
	if connectionCount(t, f) != 0 {
		t.Fatal("partial import persisted")
	}
}
func TestImportConfirmRevalidates(t *testing.T) {
	for _, change := range []string{"key deleted", "foreign key", "name added", "malformed", "deselected jump", "duplicate selection", "self", "chain", "foreign jump"} {
		t.Run(change, func(t *testing.T) {
			f := setup(t)
			request := sampleRequest(t, f.keys[0])
			baseline := 0
			if !previewResult(t, f, request).CanConfirm {
				t.Fatal("invalid fixture")
			}
			switch change {
			case "key deleted":
				if _, err := f.db.Exec("DELETE FROM ssh_keys WHERE id = ?", f.keys[0]); err != nil {
					t.Fatal(err)
				}
			case "foreign key":
				request.Selections[0].SSHKeyID = f.keys[1]
			case "name added":
				fields := f.fields()
				fields.Name = "target-2"
				result(t, f.request("POST", "/api/connections", fields, 0), 201)
				baseline = 1
			case "malformed":
				request.Config += "ProxyCommand arbitrary\n"
			case "deselected jump":
				request.Selections = request.Selections[:2]
			case "duplicate selection":
				request.Selections = append(request.Selections, request.Selections[0])
			case "self":
				request.Config = strings.Replace(request.Config, "ProxyJump bastion", "ProxyJump target-1", 1)
			case "chain":
				request.Config = strings.Replace(request.Config, "Host target-1", "ProxyJump target-2\nHost target-1", 1)
			case "foreign jump":
				fields := f.fields()
				fields.Name = "bastion"
				fields.SSHKeyID = f.keys[1]
				jump := result(t, f.request("POST", "/api/connections", fields, 1), 201)
				baseline = 1
				request.Selections = request.Selections[:2]
				request.Selections[0].JumpConnectionID = jump.ID
			}
			expect(t, f.request("POST", "/api/connections/import/confirm", request, 0), 409)
			if connectionCount(t, f) != baseline {
				t.Fatal("invalid selection wrote records")
			}
		})
	}
}
func TestImportConcurrentConfirm(t *testing.T) {
	f := setup(t)
	request := sampleRequest(t, f.keys[0])
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for range 2 {
		wg.Go(func() { codes <- f.request("POST", "/api/connections/import/confirm", request, 0).Code })
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[201] != 1 || counts[409] != 1 || connectionCount(t, f) != 3 {
		t.Fatal(counts)
	}
}
