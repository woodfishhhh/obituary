package userdirs

// Qoder and Qoder CN share project .qoder, but never their user configuration.
// docs.qoder.com/cli/settings-reference and docs.qoder.cn/cli/settings-reference.
func QoderDir() (string, error) { return agentDir("QODER_CONFIG_DIR", ".qoder") }

func QoderCNDir() (string, error) { return agentDir("QODERCN_CONFIG_DIR", ".qoder-cn") }

func init() {
	RegisterAgentDirectory(AgentDirectory{Name: "qoder", Resolve: QoderDir})
	RegisterAgentDirectory(AgentDirectory{Name: "qoder-cn", Resolve: QoderCNDir})
}
