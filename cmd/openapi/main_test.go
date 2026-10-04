package main

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestExportIsDeterministicAndCurrentWithoutServices(t *testing.T) {
	// Invalid runtime settings must not prevent contract export.
	t.Setenv("DATABASE_PATH", "/does-not-exist/database.sqlite")
	t.Setenv("ENCRYPTION_KEY_PATH", "/does-not-exist/key")
	t.Setenv("BROWSER_ORIGIN", "invalid")
	var first, second bytes.Buffer
	if err := export(&first); err != nil {
		t.Fatal(err)
	}
	if err := export(&second); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("contract export is nondeterministic")
	}
	saved, err := os.ReadFile("../../openapi/api.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), saved) {
		t.Fatal("OpenAPI artifact is stale; run make openapi")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestExportReportsWriteFailure(t *testing.T) {
	if err := export(failingWriter{}); err == nil {
		t.Fatal("export ignored an output failure")
	}
}
