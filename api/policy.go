package api

const (
	// FactMCPTool is the Datalog fact name injected by the Hub to authorize specific MCP tools.
	FactMCPTool = "allow_mcp_tool"
	
	// FactNetworkTarget is the Datalog fact name injected by the Hub to authorize connections to specific agents.
	FactNetworkTarget = "allow_network_target"
)

// PolicyConfig is the root authorization configuration for the SAM Hub.
type PolicyConfig struct {
	Version string                `yaml:"version"`
	Roles   map[string]RolePolicy `yaml:"roles"`
}

type RolePolicy struct {
	Network       NetworkPolicy `yaml:"network"`
	MCP           MCPPolicy     `yaml:"mcp"`
	CustomDatalog []string      `yaml:"custom_datalog"`
}

type NetworkPolicy struct {
	AllowedTargets []string `yaml:"allowed_targets"`
}

type MCPPolicy struct {
	AllowedTools []string `yaml:"allowed_tools"`
}

// LocalPolicy defines the optional attenuation rules for a specific SAM Node.
type LocalPolicy struct {
	Version     string      `yaml:"version"`
	Attenuation Attenuation `yaml:"attenuation"`
}

type Attenuation struct {
	Policies []string `yaml:"policies"`
	Checks   []string `yaml:"checks"`
	Rules    []string `yaml:"rules"`
}
