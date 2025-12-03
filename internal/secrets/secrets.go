package secrets

import (
	"os"
	"strings"
)

// Secret represents a single item (key without prefix + value).
type Secret struct {
	Key   string // e.g. "DB_PASSWORD"
	Value string
}

// Load returns a list of secrets from the environment + .env file.
func Load() ([]Secret, error) {
	var out []Secret

	for _, env := range os.Environ() {
		// env format is KEY=VAL
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]

		if !strings.HasPrefix(key, "ROLLWAVE_SECRET_") {
			continue
		}

		trimmed := strings.TrimPrefix(key, "ROLLWAVE_SECRET_")
		out = append(out, Secret{
			Key:   trimmed,
			Value: val,
		})
	}

	return out, nil
}
