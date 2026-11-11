// Package agentid defines CLI identities shared by adapters, directory resolvers
// and installers, without introducing an import cycle between those packages.
package agentid

// Domestic agent slugs are protocol identities, not interchangeable brands.
// Qwen, iFlow, CodeArts, CoStrict and Comate are outside this integration scope.
const (
	CodeBuddy       = "codebuddy"
	CodeBuddyIDE    = "codebuddy-ide"
	WorkBuddy       = "workbuddy"
	Qoder           = "qoder"
	QoderCN         = "qoder-cn"
	Lingma          = "lingma"
	QoderWork       = "qoderwork"
	ZCode           = "zcode"
	MiniMaxCode     = "minimax-code"
	DeepSeekHarness = "deepseek-harness"
)

// DomesticSlugs returns a fresh copy; defining an identity does not register an
// adapter or promise that a host protocol has been implemented or verified.
func DomesticSlugs() []string {
	return []string{CodeBuddy, CodeBuddyIDE, WorkBuddy, Qoder, QoderCN, Lingma,
		QoderWork, ZCode, MiniMaxCode, DeepSeekHarness}
}
