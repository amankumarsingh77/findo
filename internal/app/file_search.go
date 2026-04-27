// Package app provides the Wails-bound application layer.
package app

import (
	"path/filepath"

	"findo/internal/query"
	"findo/internal/search"
)

func (a *App) buildSearchEngine() *search.Engine {
	plannerCfg := search.PlannerConfig{
		BruteForceThreshold: a.getBruteForceThreshold(),
		OverFetchMultiplier: a.cfg.Search.OverFetchMultiplier,
	}
	planner := search.NewPlannerWithLogger(a.store, a.index, plannerCfg, a.logger.WithGroup("planner"))

	var filenameIdx search.FilenameIndex
	blendCfg := search.BlendConfig{
		RrfK:       a.cfg.FilenameSearch.RrfK,
		ExactBonus: a.cfg.FilenameSearch.ExactBonus,
	}
	if a.cfg.FilenameSearch.Enabled {
		filenameIdx = search.NewStoreFilenameIndex(a.store, a.cfg.FilenameSearch.FuzzyTopN)
	}

	return search.NewWithConfig(a.store, a.index, a.logger, planner, search.EngineConfig{
		Planner:     plannerCfg,
		Reranker:    search.RerankerConfig{RecencyBoostMultiplier: float32(a.cfg.Search.RecencyBoostMultiplier), RecencyWindowDays: a.cfg.Search.RecencyWindowDays},
		Ladder:      search.LadderConfig{Enabled: a.cfg.Search.RelaxationEnabled, DropOrder: search.ParseDropOrder(a.cfg.Search.RelaxationDropOrder)},
		Merger:      search.MergerConfig{Enabled: a.cfg.Search.FilenameMergeEnabled},
		FilenameIdx: filenameIdx,
		BlendCfg:    blendCfg,
	})
}

func blendedToSearchResultDTO(r search.BlendedResult) SearchResultDTO {
	dto := SearchResultDTO{
		FilePath:      r.File.Path,
		FileName:      filepath.Base(r.File.Path),
		FileType:      r.File.FileType,
		Extension:     r.File.Extension,
		SizeBytes:     r.File.SizeBytes,
		ThumbnailPath: r.File.ThumbnailPath,
		StartTime:     r.StartTime,
		EndTime:       r.EndTime,
		Score:         float32(r.Score),
		ModifiedAt:    r.File.ModifiedAt.Unix(),
		MatchKind:     r.MatchKind,
	}
	if len(r.Highlights) > 0 {
		dto.Highlights = make([]HighlightRangeDTO, len(r.Highlights))
		for i, h := range r.Highlights {
			dto.Highlights[i] = HighlightRangeDTO{Start: h.Start, End: h.End}
		}
	}
	return dto
}

// classifyAndEmbed classifies the raw query and embeds it only when the routing
// kind requires semantic search (KindContent or KindHybrid). It returns the kind,
// stripped query, and embedding vector (nil for KindFilename).
//
// When the filename index is unavailable (engine has no filenameIdx), the engine
// will internally force KindContent; the embedder is always called in that case.
func (a *App) classifyAndEmbed(raw string) (kind query.QueryKind, vec []float32, err error) {
	kind, _ = query.Classify(raw)

	if kind == query.KindFilename {
		return kind, nil, nil
	}

	emb, _ := a.snapshotEmbedderState()
	if emb == nil {
		return kind, nil, nil
	}

	vec, err = a.getQueryVector(emb, raw)
	return kind, vec, err
}

func blendedDTOs(results []search.BlendedResult) []SearchResultDTO {
	dtos := make([]SearchResultDTO, len(results))
	for i, r := range results {
		dtos[i] = blendedToSearchResultDTO(r)
	}
	return dtos
}
