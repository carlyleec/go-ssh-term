package connections

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/carlyleec/go-ssh-term/db/migrations"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestJumpGuidedTrust(t *testing.T) {
	lab := newJumpLab(t)
	if _, err := lab.f.db.Exec("DELETE FROM host_trust"); err != nil {
		t.Fatal(err)
	}
	before := lab.bastion.auths.Load()
	response := lab.f.request("POST", "/api/connections/"+lab.targetConfig.ID+"/host-key", nil, 0)
	expect(t, response, 200)
	var seen HostInspection
	if err := json.Unmarshal(response.Body.Bytes(), &seen); err != nil {
		t.Fatal(err)
	}
	if seen.Hop != "bastion" || seen.JumpConnectionID != lab.bastionConfig.ID || seen.State != "unknown" {
		t.Fatalf("bastion prompt: %+v", seen)
	}
	if lab.bastion.auths.Load() != before || lab.target.auths.Load() != 0 || lab.requested.Load() != 0 {
		t.Fatal("inspection authenticated before approval")
	}
	response = lab.f.request("POST", "/api/connections/"+lab.targetConfig.ID+"/host-trust", decision(seen), 0)
	expect(t, response, 200)
	var approved HostInspection
	if err := json.Unmarshal(response.Body.Bytes(), &approved); err != nil {
		t.Fatal(err)
	}
	if approved.Hop != "bastion" || approved.State != "trusted" {
		t.Fatalf("approval: %+v", approved)
	}
	target, err := lab.d.Inspect(t.Context(), lab.f.owners[0], lab.targetConfig.ID)
	if err != nil || target.Hop != "target" || target.JumpConnectionID != "" || target.Host != lab.targetConfig.Host || target.State != "unknown" {
		t.Fatalf("target prompt: %+v %v", target, err)
	}
	if lab.target.auths.Load() != 0 {
		t.Fatal("target authenticated during inspection")
	}
	if _, err := lab.d.Approve(t.Context(), lab.f.owners[0], lab.targetConfig.ID, decision(target)); err != nil {
		t.Fatal(err)
	}
	lab.awaitActive(t, 0)
	// A changed bastion blocks traversal and resetting it never approves its replacement.
	replacement, _ := newSigner(t)
	config := ssh.ServerConfig{PublicKeyCallback: lab.bastion.config.Load().PublicKeyCallback}
	config.AddHostKey(replacement)
	lab.bastion.config.Store(&config)
	requests := lab.requested.Load()
	seen, err = lab.d.Inspect(t.Context(), lab.f.owners[0], lab.targetConfig.ID)
	if err != nil || seen.State != "changed" || seen.Hop != "bastion" {
		t.Fatalf("changed bastion: %+v %v", seen, err)
	}
	if lab.requested.Load() != requests {
		t.Fatal("traversed changed bastion")
	}
	reset := decision(seen)
	reset.Fingerprint = seen.TrustedFingerprint
	if err := lab.d.ResetTrust(t.Context(), lab.f.owners[0], lab.targetConfig.ID, reset); err != nil {
		t.Fatal(err)
	}
	seen, err = lab.d.Inspect(t.Context(), lab.f.owners[0], lab.targetConfig.ID)
	if err != nil || seen.State != "unknown" {
		t.Fatalf("reset approved replacement: %+v %v", seen, err)
	}
}

func TestJumpDecisionRejectsChangedReference(t *testing.T) {
	lab := newJumpLab(t)
	if _, err := lab.f.db.Exec("DELETE FROM host_trust"); err != nil {
		t.Fatal(err)
	}
	seen, err := lab.d.Inspect(t.Context(), lab.f.owners[0], lab.targetConfig.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lab.f.db.Exec("UPDATE saved_connections SET jump_connection_id=NULL WHERE id=?", lab.targetConfig.ID); err != nil {
		t.Fatal(err)
	}
	_, err = lab.d.Approve(t.Context(), lab.f.owners[0], lab.targetConfig.ID, decision(seen))
	requireStatus(t, err, 409)
	requireStatus(t, lab.d.ResetTrust(t.Context(), lab.f.owners[0], lab.targetConfig.ID, decision(seen)), 409)
	var count int
	if err := lab.f.db.QueryRow("SELECT count(*) FROM host_trust").Scan(&count); err != nil || count != 0 {
		t.Fatalf("stale approval persisted: %d %v", count, err)
	}
}

func TestJumpFailureAuditAndCleanup(t *testing.T) {
	for _, scenario := range []struct{ hop, reason string }{
		{"bastion", "trust"}, {"target", "trust"}, {"bastion", "authentication"}, {"target", "authentication"}, {"target", "forwarding"},
	} {
		hop := scenario.hop
		t.Run(hop+"/"+scenario.reason, func(t *testing.T) {
			lab := newJumpLab(t)
			lab.approveTarget(t)
			lab.awaitActive(t, 0)
			peer := lab.bastion
			if hop == "target" {
				peer = lab.target
			}
			switch scenario.reason {
			case "trust":
				replacement, _ := newSigner(t)
				config := ssh.ServerConfig{PublicKeyCallback: peer.config.Load().PublicKeyCallback}
				config.AddHostKey(replacement)
				peer.config.Store(&config)
			case "authentication":
				config := *peer.config.Load()
				config.PublicKeyCallback = func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
					return nil, errors.New("private authentication details")
				}
				peer.config.Store(&config)
			case "forwarding":
				lab.reject.Store(true)
			}
			conn := openTerminal(t, lab.f, lab.targetConfig)
			terminalState(t, conn, "connecting")
			status := terminalState(t, conn, "failed")
			if !strings.HasPrefix(status.Message, strings.ToUpper(hop[:1])+hop[1:]+": ") {
				t.Fatalf("hop missing: %+v", status)
			}
			if strings.Contains(status.Message, "private") {
				t.Fatal("raw failure leaked")
			}
			terminalClosed(t, conn)
			events := auditEvents(t, lab.f, 3)
			expected := "ssh_setup_failed"
			if scenario.reason == "trust" {
				expected = "host_trust_rejected"
			}
			if events[1].code != expected {
				t.Fatal(events)
			}
			rows, err := lab.f.db.Query("SELECT jump_snapshot, failure_hop, event_type FROM connection_audit_events ORDER BY occurred_at")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			for rows.Next() {
				var snapshot string
				var failureHop sql.NullString
				var kind string
				if err := rows.Scan(&snapshot, &failureHop, &kind); err != nil {
					t.Fatal(err)
				}
				var fields map[string]any
				if err := json.Unmarshal([]byte(snapshot), &fields); err != nil {
					t.Fatal(err)
				}
				if len(fields) != 5 || fields["id"] != lab.bastionConfig.ID || fields["host"] != lab.bastionConfig.Host {
					t.Fatalf("unsafe or incorrect snapshot: %s", snapshot)
				}
				if (kind == "failure" && failureHop.String != hop) || (kind != "failure" && failureHop.Valid) {
					t.Fatalf("wrong hop: %s %+v", kind, failureHop)
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			lab.awaitActive(t, 0)
		})
	}
}

func TestJumpAuditKeepsBastionSnapshot(t *testing.T) {
	lab := newJumpLab(t)
	lab.approveTarget(t)
	conn := openTerminal(t, lab.f, lab.targetConfig)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	entry := registeredTerminal(t, lab.d, map[string]bool{})
	if _, err := lab.f.db.Exec("UPDATE saved_connections SET name='Edited bastion', host='changed.invalid' WHERE id=?", lab.bastionConfig.ID); err != nil {
		t.Fatal(err)
	}
	if err := lab.d.terminals.close(entry.owner, entry.id); err != nil {
		t.Fatal(err)
	}
	awaitTerminalCleanup(t, entry)
	auditEvents(t, lab.f, 2)
	var snapshots int
	if err := lab.f.db.QueryRow("SELECT count(DISTINCT jump_snapshot) FROM connection_audit_events").Scan(&snapshots); err != nil || snapshots != 1 {
		t.Fatalf("snapshot changed: %d %v", snapshots, err)
	}
	var snapshot string
	if err := lab.f.db.QueryRow("SELECT jump_snapshot FROM connection_audit_events WHERE event_type='end'").Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(snapshot, "Edited") || strings.Contains(snapshot, "changed.invalid") {
		t.Fatal("audit followed edited bastion")
	}
	lab.awaitActive(t, 0)
}

func TestJumpAuditMigration(t *testing.T) {
	f := setup(t)
	saved := result(t, f.request("POST", "/api/connections", f.fields(), 0), 201)
	d := NewDialer(f.db, nil)
	row, err := d.owned(t.Context(), f.owners[0], saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	audit := &terminalAudit{d: d, row: row, attempt: saved.ID, jump: &row}
	if err := audit.insert(t.Context(), d.q, "start", ""); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"UPDATE connection_audit_events SET jump_snapshot='invalid json'",
		"UPDATE connection_audit_events SET failure_hop='bastion'",
		"UPDATE connection_audit_events SET event_type='failure', failure_code='ssh_setup_failed', failure_hop='untrusted-value'",
	} {
		if _, err := f.db.Exec(statement); err == nil {
			t.Fatalf("accepted invalid audit: %s", statement)
		}
	}
	body, err := migrations.Files.ReadFile("20261004000300_jump_audit.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, down, _ := strings.Cut(string(body), "-- migrate:down")
	if _, err := f.db.Exec(down); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.db.QueryRow("SELECT count(*) FROM connection_audit_events").Scan(&count); err != nil || count != 1 {
		t.Fatalf("rollback lost audit: %d %v", count, err)
	}
	if _, err := f.db.Exec(up); err != nil {
		t.Fatal(err)
	}
	var jump, hop sql.NullString
	if err := f.db.QueryRow("SELECT jump_snapshot, failure_hop FROM connection_audit_events").Scan(&jump, &hop); err != nil || jump.Valid || hop.Valid {
		t.Fatalf("old events have fabricated hop details: %+v %+v %v", jump, hop, err)
	}
}
