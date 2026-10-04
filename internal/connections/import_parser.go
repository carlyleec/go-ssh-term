package connections

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxImportBytes = 64 * 1024
const MaxImportEntries = 100

type ImportDiagnostic struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}
type ImportEntry struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Username string `json:"username"`
	Port     int64  `json:"port"`
	Identity string `json:"identity"`
	Jump     string `json:"jump"`
	Line     int    `json:"line"`
}
type ImportPreview struct {
	Issues      []ImportIssue      `json:"issues" nullable:"false"`
	CanConfirm  bool               `json:"can_confirm"`
	Entries     []ImportEntry      `json:"entries" nullable:"false"`
	Diagnostics []ImportDiagnostic `json:"diagnostics" nullable:"false"`
}

func literalAlias(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || i > 0 && (c == '-' || c == '.')) {
			return false
		}
	}
	return true
}

func parseImport(source string) ImportPreview {
	result := ImportPreview{Issues: []ImportIssue{}, Entries: []ImportEntry{}, Diagnostics: []ImportDiagnostic{}}
	report := func(line int, message string) {
		result.Diagnostics = append(result.Diagnostics, ImportDiagnostic{line, message})
	}
	if len(source) > MaxImportBytes || !utf8.ValidString(source) {
		report(0, "config must be valid UTF-8 and at most 64 KiB")
		return result
	}
	var current *ImportEntry
	seen := map[string]bool{}
	names := map[string]bool{}
	finish := func() {
		if current == nil {
			return
		}
		for _, required := range []string{"hostname", "user", "identityfile"} {
			if !seen[required] {
				report(current.Line, "missing required directive: "+required)
			}
		}
		fields := ConnectionFields{Name: current.Name, Host: current.Host, Port: current.Port, Username: current.Username, SSHKeyID: "00000000-0000-0000-0000-000000000000"}
		if err := fields.normalize(); err != nil {
			report(current.Line, err.Error())
		} else {
			current.Host = fields.Host
		}
	}
	for index, raw := range strings.Split(source, "\n") {
		line := index + 1
		raw = strings.TrimSuffix(raw, "\r")
		if len(raw) > 4096 {
			report(line, "line exceeds 4096 bytes")
			continue
		}
		if strings.ContainsFunc(raw, func(c rune) bool { return unicode.IsControl(c) && c != '\t' }) {
			report(line, "control characters are unsupported")
			continue
		}
		raw = strings.Trim(raw, " \t")
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		parts := strings.Fields(raw)
		directive := strings.ToLower(parts[0])
		if directive == "host" {
			finish()
			current = nil
			seen = map[string]bool{}
		}
		if len(parts) != 2 || strings.ContainsAny(raw, "=\"'\\#%$") {
			report(line, "use one unquoted value; quoting, expansion, escapes, equals, and inline comments are unsupported")
			continue
		}
		value := parts[1]
		if directive == "host" {
			if !literalAlias(value) {
				report(line, "Host requires one literal alias without patterns")
				continue
			}
			if len(result.Entries) >= MaxImportEntries {
				report(line, "config exceeds 100 Host blocks")
				return result
			}
			if names[value] {
				report(line, "duplicate Host alias")
			}
			names[value] = true
			result.Entries = append(result.Entries, ImportEntry{Name: value, Port: 22, Line: line})
			current = &result.Entries[len(result.Entries)-1]
			continue
		}
		if current == nil {
			report(line, "directive outside a supported Host block; global settings and inheritance are unsupported")
			continue
		}
		if seen[directive] {
			report(line, "duplicate directive: "+directive)
			continue
		}
		seen[directive] = true
		switch directive {
		case "hostname":
			current.Host = value
		case "user":
			current.Username = value
		case "identityfile":
			current.Identity = value
		case "port":
			port, err := strconv.ParseInt(value, 10, 32)
			if err != nil || strings.Trim(value, "0123456789") != "" || port < 1 || port > 65535 {
				report(line, "Port must be decimal 1–65535")
			} else {
				current.Port = port
			}
		case "proxyjump":
			if !literalAlias(value) || strings.EqualFold(value, "none") {
				report(line, "ProxyJump requires one literal Host alias; endpoint overrides and multiple hops are unsupported")
			} else {
				current.Jump = value
			}
		default:
			report(line, "unsupported directive: "+parts[0])
		}
	}
	finish()
	if len(result.Entries) == 0 {
		report(0, "config contains no supported Host entries")
	}
	return result
}
