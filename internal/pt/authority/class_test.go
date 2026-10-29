package authority

import (
	"testing"
	"time"

	"github.com/rocboss/paopao-ce/internal/model/pt"
)

func TestAuthorityRulesAndPerks(t *testing.T) {
	// Retiree perk: free upload
	if !IsRetireeFreeUpload(pt.ClassRetiree) {
		t.Fatal("expected retiree to enjoy free upload (0x download)")
	}

	// Uploader perk: custom promotion
	if !CanCustomPromotion(pt.ClassUploader, false) {
		t.Fatal("expected uploader to have custom promotion permission")
	}

	// Normal user should not have custom promotion permission
	if CanCustomPromotion(pt.ClassUser, false) {
		t.Fatal("expected normal user not to have custom promotion permission")
	}

	// Crazy user can publish anonymously
	if !CanBeAnonymous(pt.ClassCrazyUser, false) {
		t.Fatal("expected crazy user to be allowed to publish anonymously")
	}

	// Karma multipliers
	if GetKarmaMultiplier(pt.ClassUploader) != 1.5 {
		t.Fatalf("expected uploader karma multiplier 1.5, got %f", GetKarmaMultiplier(pt.ClassUploader))
	}
	if GetKarmaMultiplier(pt.ClassVIP) != 1.2 {
		t.Fatalf("expected VIP karma multiplier 1.2, got %f", GetKarmaMultiplier(pt.ClassVIP))
	}

	// Download permission checks
	if !CanDownloadTorrent(pt.ClassUser, false) {
		t.Fatal("expected normal user to have download permission by default")
	}
	if CanDownloadTorrent(pt.ClassPeasant, false) {
		t.Fatal("expected peasant not to have general download permission by default (min_class_download=1)")
	}
	if !CanDownloadTorrent(pt.ClassPeasant, true) {
		t.Fatal("expected admin peasant to bypass and have download permission")
	}

	// Privilege list should not be empty
	retPerks := GetPrivilegeList(pt.ClassRetiree)
	if len(retPerks) == 0 {
		t.Fatal("expected retiree privileges to have entries")
	}

	if GetClassName(pt.ClassUser) != "普通会员" {
		t.Fatalf("expected 普通会员, got %s", GetClassName(pt.ClassUser))
	}
	if GetClassName(pt.ClassVIP) != "VIP" {
		t.Fatalf("expected VIP, got %s", GetClassName(pt.ClassVIP))
	}
	if GetClassColor(pt.ClassPowerUser) != "#1890ff" {
		t.Fatalf("expected #1890ff, got %s", GetClassColor(pt.ClassPowerUser))
	}
}

func TestCheckPromotionAndDemotion(t *testing.T) {
	now := time.Now().Unix()
	fiveWeeksAgo := now - 35*86400

	profile := &pt.UserPTProfile{
		UserID:       1,
		Class:        pt.ClassUser,
		MaxClassOnce: pt.ClassUser,
		Uploaded:     60 * 1024 * 1024 * 1024, // 60 GB
		Downloaded:   50 * 1024 * 1024 * 1024, // 50 GB -> Ratio: 1.2
	}

	// 1. Should promote to Power User (MinWeeks: 4, MinDownloadGB: 50, MinRatio: 1.05)
	newClass, invites, promoted := CheckPromotion(profile, fiveWeeksAgo)
	if !promoted || newClass != pt.ClassPowerUser {
		t.Fatalf("expected promote to PowerUser, got %d, promoted=%v", newClass, promoted)
	}
	if invites != 1 {
		t.Fatalf("expected 1 award invite, got %d", invites)
	}

	// 2. Test demotion when ratio drops below DemoteRatio (0.95 for PowerUser)
	puProfile := &pt.UserPTProfile{
		UserID:     1,
		Class:      pt.ClassPowerUser,
		Uploaded:   40 * 1024 * 1024 * 1024,
		Downloaded: 50 * 1024 * 1024 * 1024, // Ratio: 0.80 < 0.95
	}
	demoteClass, demoted := CheckDemotion(puProfile)
	if !demoted || demoteClass != pt.ClassUser {
		t.Fatalf("expected demote to User, got %d, demoted=%v", demoteClass, demoted)
	}

	// 3. Test Peasant demotion for User with bad ratio
	badUserProfile := &pt.UserPTProfile{
		UserID:     1,
		Class:      pt.ClassUser,
		Uploaded:   10 * 1024 * 1024 * 1024,
		Downloaded: 60 * 1024 * 1024 * 1024, // Ratio: 0.16 < 0.40
	}
	pClass, pDemoted := CheckDemotion(badUserProfile)
	if !pDemoted || pClass != pt.ClassPeasant {
		t.Fatalf("expected demote to Peasant, got %d, demoted=%v", pClass, pDemoted)
	}
}

func TestCustomDynamicPromotionRules(t *testing.T) {
	// Set custom rule: Power User requires only 1 week and 10 GB download and 1.0 ratio
	rules := GetAuthorityRules()
	origRules := *rules

	defer SetAuthorityRules(&origRules)

	customRules := origRules
	customRules.PromotionRules = []pt.PromotionRule{
		{
			Class:         pt.ClassPowerUser,
			ClassName:     "进阶用户",
			MinWeeks:      1,
			MinDownloadGB: 10.0,
			MinRatio:      1.0,
			DemoteRatio:   0.8,
			MinBonus:      0,
			AwardInvites:  3,
		},
	}
	SetAuthorityRules(&customRules)

	now := time.Now().Unix()
	twoWeeksAgo := now - 14*86400

	userProf := &pt.UserPTProfile{
		UserID:       2,
		Class:        pt.ClassUser,
		MaxClassOnce: pt.ClassUser,
		Uploaded:     15 * 1024 * 1024 * 1024,
		Downloaded:   12 * 1024 * 1024 * 1024, // 12 GB >= 10 GB, ratio 1.25 >= 1.0
	}

	prog := GetNextClassProgress(userProf, twoWeeksAgo)
	if prog == nil {
		t.Fatal("expected next class progress not nil")
	}
	if prog.ReqWeeks != 1 || prog.ReqDownloadGB != 10.0 || prog.ReqRatio != 1.0 {
		t.Fatalf("expected custom rules in progress, got ReqWeeks=%d ReqDownloadGB=%f", prog.ReqWeeks, prog.ReqDownloadGB)
	}

	newClass, invites, promoted := CheckPromotion(userProf, twoWeeksAgo)
	if !promoted || newClass != pt.ClassPowerUser {
		t.Fatalf("expected promotion with custom rules, got class %d, promoted=%v", newClass, promoted)
	}
	if invites != 3 {
		t.Fatalf("expected 3 invites from custom rules, got %d", invites)
	}
}

