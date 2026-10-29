// Package authority implements user classes, promotion/demotion rules and permission checks for the PT system.
package authority

import (
	"math"
	"sync"
	"time"

	"github.com/rocboss/paopao-ce/internal/conf"
	"github.com/rocboss/paopao-ce/internal/model/pt"
)

var (
	rulesMu      sync.RWMutex
	currentRules = DefaultAuthorityRules()
)

// DefaultAuthorityRules returns initial standard rules aligned with NexusPHP.
func DefaultAuthorityRules() *pt.PTAuthorityRules {
	return &pt.PTAuthorityRules{
		MinClassDownload:    pt.ClassUser,
		MinClassUpload:      pt.ClassUser,
		MinClassPromotion:   pt.ClassUploader,
		MinClassAnonymous:   pt.ClassCrazyUser,
		MinClassViewPeers:   pt.ClassPowerUser,
		RetireeFreeUpload:   true,
		RetireeExemptDemote: true,
		UploaderKarmaMulti:  1.5,
		VipKarmaMulti:       1.2,
		VeteranKarmaMulti:   1.1,
		UltimateKarmaMulti:  1.2,
		MasterKarmaMulti:    1.3,
		PromotionRules:      DefaultPromotionRules(),
	}
}

// SetAuthorityRules updates active dynamic rules in memory.
func SetAuthorityRules(r *pt.PTAuthorityRules) {
	if r == nil {
		return
	}
	rulesMu.Lock()
	defer rulesMu.Unlock()
	if len(r.PromotionRules) == 0 {
		r.PromotionRules = DefaultPromotionRules()
	}
	for i := range r.PromotionRules {
		r.PromotionRules[i].ClassName = GetClassName(r.PromotionRules[i].Class)
	}
	currentRules = r
}

// GetAuthorityRules returns a copy of current active dynamic rules.
func GetAuthorityRules() *pt.PTAuthorityRules {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	if currentRules == nil {
		return DefaultAuthorityRules()
	}
	c := *currentRules
	if len(c.PromotionRules) == 0 {
		c.PromotionRules = DefaultPromotionRules()
	}
	for i := range c.PromotionRules {
		c.PromotionRules[i].ClassName = GetClassName(c.PromotionRules[i].Class)
	}
	return &c
}

// CanDownloadTorrent checks if user has permission to download torrents.
func CanDownloadTorrent(class int, isAdmin bool) bool {
	if isAdmin || class >= pt.ClassModerator {
		return true
	}
	r := GetAuthorityRules()
	return class >= r.MinClassDownload
}

// CanUploadTorrent checks if the user has permission to upload normal torrents.
func CanUploadTorrent(class int, isAdmin bool) bool {
	if isAdmin || class >= pt.ClassModerator {
		return true
	}
	r := GetAuthorityRules()
	return class >= r.MinClassUpload
}

// CanCustomPromotion checks if user can assign custom promotional states (Free/2X etc.).
func CanCustomPromotion(class int, isAdmin bool) bool {
	if isAdmin || class >= pt.ClassModerator {
		return true
	}
	r := GetAuthorityRules()
	return class >= r.MinClassPromotion
}

// CanBeAnonymous checks if user can publish torrent anonymously.
func CanBeAnonymous(class int, isAdmin bool) bool {
	if isAdmin || class >= pt.ClassModerator {
		return true
	}
	r := GetAuthorityRules()
	return class >= r.MinClassAnonymous
}

// CanViewPeerDetails checks if user can view raw IP and details of peers.
func CanViewPeerDetails(class int, isAdmin bool) bool {
	if isAdmin || class >= pt.ClassModerator {
		return true
	}
	r := GetAuthorityRules()
	return class >= r.MinClassViewPeers
}

// IsRetireeFreeUpload checks if retiree enjoys 0x download (free upload exemption).
func IsRetireeFreeUpload(class int) bool {
	if class == pt.ClassRetiree {
		r := GetAuthorityRules()
		return r.RetireeFreeUpload
	}
	return false
}

// IsExemptFromDemotion checks if user is immune to sharing ratio demotion.
func IsExemptFromDemotion(class int) bool {
	if class == pt.ClassVIP || class >= pt.ClassRetiree || class == pt.ClassNexusMaster {
		return true
	}
	if class == pt.ClassRetiree {
		r := GetAuthorityRules()
		return r.RetireeExemptDemote
	}
	return false
}

// GetKarmaMultiplier returns karma accumulation boost factor for active seeders.
func GetKarmaMultiplier(class int) float64 {
	r := GetAuthorityRules()
	switch class {
	case pt.ClassUploader:
		if r.UploaderKarmaMulti > 0 {
			return r.UploaderKarmaMulti
		}
		return 1.5
	case pt.ClassVIP:
		if r.VipKarmaMulti > 0 {
			return r.VipKarmaMulti
		}
		return 1.2
	case pt.ClassNexusMaster:
		if r.MasterKarmaMulti > 0 {
			return r.MasterKarmaMulti
		}
		return 1.3
	case pt.ClassUltimateUser, pt.ClassExtremeUser:
		if r.UltimateKarmaMulti > 0 {
			return r.UltimateKarmaMulti
		}
		return 1.2
	case pt.ClassVeteranUser, pt.ClassInsaneUser:
		if r.VeteranKarmaMulti > 0 {
			return r.VeteranKarmaMulti
		}
		return 1.1
	default:
		return 1.0
	}
}

// GetPrivilegeList returns descriptive tags for all perks enjoyed by class.
func GetPrivilegeList(class int) []string {
	var perks []string
	switch class {
	case pt.ClassPeasant:
		perks = append(perks, "⚠️ 受限考核期", "⛔ 禁止发种", "🔒 仅可下载免费种")
	case pt.ClassUser:
		perks = append(perks, "✅ 基础做种与下载", "📤 社区发推带种")
	case pt.ClassPowerUser:
		perks = append(perks, "🔍 查看Swarm详细节点", "📨 发送注册邀请", "📊 查看Top10排行榜")
	case pt.ClassEliteUser:
		perks = append(perks, "🔍 查看Swarm详细节点", "📨 更多邀请名额", "💎 做种魔力加成")
	case pt.ClassCrazyUser:
		perks = append(perks, "🕶️ 允许匿名发种", "🔍 查看Swarm详细节点", "💎 做种魔力加成")
	case pt.ClassInsaneUser, pt.ClassVeteranUser:
		perks = append(perks, "🕶️ 允许匿名发种", "💎 1.1x魔力加成", "📨 专属资深标识")
	case pt.ClassExtremeUser, pt.ClassUltimateUser:
		perks = append(perks, "🕶️ 允许匿名发种", "💎 1.2x魔力加成", "📨 高级邀请特权")
	case pt.ClassNexusMaster:
		perks = append(perks, "👑 终身会员荣誉", "💎 1.3x魔力加成", "🛡️ 永久免疫降为贫民")
	case pt.ClassVIP:
		perks = append(perks, "⚡ 0x下载计费(免下载量)", "🛡️ 永久免除考核", "💎 1.2x魔力加成", "👑 尊贵贵宾徽章")
	case pt.ClassRetiree:
		perks = append(perks, "⚡ 免贡献上传(0x下载)", "🛡️ 终生免除考核", "☕ 安心养老保护", "🏅 荣誉退役徽章")
	case pt.ClassUploader:
		perks = append(perks, "🏷️ 自由设定种子促销", "💎 1.5x超高魔力加成", "🚀 官方发种通道", "🛡️ 永久免受降级")
	case pt.ClassModerator, pt.ClassAdministrator, pt.ClassSysOp, pt.ClassStaffLeader:
		perks = append(perks, "🛡️ 全站管理特权", "🏷️ 种子促销/置顶/删种", "⚙️ 用户等级与权限管理")
	}
	return perks
}

// DefaultClassNames provides standard NexusPHP-aligned Chinese labels.
var DefaultClassNames = map[int]string{
	pt.ClassPeasant:       "贫民",
	pt.ClassUser:          "普通会员",
	pt.ClassPowerUser:     "进阶用户",
	pt.ClassEliteUser:     "精英用户",
	pt.ClassCrazyUser:     "疯狂用户",
	pt.ClassInsaneUser:    "疯魔用户",
	pt.ClassVeteranUser:   "骨灰用户",
	pt.ClassExtremeUser:   "极品用户",
	pt.ClassUltimateUser:  "巅峰用户",
	pt.ClassNexusMaster:   "大师用户",
	pt.ClassVIP:           "VIP",
	pt.ClassRetiree:       "养老退役",
	pt.ClassUploader:      "发布员",
	pt.ClassModerator:     "版主",
	pt.ClassAdministrator: "管理员",
	pt.ClassSysOp:         "技术主管",
	pt.ClassStaffLeader:   "站长",
}

// ClassColors provides visual color badges matching NexusPHP style.
var ClassColors = map[int]string{
	pt.ClassPeasant:       "#8c8c8c",
	pt.ClassUser:          "#595959",
	pt.ClassPowerUser:     "#1890ff",
	pt.ClassEliteUser:     "#2f54eb",
	pt.ClassCrazyUser:     "#722ed1",
	pt.ClassInsaneUser:    "#eb2f96",
	pt.ClassVeteranUser:   "#fa8c16",
	pt.ClassExtremeUser:   "#fa541c",
	pt.ClassUltimateUser:  "#f5222d",
	pt.ClassNexusMaster:   "#cf1322",
	pt.ClassVIP:           "#faad14",
	pt.ClassRetiree:       "#8c8c8c",
	pt.ClassUploader:      "#52c41a",
	pt.ClassModerator:     "#13c2c2",
	pt.ClassAdministrator: "#eb2f96",
	pt.ClassSysOp:         "#722ed1",
	pt.ClassStaffLeader:   "#ff4d4f",
}

// PromotionRule defines the requirement thresholds for a given class.
type PromotionRule = pt.PromotionRule

// DefaultPromotionRules lists standard promotion requirements in ascending order (ClassPowerUser -> ClassNexusMaster).
func DefaultPromotionRules() []pt.PromotionRule {
	return []pt.PromotionRule{
		{
			Class:         pt.ClassPowerUser,
			ClassName:     "进阶用户",
			MinWeeks:      4,
			MinDownloadGB: 50.0,
			MinRatio:      1.05,
			DemoteRatio:   0.95,
			MinBonus:      0,
			AwardInvites:  1,
		},
		{
			Class:         pt.ClassEliteUser,
			ClassName:     "精英用户",
			MinWeeks:      8,
			MinDownloadGB: 120.0,
			MinRatio:      1.55,
			DemoteRatio:   1.45,
			MinBonus:      0,
			AwardInvites:  2,
		},
		{
			Class:         pt.ClassCrazyUser,
			ClassName:     "狂热用户",
			MinWeeks:      15,
			MinDownloadGB: 300.0,
			MinRatio:      2.05,
			DemoteRatio:   1.95,
			MinBonus:      0,
			AwardInvites:  3,
		},
		{
			Class:         pt.ClassInsaneUser,
			ClassName:     "疯狂用户",
			MinWeeks:      25,
			MinDownloadGB: 500.0,
			MinRatio:      2.55,
			DemoteRatio:   2.45,
			MinBonus:      0,
			AwardInvites:  4,
		},
		{
			Class:         pt.ClassVeteranUser,
			ClassName:     "资深用户",
			MinWeeks:      40,
			MinDownloadGB: 750.0,
			MinRatio:      3.05,
			DemoteRatio:   2.95,
			MinBonus:      0,
			AwardInvites:  5,
		},
		{
			Class:         pt.ClassExtremeUser,
			ClassName:     "极限用户",
			MinWeeks:      60,
			MinDownloadGB: 1000.0,
			MinRatio:      3.55,
			DemoteRatio:   3.45,
			MinBonus:      0,
			AwardInvites:  6,
		},
		{
			Class:         pt.ClassUltimateUser,
			ClassName:     "巅峰用户",
			MinWeeks:      80,
			MinDownloadGB: 1500.0,
			MinRatio:      4.05,
			DemoteRatio:   3.95,
			MinBonus:      0,
			AwardInvites:  7,
		},
		{
			Class:         pt.ClassNexusMaster,
			ClassName:     "Nexus大师",
			MinWeeks:      100,
			MinDownloadGB: 3000.0,
			MinRatio:      4.55,
			DemoteRatio:   4.45,
			MinBonus:      0,
			AwardInvites:  10,
		},
	}
}

// GetPromotionRules returns dynamic promotion rules from memory cache or default if none set.
func GetPromotionRules() []pt.PromotionRule {
	rulesMu.RLock()
	defer rulesMu.RUnlock()
	if currentRules != nil && len(currentRules.PromotionRules) > 0 {
		return currentRules.PromotionRules
	}
	return DefaultPromotionRules()
}

// StandardPromotionRules provides standard rules fallback for compatibility.
var StandardPromotionRules = DefaultPromotionRules()

// GetClassName returns the Chinese label of class, respecting custom configuration if defined.
func GetClassName(class int) string {
	if conf.PTSetting != nil && conf.PTSetting.Authority.ClassNames != nil {
		if name, ok := conf.PTSetting.Authority.ClassNames[class]; ok && name != "" {
			return name
		}
	}
	if name, ok := DefaultClassNames[class]; ok {
		return name
	}
	return "未知等级"
}

// GetClassColor returns the badge color for the class.
func GetClassColor(class int) string {
	if color, ok := ClassColors[class]; ok {
		return color
	}
	return "#595959"
}

// GetNextClassProgress computes criteria and user's current progress toward the next class.
func GetNextClassProgress(profile *pt.UserPTProfile, userCreatedAt int64) *pt.NextClassProgress {
	if profile == nil || profile.Class >= pt.ClassNexusMaster || profile.Class == pt.ClassPeasant {
		return nil
	}

	targetClass := profile.Class + 1
	rules := GetPromotionRules()
	var rule *pt.PromotionRule
	for i := range rules {
		if rules[i].Class == targetClass {
			rule = &rules[i]
			break
		}
	}
	if rule == nil {
		return nil
	}

	// Calculate elapsed registration weeks
	elapsedWeeks := 0
	if userCreatedAt > 0 {
		diff := time.Now().Unix() - userCreatedAt
		if diff > 0 {
			elapsedWeeks = int(diff / (86400 * 7))
		}
	}

	curUploadGB := float64(profile.Uploaded) / (1024 * 1024 * 1024)
	curDownloadGB := float64(profile.Downloaded) / (1024 * 1024 * 1024)
	var curRatio float64
	if profile.Downloaded > 0 {
		curRatio = float64(profile.Uploaded) / float64(profile.Downloaded)
	} else if profile.Uploaded > 0 {
		curRatio = 999.0
	}

	return &pt.NextClassProgress{
		NextClass:     targetClass,
		NextClassName: GetClassName(targetClass),
		ReqWeeks:      rule.MinWeeks,
		CurWeeks:      elapsedWeeks,
		ReqUploadGB:   math.Round(rule.MinDownloadGB*rule.MinRatio*100) / 100,
		CurUploadGB:   math.Round(curUploadGB*100) / 100,
		ReqDownloadGB: rule.MinDownloadGB,
		CurDownloadGB: math.Round(curDownloadGB*100) / 100,
		ReqRatio:      rule.MinRatio,
		CurRatio:      math.Round(curRatio*100) / 100,
		ReqBonus:      rule.MinBonus,
		CurBonus:      math.Round(profile.SeedBonus*100) / 100,
	}
}

// CheckPromotion evaluates whether the user qualifies for a higher class.
func CheckPromotion(profile *pt.UserPTProfile, userCreatedAt int64) (newClass int, awardInvites int, promoted bool) {
	if profile == nil || profile.Class < pt.ClassUser || profile.Class >= pt.ClassNexusMaster {
		return profile.Class, 0, false
	}

	// Calculate user registration weeks
	elapsedWeeks := 0
	if userCreatedAt > 0 {
		diff := time.Now().Unix() - userCreatedAt
		if diff > 0 {
			elapsedWeeks = int(diff / (86400 * 7))
		}
	}

	curDownloadGB := float64(profile.Downloaded) / (1024 * 1024 * 1024)
	var curRatio float64
	if profile.Downloaded > 0 {
		curRatio = float64(profile.Uploaded) / float64(profile.Downloaded)
	} else if profile.Uploaded > 0 {
		curRatio = 999.0
	}

	highestQualifiedClass := profile.Class
	totalInvites := 0

	rules := GetPromotionRules()
	// Check sequentially from user's next class up to Nexus Master
	for _, rule := range rules {
		if rule.Class <= profile.Class {
			continue
		}
		if elapsedWeeks >= rule.MinWeeks && curDownloadGB >= rule.MinDownloadGB && curRatio >= rule.MinRatio && profile.SeedBonus >= rule.MinBonus {
			highestQualifiedClass = rule.Class
			if rule.Class > profile.MaxClassOnce {
				totalInvites += rule.AwardInvites
			}
		} else {
			// Requirements are progressive; stop at first unmet tier
			break
		}
	}

	if highestQualifiedClass > profile.Class {
		return highestQualifiedClass, totalInvites, true
	}
	return profile.Class, 0, false
}

// CheckDemotion evaluates whether user's ratio has dropped below threshold.
func CheckDemotion(profile *pt.UserPTProfile) (newClass int, demoted bool) {
	if profile == nil || profile.Class <= pt.ClassPeasant || IsExemptFromDemotion(profile.Class) {
		// VIP, Retiree, Uploader, Staff and Master are exempt from auto demotion
		return profile.Class, false
	}

	curDownloadGB := float64(profile.Downloaded) / (1024 * 1024 * 1024)
	var curRatio float64
	if profile.Downloaded > 0 {
		curRatio = float64(profile.Uploaded) / float64(profile.Downloaded)
	} else {
		return profile.Class, false
	}

	// Case 1: Check Peasant demotion (User class with heavy download and terrible ratio)
	if profile.Class == pt.ClassUser {
		if curDownloadGB >= 50.0 && curRatio < 0.40 {
			return pt.ClassPeasant, true
		}
		return profile.Class, false
	}

	// Case 2: Check stepped demotion for classes above User
	rules := GetPromotionRules()
	for i := len(rules) - 1; i >= 0; i-- {
		rule := rules[i]
		if rule.Class == profile.Class {
			if curRatio < rule.DemoteRatio {
				return profile.Class - 1, true
			}
			break
		}
	}

	return profile.Class, false
}

// CheckPeasantRecovery checks if a Peasant has raised their ratio back up to recover to normal User class.
func CheckPeasantRecovery(profile *pt.UserPTProfile) (recovered bool) {
	if profile == nil || profile.Class != pt.ClassPeasant {
		return false
	}
	var curRatio float64
	if profile.Downloaded > 0 {
		curRatio = float64(profile.Uploaded) / float64(profile.Downloaded)
	} else {
		curRatio = 1.0
	}
	// Ratio reaches 0.80 or above -> recovers to standard User
	return curRatio >= 0.80
}
