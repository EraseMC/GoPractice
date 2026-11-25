package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/EraseMC/GoPractice/internal/app"
	"github.com/EraseMC/GoPractice/internal/config"
	"github.com/EraseMC/GoPractice/internal/netver"
	"github.com/EraseMC/GoPractice/internal/worlds"
	"github.com/df-mc/dragonfly/server/player/chat"
	"github.com/df-mc/dragonfly/server/world"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	chat.Global.Subscribe(chat.StdoutSubscriber{})

	path := flag.String("config", "config.toml", "config path")
	envPath := flag.String("env", ".env", "env path")
	demo := flag.Bool("demo", false, "send test form and scoreboard on join")
	offline := flag.Bool("offline-xuid", false, "store players without Xbox auth under a name based XUID (testing only)")
	flag.Parse()

	ctx := context.Background()
	a, err := app.New(ctx, log, *envPath)
	if err != nil {
		log.Error("start practice", "err", err)
		os.Exit(1)
	}
	defer a.Close()

	uc, err := config.Load(*path)
	if err != nil {
		log.Error("load config", "err", err)
		os.Exit(1)
	}
	lobby, err := worlds.LoadSnapshot(log, uc.World.Folder)
	if err != nil {
		log.Error("load lobby", "err", err)
		os.Exit(1)
	}
	uc.World.SaveData = false
	conf, err := uc.Config(log)
	if err != nil {
		log.Error("build config", "err", err)
		os.Exit(1)
	}
	conf.ReadOnlyWorld = true
	conf.WorldProvider = lobby.Provider()
	conf.Generator = func(world.Dimension) world.Generator { return world.NopGenerator{} }
	conf.AcceptedProtocolsProvider = netver.Provider(netver.DefaultBlocked)
	a.OfflineXUID = *offline
	a.Configure(&conf)
	if *demo {
		installDemo(a)
	}

	srv := conf.New()
	srv.CloseOnProgramEnd()
	w := srv.World()
	w.SetTime(1000)
	w.StopTime()

	a.SetStop(func() { _ = srv.Close() })
	a.Start(ctx)
	go a.Console(os.Stdin)
	srv.Listen()
	for p := range srv.Accept() {
		a.Join(p, p.Tx())
		log.Info("player joined", "name", p.Name())
	}
}
