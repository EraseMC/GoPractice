package rank

const (
	ChannelAdministrative = "practice.channel.administrative"
	ChannelOwners         = "practice.channel.owners"

	CmdBan        = "practice.command.ban"
	CmdClearChat  = "practice.command.clearchat"
	CmdCrash      = "practice.command.crash"
	CmdDuel       = "practice.command.duel"
	CmdEmoji      = "practice.command.emoji"
	CmdGameMode   = "practice.command.gamemode"
	CmdInfo       = "practice.command.info"
	CmdKick       = "practice.command.kick"
	CmdMute       = "practice.command.mute"
	CmdPing       = "practice.command.ping"
	CmdPlatform   = "practice.command.platform"
	CmdProfile    = "practice.command.profile"
	CmdRank       = "practice.command.rank"
	CmdReconnect  = "practice.command.reconnect"
	CmdReKit      = "practice.command.rekit"
	CmdReply      = "practice.command.reply"
	CmdReport     = "practice.command.report"
	CmdRestart    = "practice.command.restart"
	CmdRules      = "practice.command.rules"
	CmdSay        = "practice.command.say"
	CmdSettings   = "practice.command.settings"
	CmdSneakPeek  = "practice.command.sneakpeek"
	CmdSpawn      = "practice.command.spawn"
	CmdSpectate   = "practice.command.spectate"
	CmdStatus     = "practice.command.status"
	CmdSurrender  = "practice.command.surrender"
	CmdTeleport   = "practice.command.teleport"
	CmdTell       = "practice.command.tell"
	CmdToggleChat = "practice.command.togglechat"
	CmdUnban      = "practice.command.unban"
	CmdUnmute     = "practice.command.unmute"
	CmdStop       = "pocketmine.command.stop"

	CosmeticsDefault      = "practice.cosmetics.default"
	CosmeticsCapes        = "practice.cosmetics.capes"
	CosmeticsCapybara     = "practice.cosmetics.capybara"
	CosmeticsTagsSilver   = "practice.cosmetics.tags.silver"
	CosmeticsTagsGolden   = "practice.cosmetics.tags.golden"
	CosmeticsTagsPlatinum = "practice.cosmetics.tags.platinum"
	CosmeticsFly          = "practice.cosmetics.fly"
	CosmeticsPotionColor  = "practice.cosmetics.potioncolor"
)

var everyone = map[string]bool{
	CmdDuel:      true,
	CmdEmoji:     true,
	CmdInfo:      true,
	CmdPing:      true,
	CmdPlatform:  true,
	CmdReconnect: true,
	CmdReKit:     true,
	CmdReply:     true,
	CmdReport:    true,
	CmdRules:     true,
	CmdSettings:  true,
	CmdSpawn:     true,
	CmdSurrender: true,
	CmdTell:      true,
}

var wayner = []string{
	CosmeticsDefault,
	CosmeticsTagsSilver,
	CosmeticsFly,
}

var mystery = []string{
	CosmeticsDefault,
	CosmeticsCapes,
	CosmeticsTagsSilver,
	CosmeticsTagsGolden,
	CosmeticsFly,
}

var gangster = []string{
	CosmeticsDefault,
	CosmeticsCapes,
	CosmeticsCapybara,
	CosmeticsTagsSilver,
	CosmeticsTagsGolden,
	CosmeticsTagsPlatinum,
	CosmeticsFly,
	CosmeticsPotionColor,
}

var famousPlus = []string{
	CmdBan,
	CmdKick,
	CosmeticsDefault,
	CosmeticsCapes,
	CosmeticsCapybara,
	CosmeticsTagsSilver,
	CosmeticsTagsGolden,
	CosmeticsTagsPlatinum,
	CosmeticsFly,
	CosmeticsPotionColor,
}

var moderator = []string{
	ChannelAdministrative,
	CmdBan,
	CmdKick,
	CmdMute,
	CmdSpectate,
	CosmeticsDefault,
	CosmeticsCapes,
	CosmeticsCapybara,
	CosmeticsTagsSilver,
	CosmeticsTagsGolden,
	CosmeticsTagsPlatinum,
	CosmeticsFly,
	CosmeticsPotionColor,
}

var admin = []string{
	ChannelAdministrative,
	CmdBan,
	CmdClearChat,
	CmdKick,
	CmdMute,
	CmdSpectate,
	CmdUnban,
	CmdUnmute,
	CosmeticsDefault,
	CosmeticsCapes,
	CosmeticsCapybara,
	CosmeticsTagsSilver,
	CosmeticsTagsGolden,
	CosmeticsTagsPlatinum,
	CosmeticsFly,
	CosmeticsPotionColor,
}

var builder = []string{
	ChannelAdministrative,
	CmdBan,
	CmdClearChat,
	CmdKick,
	CmdMute,
	CmdProfile,
	CmdSpectate,
	CmdUnban,
	CmdUnmute,
	CosmeticsDefault,
	CosmeticsCapes,
	CosmeticsCapybara,
	CosmeticsTagsSilver,
	CosmeticsTagsGolden,
	CosmeticsTagsPlatinum,
	CosmeticsFly,
	CosmeticsPotionColor,
}

var manager = []string{
	CmdTeleport,
	ChannelAdministrative,
	ChannelOwners,
	CmdBan,
	CmdClearChat,
	CmdGameMode,
	CmdKick,
	CmdMute,
	CmdProfile,
	CmdRank,
	CmdSpectate,
	CmdToggleChat,
	CmdUnban,
	CmdUnmute,
	CosmeticsDefault,
	CosmeticsCapes,
	CosmeticsCapybara,
	CosmeticsTagsSilver,
	CosmeticsTagsGolden,
	CosmeticsTagsPlatinum,
	CosmeticsFly,
	CosmeticsPotionColor,
}

var developer = []string{
	CmdTeleport,
	ChannelAdministrative,
	ChannelOwners,
	CmdSneakPeek,
	CmdBan,
	CmdClearChat,
	CmdGameMode,
	CmdKick,
	CmdMute,
	CmdProfile,
	CmdRank,
	CmdSpectate,
	CmdStatus,
	CmdToggleChat,
	CmdUnban,
	CmdUnmute,
	CosmeticsDefault,
	CosmeticsCapes,
	CosmeticsCapybara,
	CosmeticsTagsSilver,
	CosmeticsTagsGolden,
	CosmeticsTagsPlatinum,
	CosmeticsFly,
	CosmeticsPotionColor,
}

var owner = []string{
	CmdTeleport,
	ChannelAdministrative,
	ChannelOwners,
	CmdBan,
	CmdClearChat,
	CmdCrash,
	CmdGameMode,
	CmdKick,
	CmdMute,
	CmdProfile,
	CmdRank,
	CmdSpectate,
	CmdStatus,
	CmdToggleChat,
	CmdUnban,
	CmdUnmute,
	CosmeticsDefault,
	CosmeticsCapes,
	CosmeticsCapybara,
	CosmeticsTagsSilver,
	CosmeticsTagsGolden,
	CosmeticsTagsPlatinum,
	CosmeticsFly,
	CosmeticsPotionColor,
}
