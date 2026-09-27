// Command fitto-club serves the Fitto Club site as a single self-contained
// binary: templates, locale bundles and static assets are embedded; the only
// state outside the binary is the content database on DATA_DIR.
package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/konorlevich/fitto-club/internal/admin"
	"github.com/konorlevich/fitto-club/internal/content"
	"github.com/konorlevich/fitto-club/internal/handler"
	"github.com/konorlevich/fitto-club/internal/render"
	"github.com/konorlevich/fitto-club/internal/site"
	"github.com/konorlevich/fitto-club/internal/store"
	"github.com/sirupsen/logrus"
)

//go:embed web/templates
var templatesFS embed.FS

//go:embed content/i18n/*.json
var contentFS embed.FS

//go:embed web/static
var staticFS embed.FS

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339})
	log.SetOutput(os.Stdout)

	resetOwner := flag.Bool("reset-owner", false, "clear the owner's admin password so the ENV bootstrap works again, then exit")
	flag.Parse()

	if err := site.LoadDotEnv(".env"); err != nil {
		log.WithError(err).Fatal("reading .env")
	}
	cfg, err := site.Load()
	if err != nil {
		log.WithError(err).Fatal("configuration is invalid")
	}

	// Launch gate: facts still awaiting the club's confirmation block a
	// production boot (TASKS.md #26). ALLOW_PENDING=1 is the explicit,
	// visible override for a staging deploy.
	if len(content.Pending) > 0 {
		f := log.WithField("pending", content.Pending)
		if cfg.IsProd() && !cfg.AllowPending {
			f.Fatal("unconfirmed facts would ship; confirm them with the club or set ALLOW_PENDING=1 for staging")
		}
		f.Warn("serving facts that the club has not confirmed yet")
	}

	// Boot-time completeness gate: a missing string in any enabled locale is
	// fatal, so a half-translated page can never ship (checklist §5).
	copies := make(map[string]*content.SiteCopy, len(cfg.Locales))
	for _, lang := range cfg.Locales {
		c, err := content.LoadCopy(contentFS, lang)
		if err != nil {
			log.WithError(err).Fatal("locale bundle failed the completeness gate")
		}
		copies[lang] = c
	}

	tmpl, err := render.Parse(templatesFS, handler.Pages)
	if err != nil {
		log.WithError(err).Fatal("template parsing failed")
	}
	static, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		log.WithError(err).Fatal("static assets are missing")
	}
	assets, err := handler.Fingerprints(static)
	if err != nil {
		log.WithError(err).Fatal("hashing static assets failed")
	}
	inline, err := handler.BuildInline(static, assets)
	if err != nil {
		log.WithError(err).Fatal("preparing inline assets failed")
	}

	st, err := store.Open(cfg.DataDir, log)
	if err != nil {
		log.WithError(err).Fatal("opening the content database")
	}
	defer st.Close()

	if *resetOwner {
		if err := st.ResetOwner(); err != nil {
			log.WithError(err).Fatal("resetting the owner password")
		}
		log.Info("owner password cleared; log in with ADMIN_OWNER_LOGIN / ADMIN_OWNER_PASSWORD and set a new one")
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	// Daily consistent snapshot of the content database onto the volume,
	// last 14 kept. Copy them off the volume too (DEPLOY.md).
	go st.BackupLoop(ctx, 14, func(err error) { log.WithError(err).Error("database backup failed") })

	srv := &handler.Server{Cfg: cfg, Copy: copies, Tmpl: tmpl, Static: static, Assets: assets,
		Store: st, Log: log, Inline: inline}
	// The admin mounts only when an owner login is configured. Its UI
	// speaks ru and ka; en is the reference bundle the others are checked
	// against, the same gate as the site copy.
	if cfg.AdminEnabled() {
		adminCopy := map[string]content.AdminCopy{}
		ref, err := content.LoadAdminCopy(contentFS, "en", nil)
		if err != nil {
			log.WithError(err).Fatal("admin locale bundle failed the completeness gate")
		}
		adminCopy["en"] = ref
		for _, l := range admin.UILangs {
			c, err := content.LoadAdminCopy(contentFS, l, ref)
			if err != nil {
				log.WithError(err).Fatal("admin locale bundle failed the completeness gate")
			}
			adminCopy[l] = c
		}
		adm, err := admin.New(cfg, st, adminCopy, templatesFS, static, assets, inline.Fonts, log)
		if err != nil {
			log.WithError(err).Fatal("admin setup failed")
		}
		srv.Admin = adm.Mount
	} else {
		log.Warn("ADMIN_OWNER_LOGIN is empty: the admin is not mounted")
	}
	h := srv.Handler()
	// Pre-render every indexable page once, so the first visitor after a
	// deploy is served from memory too (checklist §2).
	if n, err := srv.Warm(h); err != nil {
		log.WithError(err).Fatal("pre-rendering failed")
	} else {
		log.WithField("pages", n).Info("pre-rendered")
	}

	httpSrv := &http.Server{
		Addr: cfg.Addr, Handler: h,
		ReadHeaderTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	go func() {
		log.WithFields(logrus.Fields{"addr": cfg.Addr, "locales": cfg.Locales, "env": cfg.Env,
			"tag": string(cfg.Tag.Kind)}).Info("server starting")
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.WithError(err).Fatal("server failed")
		}
	}()
	<-ctx.Done()
	log.Info("shutdown signal received, draining in-flight requests")
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.WithError(err).Error("graceful shutdown timed out")
		os.Exit(1)
	}
	log.Info("stopped cleanly")
}
