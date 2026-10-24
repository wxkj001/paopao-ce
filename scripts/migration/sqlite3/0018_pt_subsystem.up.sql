CREATE TABLE IF NOT EXISTS `p_pt_category` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `name` TEXT NOT NULL DEFAULT '',
  `icon` TEXT NOT NULL DEFAULT '',
  `sort` INTEGER NOT NULL DEFAULT 0
);

INSERT OR IGNORE INTO `p_pt_category` (`id`, `name`, `icon`, `sort`) VALUES
(1, '电影', 'film-outline', 10),
(2, '电视剧', 'tv-outline', 20),
(3, '动漫', 'color-palette-outline', 30),
(4, '音乐', 'musical-notes-outline', 40),
(5, '游戏', 'game-controller-outline', 50),
(6, '应用软件', 'apps-outline', 60),
(7, '电子书/资料', 'book-outline', 70);

CREATE TABLE IF NOT EXISTS `p_torrent` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `post_id` INTEGER NOT NULL DEFAULT 0,
  `info_hash` TEXT NOT NULL DEFAULT '',
  `name` TEXT NOT NULL DEFAULT '',
  `clean_name` TEXT NOT NULL DEFAULT '',
  `small_descr` TEXT NOT NULL DEFAULT '',
  `descr` TEXT,
  `category_id` INTEGER NOT NULL DEFAULT 1,
  `size` INTEGER NOT NULL DEFAULT 0,
  `num_files` INTEGER NOT NULL DEFAULT 1,
  `user_id` INTEGER NOT NULL DEFAULT 0,
  `sp_state` INTEGER NOT NULL DEFAULT 1,
  `promotion_until` INTEGER NOT NULL DEFAULT 0,
  `seeders` INTEGER NOT NULL DEFAULT 0,
  `leechers` INTEGER NOT NULL DEFAULT 0,
  `times_completed` INTEGER NOT NULL DEFAULT 0,
  `hits` INTEGER NOT NULL DEFAULT 0,
  `is_anonymous` INTEGER NOT NULL DEFAULT 0,
  `status` INTEGER NOT NULL DEFAULT 0,
  `nfo` TEXT,
  `file_path` TEXT NOT NULL DEFAULT '',
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0,
  `deleted_at` DATETIME DEFAULT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS `idx_torrent_info_hash` ON `p_torrent` (`info_hash`);
CREATE INDEX IF NOT EXISTS `idx_torrent_post_id` ON `p_torrent` (`post_id`);
CREATE INDEX IF NOT EXISTS `idx_torrent_user_id` ON `p_torrent` (`user_id`);
CREATE INDEX IF NOT EXISTS `idx_torrent_category_id` ON `p_torrent` (`category_id`);
CREATE INDEX IF NOT EXISTS `idx_torrent_deleted_at` ON `p_torrent` (`deleted_at`);

CREATE TABLE IF NOT EXISTS `p_torrent_file` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `torrent_id` INTEGER NOT NULL DEFAULT 0,
  `file_path` TEXT NOT NULL DEFAULT '',
  `size` INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS `idx_file_torrent_id` ON `p_torrent_file` (`torrent_id`);

CREATE TABLE IF NOT EXISTS `p_torrent_peer` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `torrent_id` INTEGER NOT NULL DEFAULT 0,
  `user_id` INTEGER NOT NULL DEFAULT 0,
  `peer_id` TEXT NOT NULL DEFAULT '',
  `ip` TEXT NOT NULL DEFAULT '',
  `port` INTEGER NOT NULL DEFAULT 0,
  `uploaded` INTEGER NOT NULL DEFAULT 0,
  `downloaded` INTEGER NOT NULL DEFAULT 0,
  `to_go` INTEGER NOT NULL DEFAULT 0,
  `is_seeder` INTEGER NOT NULL DEFAULT 0,
  `agent` TEXT NOT NULL DEFAULT '',
  `last_action` INTEGER NOT NULL DEFAULT 0,
  `started` INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS `idx_torrent_peer` ON `p_torrent_peer` (`torrent_id`, `peer_id`);
CREATE INDEX IF NOT EXISTS `idx_peer_user_id` ON `p_torrent_peer` (`user_id`);
CREATE INDEX IF NOT EXISTS `idx_peer_last_action` ON `p_torrent_peer` (`last_action`);

CREATE TABLE IF NOT EXISTS `p_torrent_snatched` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `torrent_id` INTEGER NOT NULL DEFAULT 0,
  `user_id` INTEGER NOT NULL DEFAULT 0,
  `uploaded` INTEGER NOT NULL DEFAULT 0,
  `downloaded` INTEGER NOT NULL DEFAULT 0,
  `to_go` INTEGER NOT NULL DEFAULT 0,
  `seed_time` INTEGER NOT NULL DEFAULT 0,
  `leech_time` INTEGER NOT NULL DEFAULT 0,
  `is_finished` INTEGER NOT NULL DEFAULT 0,
  `completed_at` INTEGER NOT NULL DEFAULT 0,
  `last_action` INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS `idx_user_torrent_snatch` ON `p_torrent_snatched` (`torrent_id`, `user_id`);
CREATE INDEX IF NOT EXISTS `idx_snatch_user_id` ON `p_torrent_snatched` (`user_id`);

CREATE TABLE IF NOT EXISTS `p_user_pt_profile` (
  `user_id` INTEGER PRIMARY KEY,
  `passkey` TEXT NOT NULL DEFAULT '',
  `uploaded` INTEGER NOT NULL DEFAULT 0,
  `downloaded` INTEGER NOT NULL DEFAULT 0,
  `seed_bonus` REAL NOT NULL DEFAULT 0.0,
  `seed_time` INTEGER NOT NULL DEFAULT 0,
  `leech_time` INTEGER NOT NULL DEFAULT 0,
  `can_leech` INTEGER NOT NULL DEFAULT 1,
  `can_upload` INTEGER NOT NULL DEFAULT 1,
  `invites` INTEGER NOT NULL DEFAULT 0,
  `created_at` INTEGER NOT NULL DEFAULT 0,
  `updated_at` INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS `idx_user_passkey` ON `p_user_pt_profile` (`passkey`);

CREATE TABLE IF NOT EXISTS `p_pt_bonus_log` (
  `id` INTEGER PRIMARY KEY AUTOINCREMENT,
  `user_id` INTEGER NOT NULL DEFAULT 0,
  `action_type` TEXT NOT NULL DEFAULT '',
  `cost_bonus` REAL NOT NULL DEFAULT 0.0,
  `description` TEXT NOT NULL DEFAULT '',
  `created_at` INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS `idx_bonus_log_user` ON `p_pt_bonus_log` (`user_id`);
