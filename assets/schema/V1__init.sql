CREATE TABLE IF NOT EXISTS players (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    xuid VARCHAR(20) NOT NULL,
    name VARCHAR(32) NOT NULL,
    ip VARCHAR(45) NOT NULL DEFAULT '',
    device_id VARCHAR(64) NOT NULL DEFAULT '',
    player_rank VARCHAR(32) NOT NULL DEFAULT 'Player',
    language CHAR(2) NOT NULL DEFAULT 'en',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_players_xuid (xuid),
    KEY idx_players_name (name),
    KEY idx_players_ip (ip),
    KEY idx_players_device_id (device_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS statistics (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    player_id INT UNSIGNED NOT NULL,
    kills INT UNSIGNED NOT NULL DEFAULT 0,
    deaths INT UNSIGNED NOT NULL DEFAULT 0,
    play_time INT UNSIGNED NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_statistics_player (player_id),
    KEY idx_statistics_kills (kills),
    KEY idx_statistics_play_time (play_time),
    CONSTRAINT fk_statistics_player FOREIGN KEY (player_id) REFERENCES players (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS settings (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    player_id INT UNSIGNED NOT NULL,
    scoreboard TINYINT(1) NOT NULL DEFAULT 1,
    cps TINYINT(1) NOT NULL DEFAULT 0,
    autosprint TINYINT(1) NOT NULL DEFAULT 0,
    chat TINYINT(1) NOT NULL DEFAULT 1,
    hide_players TINYINT(1) NOT NULL DEFAULT 1,
    arena_respawn TINYINT(1) NOT NULL DEFAULT 0,
    lightning TINYINT(1) NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_settings_player (player_id),
    CONSTRAINT fk_settings_player FOREIGN KEY (player_id) REFERENCES players (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cosmetics (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    player_id INT UNSIGNED NOT NULL,
    cape VARCHAR(32) NULL DEFAULT NULL,
    tag VARCHAR(64) NULL DEFAULT NULL,
    fly TINYINT(1) NOT NULL DEFAULT 0,
    potion_color VARCHAR(16) NULL DEFAULT NULL,
    capybara TINYINT(1) NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uq_cosmetics_player (player_id),
    CONSTRAINT fk_cosmetics_player FOREIGN KEY (player_id) REFERENCES players (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS bans (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    nickname VARCHAR(32) NOT NULL,
    xuid VARCHAR(20) NOT NULL,
    device_id VARCHAR(64) NOT NULL DEFAULT '',
    banned_by VARCHAR(32) NOT NULL,
    ban_expiry INT UNSIGNED NOT NULL DEFAULT 0,
    banned_when INT UNSIGNED NOT NULL,
    ban_reason VARCHAR(255) NOT NULL DEFAULT '-',
    PRIMARY KEY (id),
    KEY idx_bans_nickname (nickname),
    KEY idx_bans_xuid (xuid),
    KEY idx_bans_device_id (device_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS mutes (
    id INT UNSIGNED NOT NULL AUTO_INCREMENT,
    nickname VARCHAR(32) NOT NULL,
    xuid VARCHAR(20) NOT NULL,
    muted_by VARCHAR(32) NOT NULL,
    mute_expiry INT UNSIGNED NOT NULL DEFAULT 0,
    muted_when INT UNSIGNED NOT NULL,
    mute_reason VARCHAR(255) NOT NULL DEFAULT '-',
    PRIMARY KEY (id),
    KEY idx_mutes_nickname (nickname),
    KEY idx_mutes_xuid (xuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
