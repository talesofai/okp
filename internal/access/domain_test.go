package access

import (
	"testing"

	"github.com/talesofai/okp/internal/model"
	"github.com/talesofai/okp/internal/testutil"
)

func TestPrivateDomainRequiresExplicitMembership(t *testing.T) {
	db := testutil.OpenDatabase(t)
	if err := db.Create(&model.User{UUID: "admin", AuthType: "test", Role: "admin"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.User{UUID: "reader", AuthType: "test", Role: "reader"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DomainMeta{Domain: "private", Visibility: VisibilityPrivate}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DomainMember{Domain: "private", UserID: "host", Role: "host"}).Error; err != nil {
		t.Fatal(err)
	}

	for _, userID := range []string{"", "admin", "reader"} {
		if CanReadDomain(userID, "private") {
			t.Fatalf("%q unexpectedly read private domain", userID)
		}
		if CanWriteDomain(userID, "private") {
			t.Fatalf("%q unexpectedly wrote private domain", userID)
		}
		if CanManageDomain(userID, "private") {
			t.Fatalf("%q unexpectedly managed private domain", userID)
		}
	}

	if err := db.Create(&model.DomainMember{Domain: "private", UserID: "admin", Role: "reader"}).Error; err != nil {
		t.Fatal(err)
	}
	if !CanReadDomain("admin", "private") || CanWriteDomain("admin", "private") || CanManageDomain("admin", "private") {
		t.Fatal("reader invite should grant an admin read-only private-domain access")
	}

	if err := db.Model(&model.DomainMember{}).Where("domain = ? AND user_id = ?", "private", "admin").Update("role", "writer").Error; err != nil {
		t.Fatal(err)
	}
	if !CanReadDomain("admin", "private") || !CanWriteDomain("admin", "private") || CanManageDomain("admin", "private") {
		t.Fatal("writer invite should not grant an admin private-domain management")
	}

	if !CanReadDomain("host", "private") || !CanWriteDomain("host", "private") || !CanManageDomain("host", "private") {
		t.Fatal("private-domain host should have full domain access")
	}
}

func TestPublicDomainKeepsAdminOverride(t *testing.T) {
	db := testutil.OpenDatabase(t)
	if err := db.Create(&model.User{UUID: "admin", AuthType: "test", Role: "admin"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.DomainMeta{Domain: "public", Visibility: VisibilityPublic}).Error; err != nil {
		t.Fatal(err)
	}

	if !CanReadDomain("admin", "public") || !CanWriteDomain("admin", "public") || !CanManageDomain("admin", "public") {
		t.Fatal("global admin should retain public-domain access")
	}
	if !CanReadDomain("reader", "public") || CanWriteDomain("reader", "public") || CanManageDomain("reader", "public") {
		t.Fatal("ordinary user should retain public read-only access")
	}
}
