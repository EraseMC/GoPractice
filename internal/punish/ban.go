package punish

import (
	"strings"
	"time"

	"github.com/EraseMC/GoPractice/internal/lang"
	"github.com/EraseMC/GoPractice/internal/store"
)

type banMatch int

const (
	banNone banMatch = iota
	banNickname
	banXUID
	banDevice
)

func (m *Manager) findBan(nickname, xuid, deviceID string) (store.Ban, banMatch) {
	p := m.store.Punishments
	if b, ok := p.BanByNickname(nickname); ok {
		return b, banNickname
	}
	if b, ok := p.BanByXUID(xuid); ok {
		return b, banXUID
	}
	if b, ok := p.BanByDeviceID(deviceID); ok {
		return b, banDevice
	}
	return store.Ban{}, banNone
}

func (m *Manager) removeBan(nickname, xuid, deviceID string, match banMatch) {
	p := m.store.Punishments
	switch match {
	case banXUID:
		p.RemoveBansByXUID(xuid)
	case banDevice:
		p.RemoveBansByDeviceID(deviceID)
	default:
		p.RemoveBans(nickname)
	}
}

func (m *Manager) BanKick(name, xuid, deviceID string, locale func() lang.Lang) (string, bool) {
	nickname := strings.ToLower(name)
	if xuid == "" {
		xuid = "0"
	}
	b, match := m.findBan(nickname, xuid, deviceID)
	if match == banNone {
		return "", false
	}
	if b.Expiry != 0 && time.Now().Unix() >= b.Expiry {
		m.removeBan(nickname, xuid, deviceID, match)
		return "", false
	}

	p := m.store.Punishments
	bannedNickname := "Steve"
	if bx, ok := p.BanByXUID(xuid); ok {
		bannedNickname = bx.Nickname
	}
	bannedXUID := "0"
	if bd, ok := p.BanByDeviceID(deviceID); ok {
		bannedXUID = bd.XUID
	}
	if bannedNickname != name && bannedXUID == xuid {
		if _, ok := p.BanByXUID(xuid); ok {
			p.RenameBans(xuid, nickname)
		}
	}

	b, _ = m.findBan(nickname, xuid, deviceID)
	l := locale()
	expiry := m.bundle.In(l, lang.Ban, "banNeverExpires")
	if b.Expiry != 0 {
		d, h, i := Remaining(b.Expiry - time.Now().Unix())
		expiry = m.bundle.Inf(l, lang.Ban, "banExpiry", d, h, i)
	}
	return m.bundle.Inf(l, lang.Ban, "banKick", b.Nickname, time.Unix(b.When, 0).Format("02.01.06"), b.Reason, expiry), true
}
