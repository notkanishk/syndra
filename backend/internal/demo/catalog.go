package demo

import (
	"slices"

	"mkauth/internal/models"
)

var users = []models.UserProfile{
	{ID: "dev_admin", Name: "Alice Rivera", Email: "alice@makerspace.local", Title: "Makerspace Director", Team: "Operations", Status: "active", Avatar: "AR", Location: "HQ"},
	{ID: "sam_student", Name: "Sam Patel", Email: "sam@makerspace.local", Title: "Student Maker", Team: "Members", Status: "active", Avatar: "SP", Location: "Campus"},
	{ID: "maya_staff", Name: "Maya Chen", Email: "maya@makerspace.local", Title: "Lab Coordinator", Team: "Staff", Status: "active", Avatar: "MC", Location: "HQ"},
	{ID: "leo_mentor", Name: "Leo Brooks", Email: "leo@makerspace.local", Title: "Laser Mentor", Team: "Training", Status: "active", Avatar: "LB", Location: "Annex"},
	{ID: "ava_guest", Name: "Ava Morgan", Email: "ava@makerspace.local", Title: "Visiting Artist", Team: "Residency", Status: "pending", Avatar: "AM", Location: "Studio"},
}

var projects = []models.ProjectCatalog{
	{
		ID:          "platform",
		Name:        "Platform Core",
		Kind:        "internal",
		Description: "Administrative controls for makerspace operations and governance.",
		Roles: []models.ProjectRole{
			{Key: "admin", Label: "Administrator", Description: "Full control over policies and assignments."},
			{Key: "support", Label: "Support", Description: "Operational support and day-to-day coordination."},
			{Key: "auditor", Label: "Auditor", Description: "Read-only governance visibility."},
		},
	},
	{
		ID:          "printing",
		Name:        "Printing Lab",
		Kind:        "application",
		Description: "3D printing intake, queueing, and certification access.",
		Roles: []models.ProjectRole{
			{Key: "member", Label: "Member", Description: "Standard printing access."},
			{Key: "manager", Label: "Manager", Description: "Can manage printers and materials."},
			{Key: "calibrator", Label: "Calibrator", Description: "Can tune and certify machines."},
		},
	},
	{
		ID:          "laser",
		Name:        "Laser Lab",
		Kind:        "application",
		Description: "Laser cutter training, scheduling, and supervision.",
		Roles: []models.ProjectRole{
			{Key: "trainee", Label: "Trainee", Description: "In training for laser access."},
			{Key: "mentor", Label: "Mentor", Description: "Can train and approve members."},
			{Key: "operator", Label: "Operator", Description: "Can independently run machines."},
		},
	},
	{
		ID:          "doors",
		Name:        "Door Access",
		Kind:        "infrastructure",
		Description: "Physical access control groups used by door readers and keypads.",
		Roles: []models.ProjectRole{
			{Key: "3d_lab_pin", Label: "3D Lab PIN", Description: "Unlocks the printing lab."},
			{Key: "laser_room_pin", Label: "Laser Room PIN", Description: "Unlocks the laser room."},
			{Key: "staff_entry", Label: "Staff Entry", Description: "Back-office and early access."},
		},
	},
	{
		ID:          "wiki",
		Name:        "Knowledge Base",
		Kind:        "application",
		Description: "Internal documentation, safety guides, and certification references.",
		Roles: []models.ProjectRole{
			{Key: "member", Label: "Member", Description: "Read standard guides."},
			{Key: "editor", Label: "Editor", Description: "Can update docs and runbooks."},
			{Key: "admin", Label: "Admin", Description: "Full control over the knowledge base."},
		},
	},
	{
		ID:          "finance",
		Name:        "Material Counter",
		Kind:        "application",
		Description: "Tracks material checkout, credits, and budget approvals.",
		Roles: []models.ProjectRole{
			{Key: "material_checkout", Label: "Material Checkout", Description: "Can issue inventory to members."},
			{Key: "budget_approver", Label: "Budget Approver", Description: "Can approve spend."},
		},
	},
}

var applications = []models.ApplicationCatalog{
	{
		ID:          "printing-portal",
		Name:        "Printing Portal",
		ProjectID:   "printing",
		Description: "Queue, certify, and track work orders for 3D print jobs.",
		Consumer:    "Next.js member portal",
		ClaimName:   "x_mkauth_roles",
		FormatType:  "array",
	},
	{
		ID:          "door-controller",
		Name:        "Door Controller",
		ProjectID:   "doors",
		Description: "Consumes flattened access groups for physical entry hardware.",
		Consumer:    "ESP32 keypad bridge",
		ClaimName:   "door_groups",
		FormatType:  "csv",
	},
	{
		ID:          "makerspace-wiki",
		Name:        "Knowledge Base",
		ProjectID:   "wiki",
		Description: "Role-shaped groups for docs visibility and editor access.",
		Consumer:    "Outline workspace",
		ClaimName:   "groups",
		FormatType:  "array",
	},
	{
		ID:          "material-counter",
		Name:        "Material Counter",
		ProjectID:   "finance",
		Description: "Receives compact permission strings for POS workflows.",
		Consumer:    "Counter tablet",
		ClaimName:   "permissions",
		FormatType:  "space_delimited",
	},
}

var baseGrants = map[string][]models.RoleGrant{
	"dev_admin": {
		{ProjectID: "platform", RoleKey: "admin"},
		{ProjectID: "printing", RoleKey: "manager"},
		{ProjectID: "laser", RoleKey: "mentor"},
	},
	"sam_student": {
		{ProjectID: "printing", RoleKey: "member"},
	},
	"maya_staff": {
		{ProjectID: "platform", RoleKey: "support"},
	},
	"leo_mentor": {
		{ProjectID: "laser", RoleKey: "mentor"},
		{ProjectID: "printing", RoleKey: "calibrator"},
	},
	"ava_guest": {
		{ProjectID: "wiki", RoleKey: "member"},
	},
}

func Users() []models.UserProfile {
	return slices.Clone(users)
}

func Projects() []models.ProjectCatalog {
	return slices.Clone(projects)
}

func Applications() []models.ApplicationCatalog {
	return slices.Clone(applications)
}

func FindUser(userID string) (models.UserProfile, bool) {
	for _, user := range users {
		if user.ID == userID {
			return user, true
		}
	}
	return models.UserProfile{}, false
}

func FindProject(projectID string) (models.ProjectCatalog, bool) {
	for _, project := range projects {
		if project.ID == projectID {
			return project, true
		}
	}
	return models.ProjectCatalog{}, false
}

func FindApplication(appID string) (models.ApplicationCatalog, bool) {
	for _, app := range applications {
		if app.ID == appID {
			return app, true
		}
	}
	return models.ApplicationCatalog{}, false
}

func ProjectName(projectID string) string {
	project, ok := FindProject(projectID)
	if !ok {
		return projectID
	}
	return project.Name
}

func BaseGrants(userID string) []models.RoleGrant {
	return slices.Clone(baseGrants[userID])
}

func RoleKeysForProject(projectID string) []string {
	project, ok := FindProject(projectID)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(project.Roles))
	for _, role := range project.Roles {
		keys = append(keys, role.Key)
	}
	return keys
}
