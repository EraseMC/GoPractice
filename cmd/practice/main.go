package main

import (
	"flag"
	"log/slog"
	"os"

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
	flag.Parse()
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

	srv := conf.New()
	srv.CloseOnProgramEnd()
	w := srv.World()
	w.SetTime(1000)
	w.StopTime()

	srv.Listen()
	for p := range srv.Accept() {
		log.Info("player joined", "name", p.Name())
	}
}
