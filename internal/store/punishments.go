package store

import (
	"context"
	"database/sql"
	"sync"

	"github.com/EraseMC/GoPractice/internal/db"
)

const punishmentShard = "punishments"

type Ban struct {
	Nickname string
	XUID     string
	DeviceID string
	BannedBy string
	Expiry   int64
	When     int64
	Reason   string
}

type Mute struct {
	Nickname string
	XUID     string
	MutedBy  string
	Expiry   int64
	When     int64
	Reason   string
}

type Punishments struct {
	mu    sync.RWMutex
	pool  *db.Pool
	bans  []Ban
	mutes []Mute
}

func loadPunishments(ctx context.Context, d *sql.DB) (*Punishments, error) {
	p := &Punishments{}
	rows, err := d.QueryContext(ctx, "SELECT nickname, xuid, device_id, banned_by, ban_expiry, banned_when, ban_reason FROM bans ORDER BY id")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var b Ban
		if err := rows.Scan(&b.Nickname, &b.XUID, &b.DeviceID, &b.BannedBy, &b.Expiry, &b.When, &b.Reason); err != nil {
			_ = rows.Close()
			return nil, err
		}
		p.bans = append(p.bans, b)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = d.QueryContext(ctx, "SELECT nickname, xuid, muted_by, mute_expiry, muted_when, mute_reason FROM mutes ORDER BY id")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var m Mute
		if err := rows.Scan(&m.Nickname, &m.XUID, &m.MutedBy, &m.Expiry, &m.When, &m.Reason); err != nil {
			_ = rows.Close()
			return nil, err
		}
		p.mutes = append(p.mutes, m)
	}
	_ = rows.Close()
	return p, rows.Err()
}

func (p *Punishments) attach(pool *db.Pool) {
	p.mu.Lock()
	p.pool = pool
	p.mu.Unlock()
}

func (p *Punishments) persist(sql string, args ...any) {
	if p.pool != nil {
		p.pool.Execute(db.Make(sql, args...), punishmentShard)
	}
}

func (p *Punishments) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.bans) + len(p.mutes)
}

func (p *Punishments) findBan(match func(Ban) bool) (Ban, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, b := range p.bans {
		if match(b) {
			return b, true
		}
	}
	return Ban{}, false
}

func (p *Punishments) BanByNickname(nickname string) (Ban, bool) {
	return p.findBan(func(b Ban) bool { return b.Nickname == nickname })
}

func (p *Punishments) BanByXUID(xuid string) (Ban, bool) {
	return p.findBan(func(b Ban) bool { return b.XUID == xuid })
}

func (p *Punishments) BanByDeviceID(deviceID string) (Ban, bool) {
	return p.findBan(func(b Ban) bool { return b.DeviceID == deviceID })
}

func (p *Punishments) AddBan(b Ban) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bans = append(p.bans, b)
	p.persist("INSERT INTO bans (nickname, xuid, device_id, banned_by, ban_expiry, banned_when, ban_reason) VALUES (?, ?, ?, ?, ?, ?, ?)",
		b.Nickname, b.XUID, b.DeviceID, b.BannedBy, b.Expiry, b.When, b.Reason)
}

func (p *Punishments) RemoveBans(nickname string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	kept := p.bans[:0]
	for _, b := range p.bans {
		if b.Nickname != nickname {
			kept = append(kept, b)
		}
	}
	p.bans = kept
	p.persist("DELETE FROM bans WHERE nickname = ?", nickname)
}

func (p *Punishments) RenameBans(xuid, nickname string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.bans {
		if p.bans[i].XUID == xuid {
			p.bans[i].Nickname = nickname
		}
	}
	p.persist("UPDATE bans SET nickname = ? WHERE xuid = ?", nickname, xuid)
}

func (p *Punishments) findMute(match func(Mute) bool) (Mute, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, m := range p.mutes {
		if match(m) {
			return m, true
		}
	}
	return Mute{}, false
}

func (p *Punishments) MuteByNickname(nickname string) (Mute, bool) {
	return p.findMute(func(m Mute) bool { return m.Nickname == nickname })
}

func (p *Punishments) MuteByXUID(xuid string) (Mute, bool) {
	return p.findMute(func(m Mute) bool { return m.XUID == xuid })
}

func (p *Punishments) AddMute(m Mute) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mutes = append(p.mutes, m)
	p.persist("INSERT INTO mutes (nickname, xuid, muted_by, mute_expiry, muted_when, mute_reason) VALUES (?, ?, ?, ?, ?, ?)",
		m.Nickname, m.XUID, m.MutedBy, m.Expiry, m.When, m.Reason)
}

func (p *Punishments) RemoveMutes(nickname string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	kept := p.mutes[:0]
	for _, m := range p.mutes {
		if m.Nickname != nickname {
			kept = append(kept, m)
		}
	}
	p.mutes = kept
	p.persist("DELETE FROM mutes WHERE nickname = ?", nickname)
}

func (p *Punishments) RemoveMutesByXUID(xuid string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	kept := p.mutes[:0]
	for _, m := range p.mutes {
		if m.XUID != xuid {
			kept = append(kept, m)
		}
	}
	p.mutes = kept
	p.persist("DELETE FROM mutes WHERE xuid = ?", xuid)
}

func (p *Punishments) RenameMutes(xuid, nickname string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.mutes {
		if p.mutes[i].XUID == xuid {
			p.mutes[i].Nickname = nickname
		}
	}
	p.persist("UPDATE mutes SET nickname = ? WHERE xuid = ?", nickname, xuid)
}
