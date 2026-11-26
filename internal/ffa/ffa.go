package ffa

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/EraseMC/GoPractice/internal/chat"
	"github.com/EraseMC/GoPractice/internal/combat"
	"github.com/EraseMC/GoPractice/internal/hub"
	"github.com/EraseMC/GoPractice/internal/items"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/stats"
	"github.com/EraseMC/GoPractice/internal/toggles"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/EraseMC/GoPractice/internal/worlds"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/google/uuid"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

const (
	CombatTime   = 15
	RespawnDelay = 2
	dayTime      = 1000
)

type Deps struct {
	Log     *slog.Logger
	Lang    *lang.Bundle
	Players *state.Players
	Forms   *ui.Forms
	Boards  *ui.Scoreboards
	Chat    *chat.Manager
	Toggles *toggles.Manager
	Stats   *stats.Manager
	Combat  *combat.Manager
	Items   *items.Manager
	Hub     *hub.Manager
}

type foe struct {
	id   uuid.UUID
	name string
}

type Manager struct {
	Deps

	RemoveCapybara func(o *state.Online, p *player.Player, tx *world.Tx)

	arenas []*Arena
	byName map[string]*Arena

	mu         sync.Mutex
	playing    map[uuid.UUID]*Arena
	enemy      map[uuid.UUID]foe
	combatTime map[uuid.UUID]int
	respawn    map[uuid.UUID]int
	hidden     map[uuid.UUID]map[uuid.UUID]struct{}
}

func New(d Deps) *Manager {
	m := &Manager{
		Deps:       d,
		arenas:     Arenas(),
		byName:     map[string]*Arena{},
		playing:    map[uuid.UUID]*Arena{},
		enemy:      map[uuid.UUID]foe{},
		combatTime: map[uuid.UUID]int{},
		respawn:    map[uuid.UUID]int{},
		hidden:     map[uuid.UUID]map[uuid.UUID]struct{}{},
	}
	for _, a := range m.arenas {
		m.byName[a.Folder] = a
	}
	return m
}

func (m *Manager) Load(dir string) error {
	for _, a := range m.arenas {
		snap, err := worlds.LoadSnapshot(m.Log, filepath.Join(dir, a.Folder))
		if err != nil {
			return fmt.Errorf("load arena %s: %w", a.Folder, err)
		}
		w := snap.New(m.Log)
		w.SetTime(dayTime)
		w.StopTime()
		a.world = w
		a.spawn = w.Spawn().Vec3()
	}
	worlds.OnLeave(m.onLeave)
	worlds.OnEnter(m.onEnter)
	return nil
}

func (m *Manager) Close() {
	for _, a := range m.arenas {
		if a.world != nil {
			_ = a.world.Close()
		}
	}
}

func (m *Manager) List() []*Arena {
	return m.arenas
}

func (m *Manager) Arena(o *state.Online) *Arena {
	if o == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.playing[o.UUID]
}

func (m *Manager) InFFA(o *state.Online) bool {
	return m.Arena(o) != nil
}

func (m *Manager) InArena(o *state.Online, k Kind) bool {
	a := m.Arena(o)
	return a != nil && a.Kind == k
}

func (m *Manager) Online(a *Arena) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, x := range m.playing {
		if x == a {
			n++
		}
	}
	return n
}

func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.playing)
}

func (m *Manager) arenaPlayers(a *Arena) []uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []uuid.UUID
	for id, x := range m.playing {
		if x == a {
			out = append(out, id)
		}
	}
	return out
}

func (m *Manager) InCombat(o *state.Online) bool {
	if o == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.enemy[o.UUID]
	return ok
}

func (m *Manager) EnemyName(o *state.Online) string {
	if o == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enemy[o.UUID].name
}

func (m *Manager) foeOf(o *state.Online) (foe, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.enemy[o.UUID]
	return f, ok
}

func (m *Manager) Enemy(o *state.Online) (*state.Online, bool) {
	f, ok := m.foeOf(o)
	if !ok {
		return nil, false
	}
	return m.Players.Get(f.id)
}

func (m *Manager) CombatTime(o *state.Online) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.enemy[o.UUID]; !ok {
		return 0
	}
	return m.combatTime[o.UUID]
}

func (m *Manager) HasRespawnDelay(o *state.Online) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.respawn[o.UUID]
	return ok
}

func (m *Manager) Profile(o *state.Online, _ *player.Player) combat.Profile {
	if a := m.Arena(o); a != nil {
		return a.Profile
	}
	return combat.Default
}

func (m *Manager) text(o *state.Online, key string, args ...any) string {
	return m.Lang.Textf(o.Lang(), key, args...)
}

func with(tx *world.Tx, o *state.Online, f func(tx *world.Tx, p *player.Player)) {
	if tx != nil {
		if e, ok := o.Handle.Entity(tx); ok {
			if p, ok := e.(*player.Player); ok {
				f(tx, p)
				return
			}
		}
	}
	o.Do(f)
}

func entityOf(tx *world.Tx, o *state.Online) (*player.Player, bool) {
	e, ok := o.Handle.Entity(tx)
	if !ok {
		return nil, false
	}
	p, ok := e.(*player.Player)
	return p, ok
}

type fullHealth struct{}

func (fullHealth) HealingSource() {}

func setHealth(p *player.Player, max float64) {
	p.SetMaxHealth(max)
	if missing := p.MaxHealth() - p.Health(); missing > 0 {
		p.Heal(missing, fullHealth{})
	}
}

func clearEffects(p *player.Player) {
	for _, e := range p.Effects() {
		p.RemoveEffect(e.Type())
	}
}

func (m *Manager) SetPlayerSettings(p *player.Player) {
	p.SetGameMode(world.GameModeAdventure)
	p.StopFlying()
	setHealth(p, 20)
	p.SetFood(20)
	p.Inventory().Clear()
	p.Armour().Clear()
	_, off := p.HeldItems()
	p.SetHeldItems(item.Stack{}, off)
	p.Extinguish()
	clearEffects(p)
}

func (m *Manager) JoinArena(o *state.Online, p *player.Player, tx *world.Tx, a *Arena) {
	o.SetPlace(state.PlaceFFA, a.Folder)
	if m.RemoveCapybara != nil {
		m.RemoveCapybara(o, p, tx)
	}
	m.SetPlayerSettings(p)
	a.GiveKit(p)
	m.mu.Lock()
	m.playing[o.UUID] = a
	m.mu.Unlock()
	rot := p.Rotation()
	worlds.MoveThen(tx, p, a.world, a.spawn, rot.Yaw(), rot.Pitch(), func(tx *world.Tx, p *player.Player) {
		p.SendPacket(combat.LevelEventPacket(p.Position(), packet.LevelEventSoundEndermanTeleport))
		m.UpdateScoreboard(o)
		m.UpdateNameTag(o, p)
	})
}

func (m *Manager) removeFromArena(o *state.Online, p *player.Player) {
	m.mu.Lock()
	a, ok := m.playing[o.UUID]
	delete(m.playing, o.UUID)
	m.mu.Unlock()
	if ok && a.Kind == NoDebuff && p != nil {
		m.Items.ResetPearlCooldown(p)
	}
}

func (m *Manager) RemoveFromFFA(o *state.Online, p *player.Player, tx *world.Tx) {
	m.removeFromArena(o, p)
	if m.InCombat(o) {
		m.showPlayers(tx, o, p)
		m.removeCombatStatus(tx, o, p)
	}
	m.Hub.Reset(o, p, tx)
}

func (m *Manager) ToSpawn(o *state.Online, p *player.Player, tx *world.Tx) {
	m.SetPlayerSettings(p)
	m.RemoveFromFFA(o, p, tx)
}

func (m *Manager) JoinArenaAgain(o *state.Online, p *player.Player, tx *world.Tx) {
	m.SetPlayerSettings(p)
	if m.InCombat(o) {
		m.showPlayers(tx, o, p)
		m.removeCombatStatus(tx, o, p)
	}
	a := m.Arena(o)
	if a == nil {
		return
	}
	m.removeFromArena(o, p)
	m.JoinArena(o, p, tx, a)
}

func (m *Manager) ReKit(o *state.Online, p *player.Player) {
	a := m.Arena(o)
	if a == nil {
		return
	}
	if a.Kind == NoDebuff {
		m.Items.ResetPearlCooldown(p)
	}
	m.SetPlayerSettings(p)
	a.GiveKit(p)
}

func (m *Manager) setSpectator(o *state.Online, p *player.Player, pos mgl64.Vec3) {
	setHealth(p, p.MaxHealth())
	p.SetGameMode(world.GameModeSpectator)
	p.Inventory().Clear()
	p.Armour().Clear()
	_, off := p.HeldItems()
	p.SetHeldItems(item.Stack{}, off)
	clearEffects(p)
	p.SetExperienceLevel(0)
	p.SetExperienceProgress(0)
	p.Teleport(pos.Add(mgl64.Vec3{0, 2, 0}))
	m.mu.Lock()
	m.respawn[o.UUID] = RespawnDelay
	m.mu.Unlock()
}

func (m *Manager) setCombatStatus(tx *world.Tx, a *state.Online, ap *player.Player, b *state.Online, bp *player.Player) {
	m.mu.Lock()
	m.enemy[a.UUID] = foe{id: b.UUID, name: b.Name}
	m.enemy[b.UUID] = foe{id: a.UUID, name: a.Name}
	m.combatTime[a.UUID] = CombatTime
	m.combatTime[b.UUID] = CombatTime
	m.mu.Unlock()
	m.UpdateScoreboard(a)
	m.UpdateScoreboard(b)
	m.UpdateNameTag(a, ap)
	m.UpdateNameTag(b, bp)
}

func (m *Manager) updateCombatTime(a, b *state.Online) {
	m.mu.Lock()
	m.combatTime[a.UUID] = CombatTime
	m.combatTime[b.UUID] = CombatTime
	m.mu.Unlock()
}

func (m *Manager) removeCombatStatus(tx *world.Tx, o *state.Online, p *player.Player) {
	m.removeCombat(o.UUID)
	m.UpdateScoreboard(o)
	if p != nil {
		m.UpdateNameTag(o, p)
	} else {
		with(tx, o, func(_ *world.Tx, p *player.Player) { m.UpdateNameTag(o, p) })
	}
}

func (m *Manager) removeCombat(id uuid.UUID) {
	m.mu.Lock()
	delete(m.enemy, id)
	delete(m.combatTime, id)
	m.mu.Unlock()
}

func (m *Manager) hidePlayers(tx *world.Tx, o *state.Online, p *player.Player) {
	if !m.InCombat(o) || !m.Toggles.Get(o, toggles.HidePlayers) {
		return
	}
	enemy := m.EnemyName(o)
	for e := range tx.Players() {
		t, ok := e.(*player.Player)
		if !ok {
			continue
		}
		to, ok := m.Players.Get(t.UUID())
		if !ok || to.Name == o.Name || to.Name == enemy {
			continue
		}
		m.hide(o, p, t)
	}
}

func (m *Manager) hide(o *state.Online, p, t *player.Player) {
	p.HideEntity(t)
	m.mu.Lock()
	set, ok := m.hidden[o.UUID]
	if !ok {
		set = map[uuid.UUID]struct{}{}
		m.hidden[o.UUID] = set
	}
	set[t.UUID()] = struct{}{}
	m.mu.Unlock()
}

func (m *Manager) showPlayers(tx *world.Tx, o *state.Online, p *player.Player) {
	m.mu.Lock()
	set := m.hidden[o.UUID]
	delete(m.hidden, o.UUID)
	m.mu.Unlock()
	for id := range set {
		t, ok := m.Players.Get(id)
		if !ok {
			continue
		}
		if tp, ok := entityOf(tx, t); ok {
			p.ShowEntity(tp)
		}
	}
}

func (m *Manager) onLeave(tx *world.Tx, p *player.Player) {
	id := p.UUID()
	if o, ok := m.Players.Get(id); ok {
		m.showPlayers(tx, o, p)
	}
	m.mu.Lock()
	var hiders []uuid.UUID
	for h, set := range m.hidden {
		if _, ok := set[id]; ok {
			delete(set, id)
			hiders = append(hiders, h)
		}
	}
	m.mu.Unlock()
	for _, h := range hiders {
		ho, ok := m.Players.Get(h)
		if !ok {
			continue
		}
		if hp, ok := entityOf(tx, ho); ok {
			hp.ShowEntity(p)
		}
	}
}

func (m *Manager) onEnter(tx *world.Tx, p *player.Player) {
	t, ok := m.Players.Get(p.UUID())
	if !ok {
		return
	}
	for e := range tx.Players() {
		hp, ok := e.(*player.Player)
		if !ok || hp.UUID() == p.UUID() {
			continue
		}
		h, ok := m.Players.Get(hp.UUID())
		if !ok || !m.InCombat(h) || !m.Toggles.Get(h, toggles.HidePlayers) || m.EnemyName(h) == t.Name {
			continue
		}
		m.hide(h, hp, p)
	}
}

func (m *Manager) processKill(tx *world.Tx, dead *state.Online, deadP *player.Player, killer *state.Online, killerP *player.Player) {
	da, ka := m.Arena(dead), m.Arena(killer)
	statistics := true
	if da != nil && da == ka {
		args := da.killArgs(dead.Name, killer.Name, deadP, killerP)
		for _, id := range m.arenaPlayers(da) {
			ao, ok := m.Players.Get(id)
			if !ok {
				continue
			}
			ao.Message(m.text(ao, da.KillKey, args...))
		}
		m.SetPlayerSettings(killerP)
		da.GiveKit(killerP)
		statistics = da.Stats
	}
	if statistics {
		m.Stats.AddKill(killer, 1)
		m.Stats.AddDeath(dead, 1)
	}
	m.UpdateNameTag(killer, killerP)
}

type killOptions struct {
	animation bool
	cooldowns bool
}

func (m *Manager) kill(tx *world.Tx, dead *state.Online, deadP *player.Player, killer *state.Online, killerP *player.Player, opt killOptions) {
	m.showPlayers(tx, dead, deadP)
	m.showPlayers(tx, killer, killerP)
	m.removeCombatStatus(tx, dead, deadP)
	m.removeCombatStatus(tx, killer, killerP)
	if opt.animation {
		m.Combat.DeathAnimation(tx, deadP, killerP.Position())
	}
	m.processKill(tx, dead, deadP, killer, killerP)
	m.setSpectator(dead, deadP, killerP.Position())
	if opt.cooldowns {
		m.Items.ResetPearlCooldown(deadP)
		m.Items.ResetPearlCooldown(killerP)
	}
	t := m.Lang.Lines(dead.Lang(), "ffaDeathTitle")
	ui.Title(deadP, t[0], lang.Sprintf(t[1], killer.Name))
	ui.ActionBar(killerP, m.text(killer, "ffaCombatReduced"))
}

func (m *Manager) Surrender(o *state.Online, p *player.Player, tx *world.Tx) bool {
	if !m.InCombat(o) {
		return false
	}
	enemy, ok := m.Enemy(o)
	if !ok {
		return false
	}
	ep, ok := entityOf(tx, enemy)
	if !ok {
		return false
	}
	m.kill(tx, o, p, enemy, ep, killOptions{animation: true, cooldowns: true})
	return true
}

func (m *Manager) enemyPing(o *state.Online) (int64, bool) {
	if !m.InCombat(o) {
		return 0, true
	}
	enemy, ok := m.Enemy(o)
	if !ok {
		return 0, false
	}
	return enemy.Ping(), true
}

func (m *Manager) Mode(o *state.Online) string {
	if a := m.Arena(o); a != nil {
		return a.Display
	}
	return "§8None"
}

func (m *Manager) arenaOnline(o *state.Online) int {
	if a := m.Arena(o); a != nil {
		return m.Online(a)
	}
	return 0
}

func (m *Manager) UpdateScoreboard(o *state.Online) {
	if !m.Toggles.Get(o, toggles.Scoreboard) {
		return
	}
	l := o.Lang()
	if m.InCombat(o) {
		ping, ok := m.enemyPing(o)
		if !ok {
			m.removeCombatStatus(nil, o, nil)
			return
		}
		lines := m.Lang.Lines(l, "ffaScoreboardCombat")
		m.Boards.Set(o.UUID, lines[0])
		m.Boards.SetLine(o.UUID, 1, lines[1])
		m.Boards.SetLine(o.UUID, 2, lang.Sprintf(lines[2], o.Ping()))
		m.Boards.SetLine(o.UUID, 3, lang.Sprintf(lines[3], ping))
		m.Boards.SetLine(o.UUID, 4, lines[4])
		m.Boards.SetLine(o.UUID, 5, lang.Sprintf(lines[5], m.CombatTime(o)))
		m.Boards.SetLine(o.UUID, 6, lines[6])
		m.Boards.SetLine(o.UUID, 7, lines[7])
		return
	}
	lines := m.Lang.Lines(l, "ffaScoreboardIdle")
	m.Boards.Set(o.UUID, lines[0])
	m.Boards.SetLine(o.UUID, 1, lines[1])
	m.Boards.SetLine(o.UUID, 2, lang.Sprintf(lines[2], o.Ping()))
	m.Boards.SetLine(o.UUID, 3, lang.Sprintf(lines[3], m.arenaOnline(o)))
	m.Boards.SetLine(o.UUID, 4, lines[4])
	m.Boards.SetLine(o.UUID, 5, lang.Sprintf(lines[5], m.Mode(o)))
	m.Boards.SetLine(o.UUID, 6, lines[6])
	m.Boards.SetLine(o.UUID, 7, lines[7])
}

func (m *Manager) UpdateNameTag(o *state.Online, p *player.Player) {
	p.SetNameTag(m.Chat.NameTag(o))
	if a := m.Arena(o); a != nil && a.NoScore {
		p.SetScoreTag("")
		return
	}
	p.SetScoreTag(fmt.Sprintf("§f%d", health(p)))
}

func (m *Manager) Tick() {
	for _, o := range m.Players.All() {
		if !m.InFFA(o) {
			continue
		}
		o.Do(func(tx *world.Tx, p *player.Player) {
			if !m.InFFA(o) {
				return
			}
			m.UpdateScoreboard(o)
			if !m.InCombat(o) {
				return
			}
			if m.CombatTime(o) <= 0 {
				m.showPlayers(tx, o, p)
				m.removeCombatStatus(tx, o, p)
				ui.ActionBar(p, m.text(o, "ffaCombatReduced"))
				return
			}
			m.mu.Lock()
			m.combatTime[o.UUID]--
			m.mu.Unlock()
		})
	}
}

func (m *Manager) RespawnTick() {
	for _, o := range m.Players.All() {
		if !m.InFFA(o) || !m.HasRespawnDelay(o) {
			continue
		}
		o.Do(func(tx *world.Tx, p *player.Player) {
			if !m.InFFA(o) {
				return
			}
			m.mu.Lock()
			left, ok := m.respawn[o.UUID]
			if ok && left > 0 {
				m.respawn[o.UUID] = left - 1
			} else {
				delete(m.respawn, o.UUID)
			}
			m.mu.Unlock()
			if !ok || left > 0 {
				return
			}
			if m.Toggles.Get(o, toggles.ArenaRespawn) {
				m.JoinArenaAgain(o, p, tx)
			} else {
				m.ToSpawn(o, p, tx)
			}
		})
	}
}

func (m *Manager) SendForm(o *state.Online, p *player.Player) {
	form := ui.NewSimple(func(p *player.Player, tx *world.Tx, data *int) {
		if data == nil || *data < 0 || *data >= len(m.arenas) {
			return
		}
		if !m.Hub.InHub(o) {
			return
		}
		a := m.arenas[*data]
		m.JoinArena(o, p, tx, a)
		o.Message(m.text(o, "joinedFFA", a.Name))
	})
	contents := m.Lang.Lines(o.Lang(), "ffaForm")
	form.SetTitle(contents[0])
	for i, a := range m.arenas {
		form.AddButton(lang.Sprintf(contents[i+1], m.Online(a)), ui.ImagePath, a.Icon)
	}
	m.Forms.Send(p, form)
}
