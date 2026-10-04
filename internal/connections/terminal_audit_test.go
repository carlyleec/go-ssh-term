package connections

import (
	"database/sql"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

type auditEvent struct {
	kind, code, attempt, account, saved, name, host, username string
	port, at                                                  int64
}

func auditEvents(t *testing.T, f fixture, count int) []auditEvent {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		rows, err := f.db.Query(`SELECT event_type, failure_code, attempt_id, account_id, saved_connection_id, connection_name, host, username, port, occurred_at FROM connection_audit_events ORDER BY occurred_at, rowid`)
		if err != nil {
			t.Fatal(err)
		}
		var events []auditEvent
		for rows.Next() {
			var e auditEvent
			var code sql.NullString
			if err := rows.Scan(&e.kind, &code, &e.attempt, &e.account, &e.saved, &e.name, &e.host, &e.username, &e.port, &e.at); err != nil {
				t.Fatal(err)
			}
			e.code = code.String
			events = append(events, e)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(events) >= count {
			return events
		}
		if time.Now().After(deadline) {
			t.Fatalf("audit events: %#v", events)
		}
		time.Sleep(time.Millisecond)
	}
}
func TestTerminalAuditExit(t *testing.T) {
	for _, status := range []uint32{0, 7} {
		t.Run(strconv.Itoa(int(status)), func(t *testing.T) {
			f, d, user := dialFixture(t)
			host, _ := newSigner(t)
			finish := make(chan struct{})
			p := startPeerChannels(t, host, user, func(newChannel ssh.NewChannel) {
				ch, requests, err := newChannel.Accept()
				if err != nil {
					return
				}
				defer ch.Close()
				for r := range requests {
					r.Reply(true, nil)
					if r.Type == "shell" {
						<-finish
						ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
						return
					}
				}
			})
			saved := approvedTerminal(t, f, d, p)
			conn := openTerminal(t, f, saved)
			terminalState(t, conn, "connecting")
			terminalState(t, conn, "connected")
			entry := registeredTerminal(t, d, map[string]bool{})
			close(finish)
			terminalState(t, conn, "disconnected")
			terminalClosed(t, conn)
			<-entry.done
			count := 2
			if status != 0 {
				count = 3
			}
			events := auditEvents(t, f, count)
			kinds := []string{}
			for _, e := range events {
				kinds = append(kinds, e.kind)
				if e.attempt != entry.id || e.account != f.owners[0] || e.saved != saved.ID || e.host != saved.Host || e.at <= 0 {
					t.Fatalf("wrong snapshot: %+v", e)
				}
			}
			want := []string{"start", "end"}
			if status != 0 {
				want = []string{"start", "failure", "end"}
				if events[1].code != "ssh_session_failed" {
					t.Fatal(events)
				}
			}
			if !reflect.DeepEqual(kinds, want) {
				t.Fatal(kinds)
			}
		})
	}
}
func TestTerminalAuditSnapshotAndLogout(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startRegistryPeer(t, host, user)
	saved := approvedTerminal(t, f, d, p)
	conn := openTerminal(t, f, saved)
	terminalState(t, conn, "connecting")
	terminalState(t, conn, "connected")
	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("secret terminal input")); err != nil {
		t.Fatal(err)
	}
	expectTerminalBytes(t, conn, "secret terminal input")
	if _, err := f.db.Exec(`UPDATE saved_connections SET name='edited',host='elsewhere',username='changed' WHERE id=?`, saved.ID); err != nil {
		t.Fatal(err)
	}
	expect(t, f.request("DELETE", "/api/connections/"+saved.ID, nil, 0), 204)
	expect(t, f.request("POST", "/api/auth/logout", nil, 0), 204)
	events := auditEvents(t, f, 2)
	if len(events) != 2 || events[0].kind != "start" || events[1].kind != "end" {
		t.Fatal(events)
	}
	for _, e := range events {
		if e.name != saved.Name || e.host != saved.Host || e.username != saved.Username || e.port != saved.Port || e.code != "" {
			t.Fatalf("snapshot changed: %+v", e)
		}
	}
}
func TestTerminalAuditTrustFailureAndWriteFailure(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "trust", true: "audit storage"}[unavailable], func(t *testing.T) {
			f, _, user := dialFixture(t)
			host, _ := newSigner(t)
			p := startRegistryPeer(t, host, user)
			saved := destination(t, f, p)
			if unavailable {
				if _, err := f.db.Exec(`CREATE TRIGGER reject_audit BEFORE INSERT ON connection_audit_events BEGIN SELECT RAISE(ABORT,'secret storage error'); END`); err != nil {
					t.Fatal(err)
				}
			}
			conn := openTerminal(t, f, saved)
			if !unavailable {
				terminalState(t, conn, "connecting")
			}
			status := terminalState(t, conn, "failed")
			terminalClosed(t, conn)
			if unavailable {
				if status.Message != "could not record connection attempt; try again" {
					t.Fatal(status)
				}
				if got := auditEvents(t, f, 0); len(got) != 0 {
					t.Fatal(got)
				}
			} else {
				events := auditEvents(t, f, 3)
				if len(events) != 3 || events[1].code != "host_trust_rejected" || events[2].kind != "end" {
					t.Fatal(events)
				}
			}
			if p.auths.Load() != 0 {
				t.Fatal("failure reached authentication")
			}
		})
	}
}

func TestTerminalAuditFinalEventsAreAtomic(t *testing.T) {
	f, d, user := dialFixture(t)
	host, _ := newSigner(t)
	p := startRegistryPeer(t, host, user)
	saved := approvedTerminal(t, f, d, p)
	row, err := d.owned(t.Context(), f.owners[0], saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	a := terminalAudit{d: d, row: row, attempt: saved.ID}
	if err := a.insert(t.Context(), d.q, "start", ""); err != nil {
		t.Fatal(err)
	}
	a.fail("ssh_setup_failed")
	if _, err := f.db.Exec(`CREATE TRIGGER reject_end BEFORE INSERT ON connection_audit_events WHEN NEW.event_type='end' BEGIN SELECT RAISE(ABORT,'secret'); END`); err != nil {
		t.Fatal(err)
	}
	if err := a.finishContext(t.Context()); err == nil {
		t.Fatal("expected storage failure")
	}
	events := auditEvents(t, f, 1)
	if len(events) != 1 || events[0].kind != "start" {
		t.Fatal("partially committed final audit events", events)
	}
}
