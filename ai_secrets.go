package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// aiSecretStore is the only place the plaintext AI API key is persisted.
// macOS uses the login Keychain; other platforms fall back to a 0600 file next
// to config.json. Tests replace the package-level value with a memory store.
type aiSecretStore interface {
	Get() (string, error)
	Set(string) error
	Delete() error
}

var aiSecrets aiSecretStore = defaultAISecretStore()

func defaultAISecretStore() aiSecretStore {
	if runtime.GOOS == "darwin" && securityCommandAvailable() {
		return keychainAISecretStore{}
	}
	return fileAISecretStore{path: filepath.Join(filepath.Dir(getConfigPath()), "ai.key")}
}

func securityCommandAvailable() bool {
	_, err := exec.LookPath("security")
	return err == nil
}

const (
	aiKeychainService = "com.wgit.ai"
	aiKeychainAccount = "api-key"
)

// keychainAISecretStore uses the macOS `security` CLI. The password is fed on
// stdin (security prompts for it twice when -w is the last option), so the
// secret never appears in the process argument list.
type keychainAISecretStore struct{}

func keychainCommand(args ...string) (*exec.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	return exec.CommandContext(ctx, "security", args...), cancel
}

func (keychainAISecretStore) Get() (string, error) {
	cmd, cancel := keychainCommand("find-generic-password", "-s", aiKeychainService, "-a", aiKeychainAccount, "-w")
	defer cancel()
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// `security` exits 44 when the item does not exist and 51 when the
			// keychain is locked; both mean "no usable key" to the caller.
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (keychainAISecretStore) Set(key string) error {
	cmd, cancel := keychainCommand("add-generic-password", "-U", "-s", aiKeychainService, "-a", aiKeychainAccount, "-w")
	defer cancel()
	// security prompts twice (password + confirmation) and reads both answers
	// from stdin, keeping the secret out of `ps`.
	cmd.Stdin = strings.NewReader(key + "\n" + key + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain add failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (keychainAISecretStore) Delete() error {
	cmd, cancel := keychainCommand("delete-generic-password", "-s", aiKeychainService, "-a", aiKeychainAccount)
	defer cancel()
	_, _ = cmd.CombinedOutput()
	// A missing item is the desired end state.
	return nil
}

// fileAISecretStore is the non-macOS fallback. The file is mode 0600 and lives
// outside config.json, so backups or support bundles that copy the config do
// not carry the secret.
type fileAISecretStore struct{ path string }

func (s fileAISecretStore) Get() (string, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s fileAISecretStore) Set(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("AI key is empty")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(key), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s fileAISecretStore) Delete() error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// memoryAISecretStore backs the test suite so tests never touch the real
// Keychain.
type memoryAISecretStore struct {
	mu  sync.Mutex
	key string
}

func (s *memoryAISecretStore) Get() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.key, nil
}

func (s *memoryAISecretStore) Set(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = key
	return nil
}

func (s *memoryAISecretStore) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = ""
	return nil
}

// =========================================================================
// Per-origin keyring
//
// The OS secret store holds one value. To keep keys for several API origins
// without touching the Keychain schema, that value is a small JSON envelope:
//   {"version":2,"keys":{"http://host:port":"<plaintext>"}}
// A raw value from an older build is treated as a legacy key and is only
// exposed for the saved config origin.
// =========================================================================

type aiKeyringEnvelope struct {
	Version int               `json:"version"`
	Keys    map[string]string `json:"keys"`
}

func parseAIKeyStore(raw string) (map[string]string, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, ""
	}
	if strings.HasPrefix(trimmed, "{") {
		var envelope aiKeyringEnvelope
		if json.Unmarshal([]byte(trimmed), &envelope) == nil && envelope.Version == 2 && envelope.Keys != nil {
			return envelope.Keys, ""
		}
	}
	return nil, trimmed
}

func readAIKeyring() (map[string]string, string, error) {
	raw, err := aiSecrets.Get()
	if err != nil {
		return nil, "", err
	}
	keys, legacy := parseAIKeyStore(raw)
	return keys, legacy, nil
}

func writeAIKeyring(keys map[string]string, legacy string) error {
	if len(keys) == 0 {
		if legacy != "" {
			return aiSecrets.Set(legacy)
		}
		return aiSecrets.Delete()
	}
	data, err := json.Marshal(aiKeyringEnvelope{Version: 2, Keys: keys})
	if err != nil {
		return err
	}
	return aiSecrets.Set(string(data))
}

func getAIStoredKeyForOrigin(origin string, allowLegacy bool) (string, error) {
	keys, legacy, err := readAIKeyring()
	if err != nil {
		return "", err
	}
	if key := strings.TrimSpace(keys[origin]); key != "" {
		return key, nil
	}
	if allowLegacy && strings.TrimSpace(legacy) != "" {
		// Bind the pre-keyring value to the first saved origin that reads it,
		// then persist the envelope so a later endpoint change cannot reuse it.
		key := strings.TrimSpace(legacy)
		if keys == nil {
			keys = map[string]string{}
		}
		keys[origin] = key
		_ = writeAIKeyring(keys, "")
		return key, nil
	}
	return "", nil
}

func setAIStoredKeyForOrigin(origin, key string) error {
	keys, legacy, err := readAIKeyring()
	if err != nil {
		return err
	}
	if keys == nil {
		keys = map[string]string{}
	}
	keys[origin] = key
	return writeAIKeyring(keys, legacy)
}

func deleteAIStoredKeyForOrigin(origin string, dropLegacy bool) error {
	keys, legacy, err := readAIKeyring()
	if err != nil {
		return err
	}
	if keys == nil {
		keys = map[string]string{}
	}
	delete(keys, origin)
	if dropLegacy {
		legacy = ""
	}
	return writeAIKeyring(keys, legacy)
}
