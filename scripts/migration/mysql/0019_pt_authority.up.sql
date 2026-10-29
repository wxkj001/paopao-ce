ALTER TABLE `p_user_pt_profile`
  ADD COLUMN `class` INT NOT NULL DEFAULT 1 COMMENT 'User PT Class level (0-16)',
  ADD COLUMN `max_class_once` INT NOT NULL DEFAULT 1 COMMENT 'Historical maximum class reached',
  ADD COLUMN `vip_until` BIGINT NOT NULL DEFAULT 0 COMMENT 'VIP expiration timestamp',
  ADD COLUMN `old_class` INT NOT NULL DEFAULT 1 COMMENT 'Class to revert to after VIP expires',
  ADD COLUMN `custom_title` VARCHAR(50) NOT NULL DEFAULT '' COMMENT 'Custom user title';
