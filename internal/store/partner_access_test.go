package store

import "testing"

func TestPartnerRolePermissions(t *testing.T) {
	all := []PartnerPermission{
		PartnerReadOrders, PartnerUpdateOrder, PartnerReadMenu,
		PartnerUpdateMenu, PartnerManageFailures, PartnerManageCredential, PartnerReadAudit,
	}
	tests := []struct {
		role    PartnerRole
		allowed map[PartnerPermission]bool
	}{
		{PartnerRoleOwner, map[PartnerPermission]bool{
			PartnerReadOrders: true, PartnerUpdateOrder: true, PartnerReadMenu: true,
			PartnerUpdateMenu: true, PartnerManageFailures: true, PartnerManageCredential: true, PartnerReadAudit: true,
		}},
		{PartnerRoleManager, map[PartnerPermission]bool{
			PartnerReadOrders: true, PartnerUpdateOrder: true, PartnerReadMenu: true,
			PartnerUpdateMenu: true, PartnerManageFailures: true,
		}},
		{PartnerRoleKitchen, map[PartnerPermission]bool{
			PartnerReadOrders: true, PartnerUpdateOrder: true, PartnerReadMenu: true,
		}},
		{PartnerRoleOperator, map[PartnerPermission]bool{
			PartnerReadOrders: true, PartnerReadMenu: true, PartnerManageFailures: true, PartnerReadAudit: true,
		}},
	}
	for _, test := range tests {
		t.Run(string(test.role), func(t *testing.T) {
			identity := PartnerIdentity{Role: test.role}
			for _, permission := range all {
				if got := identity.Can(permission); got != test.allowed[permission] {
					t.Errorf("Can(%s) = %v, want %v", permission, got, test.allowed[permission])
				}
			}
		})
	}
	if (PartnerIdentity{Role: "unknown"}).Can(PartnerReadOrders) {
		t.Fatal("unknown role must not have permissions")
	}
}
