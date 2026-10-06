package identity

const (
	RoleRegistrar     = "registrar"
	RoleAcademicAdmin = "academic_admin"
	RoleFinanceAdmin  = "finance_admin"
	RoleTeacher       = "teacher"
	RoleHOD           = "hod"
	RoleSuperAdmin    = "super_admin"
)

func IsStaffRole(role string) bool {
	switch role {
	case RoleRegistrar, RoleAcademicAdmin, RoleFinanceAdmin, RoleTeacher, RoleHOD, RoleSuperAdmin:
		return true
	default:
		return false
	}
}

func IsAdminRole(role string) bool {
	switch role {
	case RoleRegistrar, RoleAcademicAdmin, RoleFinanceAdmin, RoleSuperAdmin:
		return true
	default:
		return false
	}
}
