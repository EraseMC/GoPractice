package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/combat"
	"github.com/EraseMC/GoPractice/internal/command"
	"github.com/EraseMC/GoPractice/internal/config"
	"github.com/EraseMC/GoPractice/internal/db"
	"github.com/EraseMC/GoPractice/internal/handler"
	"github.com/EraseMC/GoPractice/internal/hub"
	"github.com/EraseMC/GoPractice/internal/info"
	"github.com/EraseMC/GoPractice/internal/items"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/language"
	"github.com/EraseMC/GoPractice/internal/netmw"
	"github.com/EraseMC/GoPractice/internal/overall"
	"github.com/EraseMC/GoPractice/internal/publicip"
	"github.com/EraseMC/GoPractice/internal/punish"
	"github.com/EraseMC/GoPractice/internal/rcon"
	"github.com/EraseMC/GoPractice/internal/scheduler"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/stats"
	"github.com/EraseMC/GoPractice/internal/store"
	"github.com/EraseMC/GoPractice/internal/telegram"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
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
	Punish    *punish.Manager
	Chat      *chat.Manager
	Telegram  *telegram.Client
	Commands  *command.Map
	Toggles   *toggles.Manager
	Stats     *stats.Manager
	Info      *info.Manager
	Hub       *hub.Manager
	Overall   *overall.Handler
	Combat    *combat.Manager
	Items     *items.Manager
	PublicIP  *publicip.Resolver

	OfflineXUID bool
	Port        int

	OnJoin       []func(o *state.Online, p *player.Player, tx *world.Tx)
	OnRankChange []func(o *state.Online)

	cancel context.CancelFunc
	stop   func()
	rcon   *rcon.Server
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
	a.Punish = punish.New(bundle, st)
	a.Chat = chat.New(log, bundle, st, players, a.Punish, a.Scheduler.Tick)
	a.Telegram = telegram.New(log, env)
	a.PublicIP = &publicip.Resolver{}
	a.setupHub()
	a.setupCommands()

	a.Router.OnQuit(a.quit)
	return a, nil
}

func (a *App) setupHub() {
	a.Toggles = toggles.New(a.Lang, a.Store, a.Players, a.Forms)
	a.Stats = stats.New(a.Lang, a.Store, a.Players, a.Forms)
	a.Info = info.New(a.Lang, a.Players, a.Forms)
	a.Hub = hub.New(hub.Deps{
		Log:      a.Log,
		Lang:     a.Lang,
		Store:    a.Store,
		Players:  a.Players,
		Forms:    a.Forms,
		Boards:   a.Boards,
		Chat:     a.Chat,
		Toggles:  a.Toggles,
		Stats:    a.Stats,
		Info:     a.Info,
		Language: a.Language,
	})
	a.Overall = &overall.Handler{}
	a.Combat = combat.New(a.Players, a.Toggles, a.Scheduler)
	a.Items = items.New(a.Lang, a.Players, a.Combat)

	back := func(p *player.Player, _ *world.Tx) { a.Hub.SendPreferencesForm(p) }
	a.Language.SetBack(back)
	a.Toggles.Back = back
	a.Info.Back = func(p *player.Player, _ *world.Tx) { a.Hub.SendMenuForm(p) }
	a.Toggles.OnScoreboard = func(o *state.Online, p *player.Player, _ *world.Tx, enabled bool) {
		if !enabled {
			a.Boards.Remove(p)
			return
		}
		if a.Hub.InHub(o) {
			a.Hub.UpdateScoreboard(o)
		}
	}
	a.Language.OnChange(func(p *player.Player, tx *world.Tx, _ lang.Lang) {
		if o, ok := a.Players.Get(p.UUID()); ok {
			a.Hub.LanguageChanged(o, p, tx)
		}
	})
	a.OnRankChange = append(a.OnRankChange, func(o *state.Online) {
		o.Do(func(_ *world.Tx, p *player.Player) {
			a.Hub.CheckFly(o, p)
		})
	})

	a.Net.OnRead(a.Toggles.Inbound)
	a.Net.OnRead(a.swingInbound)
	a.Router.Add(a.Combat.Handler())
	a.Router.Add(a.Items.Handler())
	a.Router.Add(a.Overall)
	a.Router.Add(a.Hub.Handler())
	a.Router.AddLast(a.Items.Tail())
	a.Router.AddLast(a.Combat.Tail())
}

func (a *App) swingInbound(c *netmw.Conn, pk packet.Packet) bool {
	ev, ok := pk.(*packet.LevelSoundEvent)
	if !ok || ev.SoundType != packet.SoundEventAttackNoDamage {
		return true
	}
	if o, ok := a.Players.Get(c.UUID()); ok {
		o.Do(func(tx *world.Tx, p *player.Player) { a.Combat.Swing(tx, p) })
	}
	return true
}

func (a *App) SetupWorld(w *world.World) error {
	return a.Hub.Setup(w)
}

func (a *App) TransferAddress(p *player.Player) string {
	ip := a.PublicIP.Get()
	if ip == "" {
		host, _, err := net.SplitHostPort(p.ServerAddress())
		if err != nil {
			return ""
		}
		ip = host
	}
	return net.JoinHostPort(ip, strconv.Itoa(a.Port))
}

func (a *App) Configure(conf *server.Config) {
	conf.Listeners = a.Net.Wrap(conf.Listeners)
}

func (a *App) SetStop(f func()) {
	a.stop = f
}

func (a *App) Start(ctx context.Context) {
	ctx, a.cancel = context.WithCancel(ctx)
	a.Scheduler.Repeat(1, a.Boards.Flush)
	a.Scheduler.Repeat(1, a.Items.Tick)
	a.Scheduler.Repeat(20, a.updateNameTags)
	a.Scheduler.Repeat(20, a.Hub.Tick)
	a.Stats.UpdateTops()
	a.Scheduler.Repeat(6000, a.Stats.UpdateTops)
	go a.PublicIP.Resolve(ctx)
	go a.Scheduler.Run(ctx)
	a.startRcon()
}

func (a *App) Join(p *player.Player, tx *world.Tx) *state.Online {
	conn, _ := a.Net.Conn(p.UUID())
	locale := language.ClientLocale(p)
	if conn != nil {
		locale = conn.ClientData().LanguageCode
	}

	l := lang.FromLocale(locale)
	xuid := p.XUID()
	if xuid == "" && a.OfflineXUID {
		xuid = "offline-" + strings.ToLower(p.Name())
	}
	if xuid != "" {
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

	o := a.Players.Add(p, xuid, conn, l)
	p.Handle(a.Router.Handler())
	a.Punish.OnJoin(o.Name, o.XUID)
	p.SetNameTag(a.Chat.NameTag(o))
	a.SyncCommands(p)
	a.Stats.Join(o)
	guard := a.Hub.InventoryHandler(o.UUID)
	p.Inventory().Handle(guard)
	p.Armour().Handle(guard)
	a.Hub.Join(o, p, tx)
	for _, f := range a.OnJoin {
		f(o, p, tx)
	}
	return o
}

func (a *App) quit(p *player.Player) {
	if o, ok := a.Players.Remove(p.UUID()); ok {
		a.Hub.Quit(o)
		a.Stats.Quit(o)
		a.Chat.Forget(o)
		a.Punish.OnQuit(o.Name)
		a.Commands.Forget(o.Name)
	}
	a.Toggles.Forget(p.UUID())
	a.Forms.Reset(p.UUID())
	a.Boards.Forget(p.UUID())
}

func (a *App) Close() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.rcon != nil {
		_ = a.rcon.Close()
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
