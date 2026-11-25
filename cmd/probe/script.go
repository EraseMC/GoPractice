package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type probeState struct {
	mu      sync.Mutex
	inv     map[uint32]protocol.ItemInstance
	slot    uint32
	npc     uint64
	form    uint32
	hasForm bool
	pos     mgl32.Vec3
}

func itemName(st protocol.ItemStack) string {
	if d, ok := st.NBTData["display"].(map[string]any); ok {
		if n, ok := d["Name"].(string); ok {
			return n
		}
	}
	return ""
}

func short(s string) string {
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}

func runScript(conn *minecraft.Conn) error {
	st := &probeState{inv: map[uint32]protocol.ItemInstance{}, pos: conn.GameData().PlayerPosition}
	self := conn.GameData().EntityRuntimeID
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			pk, err := conn.ReadPacket()
			if err != nil {
				return
			}
			report(st, self, pk)
		}
	}()
	time.Sleep(time.Second)
	for _, line := range strings.Split(script.send, "|") {
		switch {
		case strings.HasPrefix(line, "@"):
			fmt.Printf("    > action %q\n", line)
			act(conn, st, self, line[1:])
		case strings.HasPrefix(line, "/"):
			fmt.Printf("    > command %q\n", line)
			_ = conn.WritePacket(&packet.CommandRequest{CommandLine: line, CommandOrigin: protocol.CommandOrigin{Origin: protocol.CommandOriginPlayer, UUID: uuid.New()}})
		default:
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

func report(st *probeState, self uint64, pk packet.Packet) {
	switch pk := pk.(type) {
	case *packet.Text:
		fmt.Printf("      text[%d]: %q\n", pk.TextType, pk.Message)
	case *packet.SetTitle:
		fmt.Printf("      title[%d]: %q\n", pk.ActionType, pk.Text)
	case *packet.LevelSoundEvent:
		fmt.Printf("      sound: %s extra=%d\n", pk.SoundType, pk.ExtraData)
	case *packet.Transfer:
		fmt.Printf("      transfer: %s:%d\n", pk.Address, pk.Port)
	case *packet.AvailableCommands:
		fmt.Printf("      commands (%d):", len(pk.Commands))
		for _, c := range pk.Commands {
			fmt.Printf(" %s", c.Name)
		}
		fmt.Println()
	case *packet.SetActorData:
		if v, ok := pk.EntityMetadata[protocol.EntityDataKeyName]; ok {
			fmt.Printf("      nametag %d: %q\n", pk.EntityRuntimeID, v)
		}
	case *packet.AddPlayer:
		st.mu.Lock()
		st.npc = pk.EntityRuntimeID
		st.mu.Unlock()
		fmt.Printf("      add player %d %q at %v scale=%v name=%q\n", pk.EntityRuntimeID, pk.Username, pk.Position, pk.EntityMetadata[protocol.EntityDataKeyScale], pk.EntityMetadata[protocol.EntityDataKeyName])
	case *packet.AddActor:
		fmt.Printf("      add actor %d %s at %v name=%q\n", pk.EntityRuntimeID, pk.EntityType, pk.Position, pk.EntityMetadata[protocol.EntityDataKeyName])
	case *packet.Emote:
		fmt.Printf("      emote %d %s flags=%d\n", pk.EntityRuntimeID, pk.EmoteID, pk.Flags)
	case *packet.MovePlayer:
		if pk.EntityRuntimeID == self {
			st.mu.Lock()
			st.pos = pk.Position
			st.mu.Unlock()
			fmt.Printf("      move self %v yaw=%v pitch=%v mode=%d\n", pk.Position, pk.Yaw, pk.Pitch, pk.Mode)
		}
	case *packet.SetPlayerGameType:
		fmt.Printf("      gametype: %d\n", pk.GameType)
	case *packet.UpdateAbilities:
		if len(pk.AbilityData.Layers) > 0 {
			fmt.Printf("      abilities: values=%b\n", pk.AbilityData.Layers[0].Values)
		}
	case *packet.InventoryContent:
		if pk.WindowID != protocol.WindowIDInventory {
			return
		}
		st.mu.Lock()
		for i, it := range pk.Content {
			st.inv[uint32(i)] = it
			if it.Stack.NetworkID != 0 {
				fmt.Printf("      inv[%d] id=%d %q\n", i, it.Stack.NetworkID, itemName(it.Stack))
			}
		}
		st.mu.Unlock()
	case *packet.InventorySlot:
		if pk.WindowID != protocol.WindowIDInventory {
			return
		}
		st.mu.Lock()
		st.inv[pk.Slot] = pk.NewItem
		st.mu.Unlock()
		fmt.Printf("      slot[%d] id=%d %q\n", pk.Slot, pk.NewItem.Stack.NetworkID, itemName(pk.NewItem.Stack))
	case *packet.SetDisplayObjective:
		fmt.Printf("      objective: %q\n", pk.DisplayName)
	case *packet.RemoveObjective:
		fmt.Printf("      remove objective\n")
	case *packet.SetScore:
		for _, e := range pk.Entries {
			fmt.Printf("      score %d %q\n", e.Score, e.DisplayName)
		}
	case *packet.ModalFormRequest:
		st.mu.Lock()
		st.form, st.hasForm = pk.FormID, true
		st.mu.Unlock()
		fmt.Printf("      form %d: %s\n", pk.FormID, short(string(pk.FormData)))
	}
}

func act(conn *minecraft.Conn, st *probeState, self uint64, action string) {
	name, arg, _ := strings.Cut(action, ":")
	st.mu.Lock()
	defer st.mu.Unlock()
	held := st.inv[st.slot]
	switch name {
	case "slot":
		n, _ := strconv.Atoi(arg)
		st.slot = uint32(n)
		_ = conn.WritePacket(&packet.MobEquipment{EntityRuntimeID: self, NewItem: st.inv[st.slot], InventorySlot: byte(n), HotBarSlot: byte(n), WindowID: protocol.WindowIDInventory})
	case "use":
		_ = conn.WritePacket(&packet.InventoryTransaction{TransactionData: &protocol.UseItemTransactionData{
			ActionType:       protocol.UseItemActionClickAir,
			TriggerType:      protocol.TriggerTypePlayerInput,
			HotBarSlot:       int32(st.slot),
			HeldItem:         held,
			Position:         st.pos,
			ClientPrediction: protocol.ClientPredictionSuccess,
		}})
	case "npc", "hit":
		target := st.npc
		if arg != "" {
			v, _ := strconv.ParseUint(arg, 10, 64)
			target = v
		}
		kind := int32(protocol.UseItemOnEntityActionAttack)
		if name == "npc" {
			kind = protocol.UseItemOnEntityActionInteract
		}
		_ = conn.WritePacket(&packet.InventoryTransaction{TransactionData: &protocol.UseItemOnEntityTransactionData{
			TargetEntityRuntimeID: target,
			ActionType:            kind,
			HotBarSlot:            int32(st.slot),
			HeldItem:              held,
			Position:              st.pos,
		}})
	case "answer":
		if !st.hasForm {
			fmt.Println("      no form to answer")
			return
		}
		resp := packet.ModalFormResponse{FormID: st.form}
		if arg == "close" {
			resp.CancelReason = protocol.Option[uint8](packet.ModalFormCancelReasonUserClosed)
		} else {
			resp.ResponseData = protocol.Option([]byte(arg))
		}
		st.hasForm = false
		_ = conn.WritePacket(&resp)
	case "move":
		parts := strings.Split(arg, ",")
		if len(parts) != 3 {
			return
		}
		var v mgl32.Vec3
		for i, p := range parts {
			f, _ := strconv.ParseFloat(p, 32)
			v[i] = float32(f)
		}
		st.pos = v
		_ = conn.WritePacket(&packet.MovePlayer{EntityRuntimeID: self, Position: v, Mode: packet.MoveModeNormal})
	}
}
