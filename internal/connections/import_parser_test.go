package connections

import (
	"os"
	"strings"
	"testing"
)

func TestImportSample(t *testing.T) {
	source, err := os.ReadFile("../../demo/ssh_config")
	if err != nil {
		t.Fatal(err)
	}
	result := parseImport(string(source))
	if len(result.Diagnostics) != 0 || len(result.Entries) != 3 {
		t.Fatalf("%+v", result)
	}
	for i, e := range result.Entries {
		if e.Host != e.Name || e.Username != "demo" || e.Port != 22 || e.Identity != "demo/keys/demo_ed25519" {
			t.Fatalf("%+v", e)
		}
		if i > 0 && e.Jump != "bastion" {
			t.Fatalf("%+v", e)
		}
	}
}

const simpleImport = "Host lab\n HostName bastion\n User demo\n IdentityFile ~/.ssh/demo\n"

func TestImportSyntax(t *testing.T) {
	for _, suffix := range []string{"ProxyCommand touch /tmp/no", "Include other", "Match all", "ForwardAgent yes", "ProxyJump user@jump", "ProxyJump jump:22", "ProxyJump a,b", "ProxyJump none", "Port 0", "Port +22", "Port 65536", "Port nope", "User other", "Host *", "Host a b", "Host !a", "Host lab", "HostName=x", "IdentityFile \"a\"", "IdentityFile $HOME/key", "IdentityFile %d/key", "Port 22 # comment", "Port 22\\", "Port 2\x00", "Port\u00a022"} {
		t.Run(suffix, func(t *testing.T) {
			if len(parseImport(simpleImport+suffix).Diagnostics) == 0 {
				t.Fatal("accepted unsupported input")
			}
		})
	}
	for _, source := range []string{"", "User demo\n" + simpleImport, "Host only", strings.Repeat("x", MaxImportBytes+1), "\xff", simpleImport + strings.Repeat("a", 4097), strings.Repeat(simpleImport, 101)} {
		if len(parseImport(source).Diagnostics) == 0 {
			t.Fatal("accepted malformed input")
		}
	}
	if got := parseImport(strings.ReplaceAll(simpleImport, "\n", "\r\n")); len(got.Diagnostics) != 0 {
		t.Fatal(got)
	}
}

func TestImportPreviewReadOnlyAndBounds(t *testing.T) {
	f := setup(t)
	expect(t, f.request("POST", "/api/connections/import/preview", ImportRequest{Config: simpleImport}, 0), 200)
	expect(t, f.request("POST", "/api/connections/import/preview", ImportRequest{Config: simpleImport}, -1), 401)
	expect(t, f.request("POST", "/api/connections/import/preview", ImportRequest{Config: strings.Repeat("a", 600000)}, 0), 413)
	var count int
	if err := f.db.QueryRow("SELECT count(*) FROM saved_connections").Scan(&count); err != nil || count != 0 {
		t.Fatalf("preview wrote: %d %v", count, err)
	}
}

func FuzzImportParser(f *testing.F) {
	f.Add(simpleImport)
	f.Add("Host *\nProxyCommand echo unsafe\n")
	f.Fuzz(func(t *testing.T, source string) {
		result := parseImport(source)
		if len(result.Entries) > MaxImportEntries {
			t.Fatal("entry bound exceeded")
		}
		if result.CanConfirm {
			t.Fatal("parser alone authorized persistence")
		}
	})
}
