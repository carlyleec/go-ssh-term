package connections

import (
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id.String() == value
}

// NormalizeHost provides the same endpoint spelling for saved configurations
// and later host-trust lookups. It does not resolve DNS or contact the host.
func NormalizeHost(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if ip, err := netip.ParseAddr(value); err == nil {
		if ip.Zone() != "" {
			return "", false
		}
		return ip.Unmap().String(), true
	}
	value = strings.ToLower(strings.TrimSuffix(value, "."))
	if len(value) == 0 || len(value) > 253 {
		return "", false
	}
	numeric := true
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", false
			}
			if c < '0' || c > '9' {
				numeric = false
			}
		}
	}
	if numeric {
		return "", false
	}
	return value, true
}

func (v *ConnectionFields) normalize() error {
	if v.JumpConnectionID != nil && !canonicalID(*v.JumpConnectionID) {
		return failure(400, "select an owned direct jump connection")
	}
	v.Name = strings.TrimSpace(v.Name)
	if !utf8.ValidString(v.Name) || v.Name == "" || utf8.RuneCountInString(v.Name) > 64 || strings.ContainsFunc(v.Name, unicode.IsControl) {
		return failure(400, "name must contain 1 to 64 characters without control characters")
	}
	host, ok := NormalizeHost(v.Host)
	if !ok {
		return failure(400, "host must be an ASCII hostname or an IP address without a port or zone")
	}
	v.Host = host
	if v.Port < 1 || v.Port > 65535 {
		return failure(400, "port must be between 1 and 65535")
	}
	v.Username = strings.TrimSpace(v.Username)
	if len(v.Username) == 0 || len(v.Username) > 64 {
		return failure(400, "username must contain 1 to 64 ASCII letters, digits, underscores, dots, or hyphens")
	}
	for i, c := range v.Username {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || i > 0 && (c == '.' || c == '-')) {
			return failure(400, "username must start with an ASCII letter, digit, or underscore and contain only letters, digits, underscores, dots, or hyphens")
		}
	}
	if !canonicalID(v.SSHKeyID) {
		return failure(400, "select an owned SSH key")
	}
	return nil
}
