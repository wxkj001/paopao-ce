CREATE TABLE IF NOT EXISTS p_pt_category (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL DEFAULT '',
    icon VARCHAR(50) NOT NULL DEFAULT '',
    sort INT NOT NULL DEFAULT 0
);

INSERT INTO p_pt_category (id, name, icon, sort) VALUES
(1, '电影', 'film-outline', 10),
(2, '电视剧', 'tv-outline', 20),
(3, '动漫', 'color-palette-outline', 30),
(4, '音乐', 'musical-notes-outline', 40),
(5, '游戏', 'game-controller-outline', 50),
(6, '应用软件', 'apps-outline', 60),
(7, '电子书/资料', 'book-outline', 70)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

CREATE TABLE IF NOT EXISTS p_torrent (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGINT NOT NULL DEFAULT 0,
    info_hash VARCHAR(40) NOT NULL DEFAULT '',
    name VARCHAR(255) NOT NULL DEFAULT '',
    clean_name VARCHAR(255) NOT NULL DEFAULT '',
    small_descr VARCHAR(255) NOT NULL DEFAULT '',
    descr TEXT,
    category_id INT NOT NULL DEFAULT 1,
    size BIGINT NOT NULL DEFAULT 0,
    num_files INT NOT NULL DEFAULT 1,
    user_id BIGINT NOT NULL DEFAULT 0,
    sp_state SMALLINT NOT NULL DEFAULT 1,
    promotion_until BIGINT NOT NULL DEFAULT 0,
    seeders INT NOT NULL DEFAULT 0,
    leechers INT NOT NULL DEFAULT 0,
    times_completed INT NOT NULL DEFAULT 0,
    hits INT NOT NULL DEFAULT 0,
    is_anonymous BOOLEAN NOT NULL DEFAULT FALSE,
    status SMALLINT NOT NULL DEFAULT 0,
    nfo TEXT,
    file_path VARCHAR(255) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ DEFAULT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_torrent_info_hash ON p_torrent (info_hash);
CREATE INDEX IF NOT EXISTS idx_torrent_post_id ON p_torrent (post_id);
CREATE INDEX IF NOT EXISTS idx_torrent_user_id ON p_torrent (user_id);
CREATE INDEX IF NOT EXISTS idx_torrent_category_id ON p_torrent (category_id);
CREATE INDEX IF NOT EXISTS idx_torrent_deleted_at ON p_torrent (deleted_at);

CREATE TABLE IF NOT EXISTS p_torrent_file (
    id BIGSERIAL PRIMARY KEY,
    torrent_id BIGINT NOT NULL DEFAULT 0,
    file_path VARCHAR(512) NOT NULL DEFAULT '',
    size BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_file_torrent_id ON p_torrent_file (torrent_id);

CREATE TABLE IF NOT EXISTS p_torrent_peer (
    id BIGSERIAL PRIMARY KEY,
    torrent_id BIGINT NOT NULL DEFAULT 0,
    user_id BIGINT NOT NULL DEFAULT 0,
    peer_id VARCHAR(40) NOT NULL DEFAULT '',
    ip VARCHAR(64) NOT NULL DEFAULT '',
    port INT NOT NULL DEFAULT 0,
    uploaded BIGINT NOT NULL DEFAULT 0,
    downloaded BIGINT NOT NULL DEFAULT 0,
    to_go BIGINT NOT NULL DEFAULT 0,
    is_seeder BOOLEAN NOT NULL DEFAULT FALSE,
    agent VARCHAR(100) NOT NULL DEFAULT '',
    last_action BIGINT NOT NULL DEFAULT 0,
    started BIGINT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_torrent_peer ON p_torrent_peer (torrent_id, peer_id);
CREATE INDEX IF NOT EXISTS idx_peer_user_id ON p_torrent_peer (user_id);
CREATE INDEX IF NOT EXISTS idx_peer_last_action ON p_torrent_peer (last_action);

CREATE TABLE IF NOT EXISTS p_torrent_snatched (
    id BIGSERIAL PRIMARY KEY,
    torrent_id BIGINT NOT NULL DEFAULT 0,
    user_id BIGINT NOT NULL DEFAULT 0,
    uploaded BIGINT NOT NULL DEFAULT 0,
    downloaded BIGINT NOT NULL DEFAULT 0,
    to_go BIGINT NOT NULL DEFAULT 0,
    seed_time BIGINT NOT NULL DEFAULT 0,
    leech_time BIGINT NOT NULL DEFAULT 0,
    is_finished BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at BIGINT NOT NULL DEFAULT 0,
    last_action BIGINT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_torrent_snatch ON p_torrent_snatched (torrent_id, user_id);
CREATE INDEX IF NOT EXISTS idx_snatch_user_id ON p_torrent_snatched (user_id);

CREATE TABLE IF NOT EXISTS p_user_pt_profile (
    user_id BIGINT PRIMARY KEY,
    passkey VARCHAR(32) NOT NULL DEFAULT '',
    uploaded BIGINT NOT NULL DEFAULT 0,
    downloaded BIGINT NOT NULL DEFAULT 0,
    seed_bonus NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    seed_time BIGINT NOT NULL DEFAULT 0,
    leech_time BIGINT NOT NULL DEFAULT 0,
    can_leech BOOLEAN NOT NULL DEFAULT TRUE,
    can_upload BOOLEAN NOT NULL DEFAULT TRUE,
    invites INT NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL DEFAULT 0,
    updated_at BIGINT NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_passkey ON p_user_pt_profile (passkey);

CREATE TABLE IF NOT EXISTS p_pt_bonus_log (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL DEFAULT 0,
    action_type VARCHAR(50) NOT NULL DEFAULT '',
    cost_bonus NUMERIC(12,2) NOT NULL DEFAULT 0.00,
    description VARCHAR(255) NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_bonus_log_user ON p_pt_bonus_log (user_id);
