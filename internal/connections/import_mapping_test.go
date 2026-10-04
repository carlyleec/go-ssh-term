package connections

import (
	"encoding/json"
	"testing"
	"time"
)

func previewResult(t *testing.T, f fixture, request ImportRequest) ImportPreview {
	t.Helper()
	w := f.request("POST", "/api/connections/import/preview", request, 0)
	expect(t, w, 200)
	var result ImportPreview
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestImportMappings(t *testing.T) {
	f := setup(t)
	request := ImportRequest{Config: simpleImport, Selections: []ImportSelection{{Name: "lab", SSHKeyID: f.keys[0]}}}
	if !previewResult(t, f, request).CanConfirm {
		t.Fatal("owned key should resolve")
	}
	request.Selections[0].SSHKeyID = f.keys[1]
	if previewResult(t, f, request).CanConfirm {
		t.Fatal("accepted foreign key")
	}
	request.Selections[0].SSHKeyID = f.keys[0]
	foreign := f.fields()
	foreign.Name = "lab"
	foreign.SSHKeyID = f.keys[1]
	result(t, f.request("POST", "/api/connections", foreign, 1), 201)
	if !previewResult(t, f, request).CanConfirm {
		t.Fatal("foreign name blocked import")
	}
	own := f.fields()
	own.Name = "lab"
	result(t, f.request("POST", "/api/connections", own, 0), 201)
	if previewResult(t, f, request).CanConfirm {
		t.Fatal("accepted name collision")
	}
}

func TestImportJumpMapping(t *testing.T) {
	f := setup(t)
	fields := f.fields()
	fields.Name = "jump"
	jump := result(t, f.request("POST", "/api/connections", fields, 0), 201)
	request := ImportRequest{Config: simpleImport + " ProxyJump jump\n", Selections: []ImportSelection{{Name: "lab", SSHKeyID: f.keys[0]}}}
	if previewResult(t, f, request).CanConfirm {
		t.Fatal("implicitly resolved jump")
	}
	request.Selections[0].JumpConnectionID = jump.ID
	request.Selections[0].JumpUpdatedAt = jump.UpdatedAt.Format(time.RFC3339Nano)
	if !previewResult(t, f, request).CanConfirm {
		t.Fatal("explicit jump did not resolve")
	}
	request.Selections[0].JumpUpdatedAt = "stale"
	if previewResult(t, f, request).CanConfirm {
		t.Fatal("accepted stale jump")
	}
	request.Config += "Host jump\nHostName bastion\nUser demo\nIdentityFile key\n"
	request.Selections = []ImportSelection{{Name: "lab", SSHKeyID: f.keys[0]}, {Name: "jump", SSHKeyID: f.keys[0]}}
	expect(t, f.request("DELETE", "/api/connections/"+jump.ID, nil, 0), 204)
	if !previewResult(t, f, request).CanConfirm {
		t.Fatal("selected dependency did not resolve")
	}
	request.Selections = request.Selections[:1]
	if previewResult(t, f, request).CanConfirm {
		t.Fatal("accepted deselected dependency")
	}
}
