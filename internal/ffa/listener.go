package ffa

import (
	"time"

	"github.com/EraseMC/GoPractice/internal/gamemode"
	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/state"
	"github.com/EraseMC/GoPractice/internal/ui"
	"github.com/df-mc/dragonfly/server/entity"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/df-mc/dragonfly/server/player"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
)

func (m *Manager) Handler() player.Handler {
	return &listener{m: m}
}

type listener struct {
	player.NopHandler
	m *Manager
}

func (l *listener) online(p *player.Player) (*state.Online, bool) {
	return l.m.Players.Get(p.UUID())
}

func (l *listener) HandleHurt(ctx *player.Context, damage *float64, _ bool, _ *time.Duration, src world.DamageSource) {
	p := ctx.Player()
	o, ok := l.online(p)
	if !ok || !l.m.InFFA(o) {
		return
	}
	tx := p.Tx()
	m := l.m
	switch s := src.(type) {
	case entity.AttackDamageSource:
		d, ok := s.Attacker.(*player.Player)
		if !ok {
			break
		}
		do, ok := l.online(d)
		if !ok || !m.InFFA(do) {
			break
		}
		if do.UUID == o.UUID {
			ctx.Cancel()
			return
		}
		if m.InCombat(do) && m.EnemyName(do) != o.Name {
			ui.ActionBar(d, m.text(do, "ffaSelfInCombat", m.EnemyName(do)))
			ctx.Cancel()
			return
		}
		if m.InCombat(o) && m.EnemyName(o) != do.Name {
			ui.ActionBar(d, m.text(do, "ffaPlayerInCombat", o.Name, m.EnemyName(o)))
			ctx.Cancel()
			return
		}
		if !(m.InCombat(do) && m.InCombat(o)) {
			ui.ActionBar(d, m.text(do, "ffaYouAreNowInCombat", o.Name))
			ui.ActionBar(p, m.text(o, "ffaYouAreNowInCombat", do.Name))
			m.setCombatStatus(tx, do, d, o, p)
			m.hidePlayers(tx, do, d)
			m.hidePlayers(tx, o, p)
			m.UpdateNameTag(o, p)
			return
		}
		if p.Health()-max(0, *damage-p.Absorption()) <= 0 {
			m.kill(tx, o, p, do, d, killOptions{animation: true, cooldowns: true})
			ctx.Cancel()
			return
		}
		m.updateCombatTime(do, o)
		m.UpdateNameTag(o, p)
		return
	case entity.VoidDamageSource:
		ctx.Cancel()
		if m.InCombat(o) {
			enemy, ok := m.Enemy(o)
			if !ok {
				return
			}
			ep, ok := entityOf(tx, enemy)
			if !ok {
				return
			}
			m.kill(tx, o, p, enemy, ep, killOptions{})
			return
		}
		m.setSpectator(o, p, m.arenaSpawn(o, p))
		t := m.Lang.Lines(o.Lang(), "ffaDeathTitle")
		ui.Title(p, t[0], lang.Sprintf(t[1], "..."))
		return
	}
	if _, attack := src.(entity.AttackDamageSource); !attack {
		ctx.Cancel()
		return
	}
	m.UpdateNameTag(o, p)
}

func (m *Manager) arenaSpawn(o *state.Online, p *player.Player) mgl64.Vec3 {
	if a := m.Arena(o); a != nil {
		return a.spawn
	}
	return p.Position()
}

func (l *listener) HandleDeath(p *player.Player, src world.DamageSource, keepInv *bool) {
	o, ok := l.online(p)
	if !ok || !l.m.InFFA(o) {
		return
	}
	*keepInv = true
	s, ok := src.(entity.AttackDamageSource)
	if !ok {
		return
	}
	d, ok := s.Attacker.(*player.Player)
	if !ok {
		return
	}
	do, ok := l.online(d)
	if !ok {
		return
	}
	m, tx := l.m, p.Tx()
	m.showPlayers(tx, o, p)
	m.showPlayers(tx, do, d)
	m.removeCombatStatus(tx, o, p)
	m.removeCombatStatus(tx, do, d)
	m.Items.ResetPearlCooldown(p)
	m.Items.ResetPearlCooldown(d)
	ui.ActionBar(d, m.text(do, "ffaCombatReduced"))
	m.processKill(tx, o, p, do, d)
}

func (l *listener) HandleRespawn(p *player.Player, pos *mgl64.Vec3, w **world.World) {
	o, ok := l.online(p)
	if !ok || !l.m.InFFA(o) {
		return
	}
	*pos, *w = l.m.Hub.Spawn(), l.m.Hub.World()
	p.H().Do(func(tx *world.Tx, e world.Entity) {
		if p, ok := e.(*player.Player); ok && l.m.InFFA(o) {
			l.m.ToSpawn(o, p, tx)
		}
	})
}

func (l *listener) HandleItemUse(ctx *player.Context) {
	p := ctx.Player()
	o, ok := l.online(p)
	if !ok || !l.m.InFFA(o) {
		return
	}
	held, _ := p.HeldItems()
	switch held.Item().(type) {
	case item.Consumable, item.SplashPotion, item.EnderPearl:
	default:
		ctx.Cancel()
	}
	if !gamemode.IsAdventure(p.GameMode()) {
		ctx.Cancel()
	}
}

func (l *listener) HandleQuit(p *player.Player) {
	m := l.m
	tx := p.Tx()
	o, ok := l.online(p)
	if !ok {
		m.onLeave(tx, p)
		return
	}
	if m.InFFA(o) {
		m.mu.Lock()
		delete(m.respawn, o.UUID)
		m.mu.Unlock()
		if m.InCombat(o) {
			f, _ := m.foeOf(o)
			enemy, ok := m.Enemy(o)
			var ep *player.Player
			if ok {
				ep, ok = entityOf(tx, enemy)
			}
			if ok {
				m.Items.ResetPearlCooldown(ep)
				m.showPlayers(tx, enemy, ep)
				m.removeCombatStatus(tx, enemy, ep)
				m.Combat.DeathAnimation(tx, p, ep.Position())
				m.processKill(tx, o, p, enemy, ep)
				ui.ActionBar(ep, m.text(enemy, "ffaCombatReduced"))
			} else {
				m.removeCombat(f.id)
			}
			m.showPlayers(tx, o, p)
			m.removeCombat(o.UUID)
		}
		m.removeFromArena(o, nil)
	}
	m.onLeave(tx, p)
}
