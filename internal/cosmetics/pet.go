package cosmetics

import (
	"maps"
	"math/rand/v2"
	"time"

	"github.com/EraseMC/GoPractice/internal/netmw"
	"github.com/EraseMC/GoPractice/internal/rank"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type pet struct {
	player.NopHandler
	manager   *Manager
	owner     *state.Online
	handle    *world.EntityHandle
	viewers   map[*netmw.Conn]petView
	lastEmote time.Time
}

type petView struct {
	riderID, ownerID int64
}

func (m *Manager) Enter(tx *world.Tx, p *player.Player) {
	o, ok := m.Players.Get(p.UUID())
	if !ok || o.Handle != p.H() || !m.Hub.InHub(o) || tx.World() != m.Hub.World() {
		return
	}
	profile := m.profile(o)
	if !profile.Cosmetics.Capybara || !rank.Has(profile.Rank, rank.CosmeticsCapybara) {
		return
	}
	id := uuid.New()
	name := ""
	if rand.IntN(1000) == 0 {
		name = "§6coconut doggy"
	}
	conf := player.Config{Name: name, UUID: id, Skin: m.petSkin, Position: p.Position(), Rotation: p.Rotation(), GameMode: world.GameModeAdventure}
	h := world.EntitySpawnOpts{ID: id, Position: p.Position(), Rotation: p.Rotation()}.New(player.Type, conf)
	pet := &pet{manager: m, owner: o, handle: h, viewers: make(map[*netmw.Conn]petView)}
	m.mu.Lock()
	if m.pets[o.UUID] != nil {
		m.mu.Unlock()
		return
	}
	m.pets[o.UUID], m.petsByID[id] = pet, pet
	m.mu.Unlock()

	actor := tx.AddEntity(h).(*player.Player)
	actor.Handle(pet)
	actor.SetAlwaysShowNameTag(name != "")
	pet.linkViewers(tx, p)
}

func (m *Manager) Leave(tx *world.Tx, p *player.Player) {
	m.mu.Lock()
	pet := m.pets[p.UUID()]
	m.mu.Unlock()
	if pet == nil || pet.owner.Handle != p.H() {
		return
	}
	if e, ok := pet.handle.Entity(tx); ok {
		pet.remove(e.(*player.Player))
	}
}

func (m *Manager) forgetViewer(c *netmw.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, pet := range m.views[c] {
		delete(pet.viewers, c)
	}
	delete(m.views, c)
}

func (p *pet) remove(actor *player.Player) {
	m := p.manager
	m.mu.Lock()
	if m.pets[p.owner.UUID] != p {
		m.mu.Unlock()
		return
	}
	delete(m.pets, p.owner.UUID)
	delete(m.petsByID, p.handle.UUID())
	views := p.viewers
	for c, view := range views {
		delete(m.views[c], view.riderID)
		if len(m.views[c]) == 0 {
			delete(m.views, c)
		}
	}
	p.viewers = nil
	m.mu.Unlock()
	for c, view := range views {
		if view.ownerID != 0 {
			_ = c.WritePacket(petLink(view, protocol.EntityLinkRemove))
		}
	}
	_ = actor.Close()
}

func (p *pet) HandleTick(ctx *player.Context, _ int64) {
	ctx.Cancel()
	actor, tx := ctx.Player(), ctx.Player().Tx()
	e, ok := p.owner.Handle.Entity(tx)
	if !ok || !p.manager.Hub.InHub(p.owner) {
		p.remove(actor)
		return
	}
	owner := e.(*player.Player)
	delta := owner.Position().Sub(actor.Position())
	yaw := owner.Rotation().Yaw() - actor.Rotation().Yaw()
	pitch := owner.Rotation().Pitch() - actor.Rotation().Pitch()
	if delta.LenSqr() > 0 || yaw != 0 || pitch != 0 {
		actor.Move(delta, yaw, pitch)
	}
	p.linkViewers(tx, owner)
}

func (p *pet) HandleHurt(ctx *player.Context, _ *float64, _ bool, _ *time.Duration, _ world.DamageSource) {
	ctx.Cancel()
}

func petLink(view petView, kind byte) *packet.SetActorLink {
	return &packet.SetActorLink{EntityLink: protocol.EntityLink{
		RiddenEntityUniqueID: view.ownerID, RiderEntityUniqueID: view.riderID, Type: kind, Immediate: true,
	}}
}

func (p *pet) linkViewers(tx *world.Tx, owner *player.Player) {
	m := p.manager
	m.mu.Lock()
	var pending map[*netmw.Conn]petView
	for c, view := range p.viewers {
		if view.ownerID == 0 {
			if pending == nil {
				pending = make(map[*netmw.Conn]petView)
			}
			pending[c] = view
		}
	}
	m.mu.Unlock()
	for c, view := range pending {
		o, ok := m.Players.Get(c.UUID())
		if !ok || o.Conn != c {
			continue
		}
		e, ok := o.Handle.Entity(tx)
		if !ok {
			continue
		}
		viewer := e.(*player.Player)
		id, ok := viewer.EntityRuntimeID(owner)
		if !ok {
			continue
		}
		m.mu.Lock()
		current, shown := p.viewers[c]
		link := shown && current == view
		if link {
			view.ownerID = int64(id)
			p.viewers[c] = view
		}
		m.mu.Unlock()
		if link {
			viewer.SendPacket(petLink(view, protocol.EntityLinkRider))
		}
	}
}

func petMetadata(metadata protocol.EntityMetadata) {
	metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagRiding)
	metadata.SetFlag(protocol.EntityDataKeyFlags, protocol.EntityDataFlagNoAI)
	metadata[protocol.EntityDataKeySeatOffset] = mgl32.Vec3{0.84, 0, 0}
	metadata[protocol.EntityDataKeyWidth] = float32(0.6)
	metadata[protocol.EntityDataKeyHeight] = float32(0.4)
}

func (m *Manager) Outbound(c *netmw.Conn, pk packet.Packet) bool {
	switch v := pk.(type) {
	case *packet.AddPlayer:
		m.mu.Lock()
		for _, p := range m.views[c] {
			if p.owner.UUID == v.UUID {
				view := p.viewers[c]
				view.ownerID = 0
				p.viewers[c] = view
			}
		}
		if p := m.petsByID[v.UUID]; p != nil {
			if m.views[c] == nil {
				m.views[c] = make(map[int64]*pet)
			}
			rider := int64(v.EntityRuntimeID)
			m.views[c][rider] = p
			p.viewers[c] = petView{riderID: rider}
			petMetadata(v.EntityMetadata)
		}
		m.mu.Unlock()
	case *packet.SetActorData:
		m.mu.Lock()
		p := m.views[c][int64(v.EntityRuntimeID)]
		m.mu.Unlock()
		if p != nil {
			petMetadata(v.EntityMetadata)
		}
	case *packet.RemoveActor:
		m.mu.Lock()
		views := m.views[c]
		if p := views[v.EntityUniqueID]; p != nil {
			delete(p.viewers, c)
			delete(views, v.EntityUniqueID)
		}
		for _, p := range views {
			if view := p.viewers[c]; view.ownerID == v.EntityUniqueID {
				view.ownerID = 0
				p.viewers[c] = view
			}
		}
		if len(views) == 0 {
			delete(m.views, c)
		}
		m.mu.Unlock()
	}
	return true
}

func (m *Manager) Inbound(c *netmw.Conn, pk packet.Packet) bool {
	emote, ok := pk.(*packet.Emote)
	if !ok || emote.EntityRuntimeID != 1 {
		return true
	}
	id, err := uuid.Parse(emote.EmoteID)
	if err != nil {
		return true
	}
	o, ok := m.Players.Get(c.UUID())
	if ok && o.Conn == c && m.Hub.InHub(o) {
		o.Do(func(tx *world.Tx, p *player.Player) { m.emote(tx, p, id) })
	}
	return true
}

func (m *Manager) emote(tx *world.Tx, owner *player.Player, id uuid.UUID) {
	m.mu.Lock()
	p := m.pets[owner.UUID()]
	if p == nil || time.Since(p.lastEmote) < time.Second {
		m.mu.Unlock()
		return
	}
	p.lastEmote = time.Now()
	views := maps.Clone(p.viewers)
	m.mu.Unlock()

	for c, view := range views {
		if view.ownerID == 0 || c.Protocol() < 589 {
			continue
		}
		if o, ok := m.Players.Get(c.UUID()); ok && o.Conn == c {
			if e, ok := o.Handle.Entity(tx); ok {
				e.(*player.Player).SendPacket(&packet.Emote{
					EntityRuntimeID: uint64(view.riderID), EmoteID: id.String(),
					Flags: packet.EmoteFlagServerSide | packet.EmoteFlagMuteChat,
				})
			}
		}
	}
}
