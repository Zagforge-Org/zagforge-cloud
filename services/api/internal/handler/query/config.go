package query

// SystemPrompt is the system-level instruction prepended to every query.
const SystemPrompt = `You are an expert on this codebase. Refer to files by their full path. Answer concisely and precisely. If unsure, say so.`

// ProviderOrder defines the fallback preference for AI provider selection.
// The first provider with a configured key wins.
var ProviderOrder = []string{"anthropic", "openai", "google", "xai"}
