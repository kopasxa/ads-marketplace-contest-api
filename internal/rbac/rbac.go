package rbac

// Role constants
const (
	RoleOwner     = "owner"
	RoleManager   = "manager"
	RoleAdvertiser = "advertiser"
)

// Permission constants
const (
	PermManageChannel  = "manage_channel"
	PermManageListing  = "manage_listing"
	PermAcceptDeal     = "accept_deal"
	PermRejectDeal     = "reject_deal"
	PermSubmitCreative = "submit_creative"
	PermSetWallet      = "set_wallet"
	PermWithdraw       = "withdraw"
	PermCreateDeal     = "create_deal"
	PermApproveCreative = "approve_creative"
)

// RolePermissions defines what each role can do.
var RolePermissions = map[string][]string{
	RoleOwner: {
		PermManageChannel, PermManageListing, PermAcceptDeal, PermRejectDeal,
		PermSubmitCreative, PermSetWallet, PermWithdraw,
	},
	RoleManager: {
		PermManageChannel, PermManageListing, PermAcceptDeal, PermRejectDeal,
		PermSubmitCreative,
		// Manager CANNOT: PermSetWallet, PermWithdraw
	},
	RoleAdvertiser: {
		PermCreateDeal, PermApproveCreative,
	},
}

// HasPermission checks if a role has a specific permission.
func HasPermission(role, permission string) bool {
	perms, ok := RolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

// IsFinancialOperation checks if permission is financial (owner-only).
func IsFinancialOperation(permission string) bool {
	return permission == PermSetWallet || permission == PermWithdraw
}
