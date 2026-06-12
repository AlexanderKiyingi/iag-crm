package models

import "strings"

type PermissionDescriptor struct {
	Name        string
	Description string
}

var PermissionModules = []string{
	"accounts", "contacts", "leads", "deals", "quotes", "activities", "tickets",
	"campaigns", "segments", "journeys", "personas", "events", "content", "email",
	"social", "seo", "budget", "mqls", "loyalty", "bridge", "outlets", "exports",
	"insights", "ai", "integrations", "audit", "admin",
}

func PermissionDescriptors() []PermissionDescriptor {
	actions := []struct {
		suffix string
		desc   string
	}{
		{"read", "View"},
		{"create", "Create"},
		{"update", "Update"},
		{"delete", "Delete"},
	}
	var out []PermissionDescriptor
	for _, m := range PermissionModules {
		for _, a := range actions {
			out = append(out, PermissionDescriptor{
				Name:        m + "." + a.suffix,
				Description: a.desc + " " + m,
			})
		}
	}
	return out
}

func PermissionCatalogData() map[string]any {
	keys := make([]string, 0, len(PermissionModules)*4)
	for _, m := range PermissionModules {
		for _, a := range []string{"read", "create", "update", "delete"} {
			keys = append(keys, m+"."+a)
		}
	}
	roleLabels := make(map[string]string, len(Roles))
	for id, spec := range Roles {
		roleLabels[id] = spec.Label
	}
	builtin := make([]string, 0, len(Roles))
	for id := range Roles {
		builtin = append(builtin, id)
	}
	return map[string]any{
		"modules":      PermissionModules,
		"actions":      []string{"read", "create", "update", "delete"},
		"allKeys":      keys,
		"roleLabels":   roleLabels,
		"builtinRoles": builtin,
	}
}

func BuiltinRolesPermissions() map[string]any {
	out := make(map[string]any, len(Roles))
	for id, spec := range Roles {
		out[id] = map[string]any{
			"label":  spec.Label,
			"full":   spec.Full,
			"pages":  spec.Pages,
			"modals": spec.Modals,
		}
	}
	return map[string]any{"roles": out}
}

func CanMutateRole(role string, perms []string) bool {
	if role == "md" || role == "head_commercial" || role == "head_marketing" {
		return true
	}
	for _, p := range perms {
		if p == "*" || stringsHasSuffixUpdate(p) {
			return true
		}
	}
	return false
}

func stringsHasSuffixUpdate(p string) bool {
	return strings.HasSuffix(p, ".create") || strings.HasSuffix(p, ".update") || strings.HasSuffix(p, ".delete")
}

func CanManageAdmin(perms []string, isStaff, isSuperuser bool) bool {
	if isSuperuser || isStaff {
		return true
	}
	for _, p := range perms {
		if p == "admin.read" || p == "admin.update" || p == "*" {
			return true
		}
	}
	return false
}
