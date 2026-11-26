package agentrun

// ToolSession keeps the tools, instructions, and catalog refresh owned by one run.
type ToolSession struct {
	Tools        []ReadTool
	Instructions string
	Refresh      func() ([]ReadTool, bool)
}
