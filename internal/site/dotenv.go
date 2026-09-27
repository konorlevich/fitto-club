package site

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv reads KEY=VALUE lines from a .env file into the process
// environment, without overriding anything already set. It is deliberately
// tiny and deliberately literal: values are taken EXACTLY as written, with no
// variable expansion.
//
// That last part is the whole point. A bcrypt hash looks like $2a$12$..., and
// any loader that expands $VAR silently turns it into garbage that can never
// match a password - which presents as "the password is wrong" with no clue
// why. Quotes wrapping a whole value are stripped; nothing else is
// interpreted.
//
// A missing file is not an error: production sets real environment variables.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Strip one matching pair of wrapping quotes, and nothing more.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if key == "" {
			continue
		}
		if _, set := os.LookupEnv(key); set {
			continue // a real environment variable always wins
		}
		if err := os.Setenv(key, val); err != nil {
			return err
		}
	}
	return sc.Err()
}
