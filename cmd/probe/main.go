package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/EraseMC/GoPractice/internal/netver"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19132", "server address")
	only := flag.Int("proto", 0, "probe only this protocol ID (0 = all)")
	uiWait := flag.Duration("ui", 0, "after spawn, answer forms and report ui packets for this long")
	name := flag.String("name", "", "display name (default probe<protocol>)")
	send := flag.String("send", "", "lines to send after spawn separated by |, lines starting with / are commands")
	gap := flag.Duration("gap", 2500*time.Millisecond, "delay between sent lines")
	stay := flag.Duration("stay", 3*time.Second, "how long to stay after the last line")
	flag.Parse()
	script = scriptConf{name: *name, send: *send, gap: *gap, stay: *stay}

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
		err := join(*addr, p, *uiWait)
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

type scriptConf struct {
	name, send string
	gap, stay  time.Duration
}

var script scriptConf

func join(addr string, p minecraft.Protocol, uiWait time.Duration) error {
	d := minecraft.Dialer{Protocol: p}
	d.IdentityData.DisplayName = fmt.Sprintf("probe%d", p.ID())
	if script.name != "" {
		d.IdentityData.DisplayName = script.name
	}
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
	if uiWait > 0 {
		return watchUI(conn, uiWait)
	}
	if script.send != "" {
		return runScript(conn)
	}
	return nil
}

func runScript(conn *minecraft.Conn) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				return
			}
			switch pk := pk.(type) {
			case *packet.Text:
				fmt.Printf("      text[%d]: %q\n", pk.TextType, pk.Message)
			case *packet.AvailableCommands:
				fmt.Printf("      commands (%d):", len(pk.Commands))
				for _, c := range pk.Commands {
					fmt.Printf(" %s", c.Name)
				}
				fmt.Println()
				for _, c := range pk.Commands {
					fmt.Printf("        /%s %q overloads=%d\n", c.Name, c.Description, len(c.Overloads))
				}
			case *packet.SetActorData:
				if v, ok := pk.EntityMetadata[protocol.EntityDataKeyName]; ok && pk.EntityRuntimeID == conn.GameData().EntityRuntimeID {
					fmt.Printf("      nametag: %q\n", v)
				}
			}
		}
	}()
	time.Sleep(time.Second)
	for _, line := range strings.Split(script.send, "|") {
		if strings.HasPrefix(line, "/") {
			fmt.Printf("    > command %q\n", line)
			_ = conn.WritePacket(&packet.CommandRequest{CommandLine: line, CommandOrigin: protocol.CommandOrigin{Origin: protocol.CommandOriginPlayer, UUID: uuid.New()}})
		} else {
			fmt.Printf("    > chat %q\n", line)
			_ = conn.WritePacket(&packet.Text{TextType: packet.TextTypeChat, SourceName: conn.IdentityData().DisplayName, Message: line, XUID: conn.IdentityData().XUID})
		}
		time.Sleep(script.gap)
	}
	time.Sleep(script.stay)
	_ = conn.Close()
	<-done
	return nil
}

func watchUI(conn *minecraft.Conn, wait time.Duration) error {
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	answers := [][]byte{[]byte("1"), []byte("true")}
	forms, scores, objectives := 0, 0, 0
	for {
		pk, err := conn.ReadPacket()
		if err != nil {
			break
		}
		switch pk := pk.(type) {
		case *packet.Text:
			fmt.Printf("      text: %q\n", pk.Message)
		case *packet.SetDisplayObjective:
			objectives++
			fmt.Printf("      objective: %q\n", pk.DisplayName)
		case *packet.SetScore:
			scores++
			for _, e := range pk.Entries {
				fmt.Printf("      score %d type=%d %q\n", e.Score, e.IdentityType, e.DisplayName)
			}
		case *packet.ModalFormRequest:
			forms++
			fmt.Printf("      form %d: %s\n", pk.FormID, pk.FormData)
			if len(answers) > 0 {
				_ = conn.WritePacket(&packet.ModalFormResponse{FormID: pk.FormID, ResponseData: protocol.Option(answers[0])})
				answers = answers[1:]
			}
		}
	}
	if forms < 2 || objectives == 0 || scores == 0 {
		return fmt.Errorf("ui: forms=%d objectives=%d scores=%d", forms, objectives, scores)
	}
	return nil
}
