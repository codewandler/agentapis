package conversation

import "github.com/codewandler/agentapis/api/unified"

// Option configures a Session.
type Option func(*config)

type config struct {
	model       string
	maxTokens   int
	temperature float64
	tools       []unified.Tool
	toolChoice  unified.ToolChoice
	system      []string
	developer   []string
	strategy    Strategy
	caps        Capabilities
	projector   MessageProjector
}

func defaultConfig() config { return config{strategy: StrategyAuto, projector: DefaultMessageProjector{}} }

func applyOptions(opts []Option) config {
	cfg := defaultConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

func WithModel(model string) Option { return func(c *config) { c.model = model } }
func WithMaxTokens(max int) Option { return func(c *config) { c.maxTokens = max } }
func WithTemperature(v float64) Option { return func(c *config) { c.temperature = v } }
func WithTools(tools []unified.Tool) Option { return func(c *config) { c.tools = append([]unified.Tool(nil), tools...) } }
func WithToolChoice(choice unified.ToolChoice) Option { return func(c *config) { c.toolChoice = choice } }
func WithSystem(lines ...string) Option { return func(c *config) { c.system = append([]string(nil), lines...) } }
func WithDeveloper(lines ...string) Option { return func(c *config) { c.developer = append([]string(nil), lines...) } }
func WithStrategy(strategy Strategy) Option { return func(c *config) { c.strategy = strategy } }
func WithCapabilities(caps Capabilities) Option { return func(c *config) { c.caps = caps } }

// WithMessageProjector configures how canonical session state is projected into outbound messages for the next turn.
func WithMessageProjector(projector MessageProjector) Option { return func(c *config) { c.projector = projector } }
