ALTER TABLE "p_user_pt_profile"
  DROP COLUMN IF EXISTS "class",
  DROP COLUMN IF EXISTS "max_class_once",
  DROP COLUMN IF EXISTS "vip_until",
  DROP COLUMN IF EXISTS "old_class",
  DROP COLUMN IF EXISTS "custom_title";
