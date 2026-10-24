CREATE TABLE IF NOT EXISTS `p_pt_category` (
  `id` int NOT NULL AUTO_INCREMENT,
  `name` varchar(50) NOT NULL DEFAULT '',
  `icon` varchar(50) NOT NULL DEFAULT '',
  `sort` int NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

INSERT INTO `p_pt_category` (`id`, `name`, `icon`, `sort`) VALUES
(1, '电影', 'film-outline', 10),
(2, '电视剧', 'tv-outline', 20),
(3, '动漫', 'color-palette-outline', 30),
(4, '音乐', 'musical-notes-outline', 40),
(5, '游戏', 'game-controller-outline', 50),
(6, '应用软件', 'apps-outline', 60),
(7, '电子书/资料', 'book-outline', 70)
ON DUPLICATE KEY UPDATE `name`=VALUES(`name`);

CREATE TABLE IF NOT EXISTS `p_torrent` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `post_id` bigint NOT NULL DEFAULT 0,
  `info_hash` varchar(40) NOT NULL DEFAULT '',
  `name` varchar(255) NOT NULL DEFAULT '',
  `clean_name` varchar(255) NOT NULL DEFAULT '',
  `small_descr` varchar(255) NOT NULL DEFAULT '',
  `descr` mediumtext,
  `category_id` int NOT NULL DEFAULT 1,
  `size` bigint NOT NULL DEFAULT 0,
  `num_files` int NOT NULL DEFAULT 1,
  `user_id` bigint NOT NULL DEFAULT 0,
  `sp_state` tinyint NOT NULL DEFAULT 1 COMMENT '1:Normal, 2:Free, 3:2X, 4:2XFree, 5:50%, 6:2X50%, 7:30%',
  `promotion_until` bigint NOT NULL DEFAULT 0,
  `seeders` int NOT NULL DEFAULT 0,
  `leechers` int NOT NULL DEFAULT 0,
  `times_completed` int NOT NULL DEFAULT 0,
  `hits` int NOT NULL DEFAULT 0,
  `is_anonymous` tinyint(1) NOT NULL DEFAULT 0,
  `status` tinyint NOT NULL DEFAULT 0 COMMENT '0:Normal, 1:Banned, 2:Audit',
  `nfo` mediumtext,
  `file_path` varchar(255) NOT NULL DEFAULT '',
  `created_at` bigint NOT NULL DEFAULT 0,
  `updated_at` bigint NOT NULL DEFAULT 0,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_torrent_info_hash` (`info_hash`),
  KEY `idx_torrent_post_id` (`post_id`),
  KEY `idx_torrent_user_id` (`user_id`),
  KEY `idx_torrent_category_id` (`category_id`),
  KEY `idx_torrent_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `p_torrent_file` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `torrent_id` bigint unsigned NOT NULL DEFAULT 0,
  `file_path` varchar(512) NOT NULL DEFAULT '',
  `size` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_file_torrent_id` (`torrent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `p_torrent_peer` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `torrent_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_id` bigint NOT NULL DEFAULT 0,
  `peer_id` varchar(40) NOT NULL DEFAULT '',
  `ip` varchar(64) NOT NULL DEFAULT '',
  `port` int NOT NULL DEFAULT 0,
  `uploaded` bigint unsigned NOT NULL DEFAULT 0,
  `downloaded` bigint unsigned NOT NULL DEFAULT 0,
  `to_go` bigint unsigned NOT NULL DEFAULT 0,
  `is_seeder` tinyint(1) NOT NULL DEFAULT 0,
  `agent` varchar(100) NOT NULL DEFAULT '',
  `last_action` bigint NOT NULL DEFAULT 0,
  `started` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_torrent_peer` (`torrent_id`, `peer_id`),
  KEY `idx_peer_user_id` (`user_id`),
  KEY `idx_peer_last_action` (`last_action`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `p_torrent_snatched` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `torrent_id` bigint unsigned NOT NULL DEFAULT 0,
  `user_id` bigint NOT NULL DEFAULT 0,
  `uploaded` bigint unsigned NOT NULL DEFAULT 0,
  `downloaded` bigint unsigned NOT NULL DEFAULT 0,
  `to_go` bigint unsigned NOT NULL DEFAULT 0,
  `seed_time` bigint NOT NULL DEFAULT 0,
  `leech_time` bigint NOT NULL DEFAULT 0,
  `is_finished` tinyint(1) NOT NULL DEFAULT 0,
  `completed_at` bigint NOT NULL DEFAULT 0,
  `last_action` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_user_torrent_snatch` (`torrent_id`, `user_id`),
  KEY `idx_snatch_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `p_user_pt_profile` (
  `user_id` bigint NOT NULL,
  `passkey` varchar(32) NOT NULL DEFAULT '',
  `uploaded` bigint unsigned NOT NULL DEFAULT 0,
  `downloaded` bigint unsigned NOT NULL DEFAULT 0,
  `seed_bonus` decimal(12,2) NOT NULL DEFAULT 0.00,
  `seed_time` bigint NOT NULL DEFAULT 0,
  `leech_time` bigint NOT NULL DEFAULT 0,
  `can_leech` tinyint(1) NOT NULL DEFAULT 1,
  `can_upload` tinyint(1) NOT NULL DEFAULT 1,
  `invites` int NOT NULL DEFAULT 0,
  `created_at` bigint NOT NULL DEFAULT 0,
  `updated_at` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`user_id`),
  UNIQUE KEY `idx_user_passkey` (`passkey`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `p_pt_bonus_log` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint NOT NULL DEFAULT 0,
  `action_type` varchar(50) NOT NULL DEFAULT '',
  `cost_bonus` decimal(12,2) NOT NULL DEFAULT 0.00,
  `description` varchar(255) NOT NULL DEFAULT '',
  `created_at` bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_bonus_log_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
