package app

import (
	"findo/internal/embedder"
)

func (a *App) snapshotEmbedderState() (embedder.Embedder, llmQueryParser) {
	a.apiKeyMu.RLock()
	defer a.apiKeyMu.RUnlock()
	return a.embedder, a.llmParser
}
