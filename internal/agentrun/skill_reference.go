package agentrun

// SkillReference is the display metadata captured for an explicitly selected skill.
type SkillReference struct {
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
}
