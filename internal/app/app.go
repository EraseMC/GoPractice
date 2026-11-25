package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/EraseMC/GoPractice/internal/config"
	"github.com/EraseMC/GoPractice/internal/db"
	"github.com/EraseMC/GoPractice/internal/handler"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/language"
	"github.com/EraseMC/GoPractice/internal/netmw"
	"github.com/EraseMC/GoPractice/internal/scheduler"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
)

type App struct {
	Log       *slog.Logger
	Env       config.Env
	Lang      *lang.Bundle
	SQL       *sql.DB
	Pool      *db.Pool
	Store     *store.Store
	Scheduler *scheduler.Scheduler
	Net       *netmw.Middleware
	Players   *state.Players
	Forms     *ui.Forms
	Boards    *ui.Scoreboards
	Router    *handler.Router
	Language  *language.Manager

	OnJoin []func(o *state.Online, p *player.Player, tx *world.Tx)

	cancel context.CancelFunc
}

func New(ctx context.Context, log *slog.Logger, envPath string) (*App, error) {
	env, err := config.LoadEnv(envPath)
	if err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}
	bundle, err := lang.Load()
	if err != nil {
		return nil, fmt.Errorf("load translations: %w", err)
	}

	dbConf := db.ConfigFromEnv(env)
	sqlDB, err := db.Open(ctx, dbConf)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, sqlDB, log); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	started := time.Now()
	st, err := store.Load(ctx, sqlDB)
	if err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	log.Info(fmt.Sprintf("Loaded %d players and %d punishments in %.2fs", st.Count(), st.Punishments.Count(), time.Since(started).Seconds()))
	pool := db.NewPool(sqlDB, dbConf.Workers, log)
	st.Attach(pool)

	players := state.NewPlayers()
	forms := ui.NewForms()
	a := &App{
		Log:       log,
		Env:       env,
		Lang:      bundle,
		SQL:       sqlDB,
		Pool:      pool,
		Store:     st,
		Scheduler: scheduler.New(log),
		Net:       netmw.New(),
		Players:   players,
		Forms:     forms,
		Boards:    ui.NewScoreboards(players),
		Router:    handler.NewRouter(),
	}
	a.Language = language.New(log, bundle, st, players, forms)

	a.Router.OnQuit(a.quit)
	return a, nil
}

func (a *App) Configure(conf *server.Config) {
	conf.Listeners = a.Net.Wrap(conf.Listeners)
}

func (a *App) Start(ctx context.Context) {
	ctx, a.cancel = context.WithCancel(ctx)
	a.Scheduler.Repeat(1, a.Boards.Flush)
	go a.Scheduler.Run(ctx)
}

func (a *App) Join(p *player.Player, tx *world.Tx) *state.Online {
	conn, _ := a.Net.Conn(p.UUID())
	locale := language.ClientLocale(p)
	if conn != nil {
		locale = conn.ClientData().LanguageCode
	}

	l := lang.FromLocale(locale)
	if xuid := p.XUID(); xuid != "" {
		prof := a.Store.Login(store.Identity{
			XUID:     xuid,
			Name:     p.Name(),
			IP:       hostOf(p.Addr()),
			DeviceID: p.DeviceID(),
			Locale:   locale,
		})
		if pl, ok := lang.Parse(prof.Locale); ok {
			l = pl
		}
	} else {
		a.Log.Warn("player has no XUID, profile is not stored", "name", p.Name())
	}

	o := a.Players.Add(p, conn, l)
	p.Handle(a.Router.Handler())
	for _, f := range a.OnJoin {
		f(o, p, tx)
	}
	return o
}

func (a *App) quit(p *player.Player) {
	a.Players.Remove(p.UUID())
	a.Forms.Reset(p.UUID())
	a.Boards.Forget(p.UUID())
}

func (a *App) Close() {
	if a.cancel != nil {
		a.cancel()
	}
	a.Pool.Close()
	_ = a.SQL.Close()
}

func hostOf(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}
