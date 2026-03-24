package customers

import (
	"fmt"
	"os"
)

// KeyProvider supplies encryption keys for PII data.
// Implementations must support versioned keys for key rotation.
type KeyProvider interface {
	// PIIKey returns the key for the current version (used for new encryptions).
	PIIKey() ([]byte, error)
	// PIIKeyByVersion returns the key for a specific version (used for decryption).
	PIIKeyByVersion(version int) ([]byte, error)
	// CurrentKeyVersion returns the version number of the current key.
	CurrentKeyVersion() int
}

// FixedKeyProvider returns a hardcoded key at version 1. Suitable for WASM/testing.
type FixedKeyProvider struct{ Key []byte }

func (f FixedKeyProvider) PIIKey() ([]byte, error) { return f.Key, nil }
func (f FixedKeyProvider) PIIKeyByVersion(version int) ([]byte, error) {
	if version != 1 {
		return nil, fmt.Errorf("unknown key version %d", version)
	}
	return f.Key, nil
}
func (f FixedKeyProvider) CurrentKeyVersion() int { return 1 }

// EnvKeyProvider reads the key from an environment variable. Single version.
type EnvKeyProvider struct{ VarName string }

func (e EnvKeyProvider) PIIKey() ([]byte, error) {
	key := os.Getenv(e.VarName)
	if key == "" {
		return nil, fmt.Errorf("env var %s not set", e.VarName)
	}
	return []byte(key), nil
}
func (e EnvKeyProvider) PIIKeyByVersion(version int) ([]byte, error) {
	if version != 1 {
		return nil, fmt.Errorf("unknown key version %d", version)
	}
	return e.PIIKey()
}
func (e EnvKeyProvider) CurrentKeyVersion() int { return 1 }

// VersionedKeyProvider holds multiple keys indexed by version.
// The current version is used for new encryptions; older versions remain
// available for decryption until all rows are rotated.
type VersionedKeyProvider struct {
	Keys    map[int][]byte
	Current int
}

func (v VersionedKeyProvider) PIIKey() ([]byte, error) {
	return v.PIIKeyByVersion(v.Current)
}

func (v VersionedKeyProvider) PIIKeyByVersion(version int) ([]byte, error) {
	key, ok := v.Keys[version]
	if !ok {
		return nil, fmt.Errorf("unknown key version %d", version)
	}
	return key, nil
}

func (v VersionedKeyProvider) CurrentKeyVersion() int { return v.Current }
