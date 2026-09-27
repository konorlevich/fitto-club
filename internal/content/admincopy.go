package content

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// AdminCopy is the admin UI's fixed strings for one language: a flat key ->
// text map. The admin speaks ru and ka to the editors; en is the reference
// bundle every other one is checked against (checklist §5), so a key added
// in one language must be added in all.
type AdminCopy map[string]string

// T returns the string for key, or the key itself in brackets so a missing
// string is visible on the page instead of silently empty.
func (c AdminCopy) T(key string) string {
	if v, ok := c[key]; ok {
		return v
	}
	return "[" + key + "]"
}

// LoadAdminCopy reads content/i18n/admin.<lang>.json. With a reference
// bundle given, the key sets must match exactly and no value may be empty.
func LoadAdminCopy(fsys fs.FS, lang string, ref AdminCopy) (AdminCopy, error) {
	b, err := fs.ReadFile(fsys, "content/i18n/admin."+lang+".json")
	if err != nil {
		return nil, fmt.Errorf("admin locale %s: %w", lang, err)
	}
	var c AdminCopy
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("admin locale %s: %w", lang, err)
	}
	var empty []string
	for k, v := range c {
		if strings.TrimSpace(v) == "" {
			empty = append(empty, k)
		}
	}
	if len(empty) > 0 {
		sort.Strings(empty)
		return nil, fmt.Errorf("admin locale %s has empty values: %s", lang, strings.Join(empty, ", "))
	}
	if ref != nil {
		var missing, extra []string
		for k := range ref {
			if _, ok := c[k]; !ok {
				missing = append(missing, k)
			}
		}
		for k := range c {
			if _, ok := ref[k]; !ok {
				extra = append(extra, k)
			}
		}
		sort.Strings(missing)
		sort.Strings(extra)
		if len(missing)+len(extra) > 0 {
			return nil, fmt.Errorf("admin locale %s differs from en: missing %v, extra %v", lang, missing, extra)
		}
	}
	return c, nil
}
