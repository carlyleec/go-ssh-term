package connections

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func newJumpOwnershipLab(t *testing.T) *jumpLab {
	t.Helper()
	lab := newJumpLab(t)
	// The dial fixture replaces the CRUD fixture's keys with one encrypted key.
	// Restore a foreign reference for valid account reassignment without ever
	// making that account's placeholder key usable for SSH authentication.
	if _, err := lab.f.db.Exec("INSERT INTO ssh_keys VALUES (?, ?, 'Foreign key', 'SHA256:foreign', X'01ff', 1)", lab.f.keys[1], lab.f.owners[1]); err != nil {
		t.Fatal(err)
	}
	return lab
}

// Change valid saved state after the attempt records its route, without disabling
// the database's graph or ownership constraints.
func jumpMutation(lab *jumpLab, mode, upstream string) string {
	detach := fmt.Sprintf("UPDATE saved_connections SET jump_connection_id=NULL WHERE id='%s';", lab.targetConfig.ID)
	switch mode {
	case "deleted":
		return detach + fmt.Sprintf("DELETE FROM saved_connections WHERE id='%s';", lab.bastionConfig.ID)
	case "foreign":
		return detach + fmt.Sprintf("UPDATE saved_connections SET account_id='%s', ssh_key_id='%s' WHERE id='%s';", lab.f.owners[1], lab.f.keys[1], lab.bastionConfig.ID)
	case "chained":
		return detach + fmt.Sprintf("UPDATE saved_connections SET jump_connection_id='%s' WHERE id='%s';", upstream, lab.bastionConfig.ID)
	default:
		panic("unknown mutation")
	}
}

func TestJumpRevalidatesAfterAuditSnapshot(t *testing.T) {
	for _, mode := range []string{"deleted", "foreign", "chained"} {
		t.Run(mode, func(t *testing.T) {
			lab := newJumpOwnershipLab(t)
			lab.approveTarget(t)
			lab.awaitActive(t, 0)
			upstream := result(t, lab.f.request("POST", "/api/connections", lab.f.fields(), 0), 201)
			beforeB, beforeT, beforeRequests := lab.bastion.auths.Load(), lab.target.auths.Load(), lab.requested.Load()
			// The trigger is a deterministic barrier between snapshot persistence and
			// transport setup; no test-only hooks are needed in the dialer.
			trigger := fmt.Sprintf(`CREATE TRIGGER change_jump_after_start AFTER INSERT ON connection_audit_events
    WHEN NEW.event_type='start' AND NEW.saved_connection_id='%s'
    BEGIN %s END`, lab.targetConfig.ID, jumpMutation(lab, mode, upstream.ID))
			if _, err := lab.f.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			conn := openTerminal(t, lab.f, lab.targetConfig)
			terminalState(t, conn, "connecting")
			status := terminalState(t, conn, "failed")
			want := "Bastion: connection not found"
			if mode == "chained" {
				want = "Bastion: select an owned direct jump connection"
			}
			if status.Message != want {
				t.Fatalf("unsafe or incorrect rejection: %q", status.Message)
			}
			terminalClosed(t, conn)
			events := auditEvents(t, lab.f, 3)
			if events[0].kind != "start" || events[1].kind != "failure" || events[2].kind != "end" {
				t.Fatal(events)
			}
			if lab.bastion.auths.Load() != beforeB || lab.target.auths.Load() != beforeT || lab.requested.Load() != beforeRequests {
				t.Fatal("rejected jump still authenticated or forwarded")
			}
			var hop, snapshot string
			if err := lab.f.db.QueryRow("SELECT failure_hop, jump_snapshot FROM connection_audit_events WHERE event_type='failure'").Scan(&hop, &snapshot); err != nil {
				t.Fatal(err)
			}
			var saved map[string]any
			if err := json.Unmarshal([]byte(snapshot), &saved); err != nil {
				t.Fatal(err)
			}
			if hop != "bastion" || saved["id"] != lab.bastionConfig.ID || saved["host"] != lab.bastionConfig.Host {
				t.Fatalf("lost attempted route: %s %s", hop, snapshot)
			}
			lab.awaitActive(t, 0)
			lab.d.terminals.mu.Lock()
			var finishing []*liveTerminal
			for _, entry := range lab.d.terminals.entries {
				finishing = append(finishing, entry)
			}
			lab.d.terminals.mu.Unlock()
			for _, entry := range finishing {
				awaitTerminalCleanup(t, entry)
			}
		})
	}
}

func TestJumpCancellationWhileOwnershipChanges(t *testing.T) {
	for _, mode := range []string{"close", "logout"} {
		t.Run(mode, func(t *testing.T) {
			lab := newJumpOwnershipLab(t)
			lab.approveTarget(t)
			seen := map[string]bool{}
			first := openTerminal(t, lab.f, lab.targetConfig)
			terminalState(t, first, "connecting")
			terminalState(t, first, "connected")
			live := registeredTerminal(t, lab.d, seen)
			lab.awaitActive(t, 1)
			for len(lab.opened) > 0 {
				<-lab.opened
			}
			beforeTarget := lab.target.auths.Load()
			lab.hold.Store(true)
			second := openTerminal(t, lab.f, lab.targetConfig)
			terminalState(t, second, "connecting")
			select {
			case <-lab.opened:
			case <-time.After(2 * time.Second):
				t.Fatal("pending attempt never reached forwarding")
			}
			// A pending entry has no published shell yet, so locate it under the registry lock.
			lab.d.terminals.mu.Lock()
			var pending *liveTerminal
			for _, entry := range lab.d.terminals.entries {
				if entry.id != live.id {
					pending = entry
				}
			}
			lab.d.terminals.mu.Unlock()
			if pending == nil {
				t.Fatal("missing pending attempt")
			}
			if _, err := lab.f.db.Exec(jumpMutation(lab, "foreign", "")); err != nil {
				t.Fatal(err)
			}
			if mode == "logout" {
				expect(t, lab.f.request("POST", "/api/auth/logout", nil, 0), 204)
				awaitTerminalCleanup(t, live)
			} else {
				if err := lab.d.terminals.close(pending.owner, pending.id); err != nil {
					t.Fatal(err)
				}
			}
			// Cleanup must finish while the server still refuses to answer channel-open.
			awaitTerminalCleanup(t, pending)
			if err := second.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, _, err := second.ReadMessage(); err == nil {
				t.Fatal("canceled setup published late readiness")
			}
			select {
			case lab.release <- struct{}{}:
			case <-time.After(time.Second):
				t.Fatal("controlled forwarding peer did not release")
			}
			if mode == "close" {
				lab.awaitActive(t, 1)
				if err := lab.d.terminals.input(live.owner, live.id, []byte("survives ownership change")); err != nil {
					t.Fatal(err)
				}
				expectTerminalBytes(t, first, "survives ownership change")
				if err := lab.d.terminals.close(live.owner, live.id); err != nil {
					t.Fatal(err)
				}
				awaitTerminalCleanup(t, live)
			}
			lab.awaitActive(t, 0)
			if lab.target.auths.Load() != beforeTarget {
				t.Fatal("canceled attempt authenticated target")
			}
			assertAttemptHistory(t, lab.f, map[string][]string{live.id: {"start", "end"}, pending.id: {"start", "end"}})
			var changed sql.NullString
			if err := lab.f.db.QueryRow("SELECT jump_connection_id FROM saved_connections WHERE id=?", lab.targetConfig.ID).Scan(&changed); err != nil || changed.Valid {
				t.Fatalf("route mutation failed: %+v %v", changed, err)
			}
		})
	}
}
