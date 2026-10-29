ALTER TABLE `p_user_pt_profile` ADD COLUMN `class` INTEGER NOT NULL DEFAULT 1;
ALTER TABLE `p_user_pt_profile` ADD COLUMN `max_class_once` INTEGER NOT NULL DEFAULT 1;
ALTER TABLE `p_user_pt_profile` ADD COLUMN `vip_until` INTEGER NOT NULL DEFAULT 0;
ALTER TABLE `p_user_pt_profile` ADD COLUMN `old_class` INTEGER NOT NULL DEFAULT 1;
ALTER TABLE `p_user_pt_profile` ADD COLUMN `custom_title` TEXT NOT NULL DEFAULT '';
