package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	_ "time/tzdata"

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
	a.Port = listenPort(uc.Network.Address)
	a.Configure(&conf)

	srv := conf.New()
	srv.CloseOnProgramEnd()
	w := srv.World()
	w.SetTime(1000)
	w.StopTime()
	w.SetDefaultGameMode(world.GameModeAdventure)
	if err := a.SetupWorld(w, filepath.Dir(uc.World.Folder)); err != nil {
		log.Error("setup hub", "err", err)
		os.Exit(1)
	}

	a.SetStop(func() { _ = srv.Close() })
	a.Start(ctx)
	go a.Console(os.Stdin)
	srv.Listen()
	for p := range srv.Accept() {
		a.Join(p, p.Tx())
		log.Info("player joined", "name", p.Name())
	}
}

func listenPort(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 19132
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return 19132
	}
	return n
}
