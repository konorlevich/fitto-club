// Package site holds process-wide configuration and the resolved analytics tag.
package site

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

// DefaultLocale is the reference locale and the hreflang x-default target.
const DefaultLocale = "en"

// Supported is every locale the site has copy for. LOCALES may enable a
// subset of these; nothing outside this list can ever be enabled.
var Supported = []string{"en", "ru", "ka"}

// TagKind is the resolved type of the Google tag id. Resolving once at boot
// means templates branch on a typed value instead of prefix-matching a string.
type TagKind string

const (
	TagNone TagKind = ""
	TagGTM  TagKind = "gtm"
	TagGA4  TagKind = "ga4"
)

type Tag struct {
	Kind TagKind
	ID   string
}

type Config struct {
	BaseURL string
	Locales []string
	Tag     Tag
	Env     string
	DataDir string
	Addr    string
	TZ      *time.Location
	// AllowPending lets a production boot proceed while facts still await the
	// club's confirmation (content.Pending). Off by default: a launch with an
	// unverified price is exactly what the gate exists to stop.
	AllowPending bool
	// NoIndex keeps a pre-launch deploy out of search engines: robots.txt
	// disallows everything, every page says noindex,nofollow, and every
	// response carries X-Robots-Tag. For the temporary *.up.railway.app
	// domain until the site launches on fitto.club.
	NoIndex bool
	// Admin bootstrap: the owner logs in once with these, then sets a real
	// password stored in the database; after that the ENV pair is ignored.
	// Empty login disables the admin entirely.
	AdminLogin, AdminPassword string
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// resolveTag turns whatever id was handed over into a typed value. Unset is a
// clean no-op; a set-but-unrecognized prefix is fatal, so a typo surfaces on
// the deploy rather than as months of missing data.
func resolveTag() (Tag, error) {
	raw := env("GTM_ID", "")
	if raw == "" {
		return Tag{Kind: TagNone}, nil
	}
	up := strings.ToUpper(raw)
	switch {
	case strings.HasPrefix(up, "GTM-"):
		return Tag{Kind: TagGTM, ID: raw}, nil
	case strings.HasPrefix(up, "G-"):
		return Tag{Kind: TagGA4, ID: raw}, nil
	case strings.HasPrefix(up, "UA-"):
		return Tag{}, fmt.Errorf("GTM_ID %q is a Universal Analytics id; UA stopped processing in 2023 - use a GTM- or G- id", raw)
	default:
		return Tag{}, fmt.Errorf("GTM_ID %q has an unrecognized prefix (expected GTM- or G-)", raw)
	}
}

func Load() (Config, error) {
	tag, err := resolveTag()
	if err != nil {
		return Config{}, err
	}
	var locales []string
	for l := range strings.SplitSeq(env("LOCALES", "en,ru,ka"), ",") {
		l = strings.ToLower(strings.TrimSpace(l))
		if l == "" {
			continue
		}
		if !slices.Contains(Supported, l) {
			return Config{}, fmt.Errorf("LOCALES contains %q; supported locales are %v", l, Supported)
		}
		locales = append(locales, l)
	}
	if !slices.Contains(locales, DefaultLocale) {
		return Config{}, fmt.Errorf("LOCALES must include the reference locale %q", DefaultLocale)
	}
	// Tbilisi has no DST; the fixed zone is the fallback when the host has no
	// tzdata, so "open now" can never silently drift to UTC.
	tz, err := time.LoadLocation("Asia/Tbilisi")
	if err != nil {
		tz = time.FixedZone("Asia/Tbilisi", 4*60*60)
	}
	return Config{
		// BASE_URL feeds canonical URLs, hreflang, the OG image and the
		// sitemap. The fallback is the real domain, which is correct for
		// production and harmless in development.
		BaseURL:       strings.TrimRight(env("BASE_URL", "https://fitto.club"), "/"),
		Locales:       locales,
		Tag:           tag,
		Env:           env("ENV", "development"),
		DataDir:       env("DATA_DIR", "./data"),
		Addr:          ":" + env("PORT", "8080"),
		TZ:            tz,
		AllowPending:  env("ALLOW_PENDING", "") == "1",
		NoIndex:       env("NOINDEX", "") == "1",
		AdminLogin:    env("ADMIN_OWNER_LOGIN", ""),
		AdminPassword: os.Getenv("ADMIN_OWNER_PASSWORD"),
	}, nil
}

func (c Config) HasLocale(l string) bool { return slices.Contains(c.Locales, l) }
func (c Config) AdminEnabled() bool      { return c.AdminLogin != "" }
func (c Config) IsProd() bool            { return c.Env == "production" }
