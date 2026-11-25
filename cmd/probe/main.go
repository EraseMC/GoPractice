package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/EraseMC/GoPractice/internal/netver"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19132", "server address")
	only := flag.Int("proto", 0, "probe only this protocol ID (0 = all)")
	flag.Parse()

	world.DefaultBlockRegistry.Finalize()
	protocols, err := netver.Provider(netver.DefaultBlocked)(world.DefaultBlockRegistry)
	if err != nil {
		fmt.Println("build protocols:", err)
		os.Exit(1)
	}
	protocols = append(protocols, minecraft.DefaultProtocol)

	failed := false
	for _, p := range protocols {
		if *only != 0 && p.ID() != int32(*only) {
			continue
		}
		start := time.Now()
		err := join(*addr, p)
		status := "OK"
		if err != nil {
			status, failed = "FAIL "+err.Error(), true
		}
		fmt.Printf("%5d %-12s %-6s %s\n", p.ID(), p.Ver(), time.Since(start).Round(time.Millisecond), status)
	}
	if failed {
		os.Exit(1)
	}
}

func join(addr string, p minecraft.Protocol) error {
	d := minecraft.Dialer{Protocol: p}
	d.IdentityData.DisplayName = fmt.Sprintf("probe%d", p.ID())
	d.DownloadResourcePack = func(uuid.UUID, string, int, int) bool { return true }
	network := "raknet"
	if p.ID() < 649 {
		network = "raknet-v10"
	}
	conn, err := d.DialTimeout(network, addr, 15*time.Second)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	if err := conn.DoSpawnTimeout(15 * time.Second); err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	return nil
}
