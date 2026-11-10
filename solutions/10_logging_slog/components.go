package shoplog

import (
	"context"
	"log/slog"
	"maps"
)

// componentLevels applies a per-component threshold on top of next.
type componentLevels struct {
	next      slog.Handler
	def       slog.Level
	levels    map[string]slog.Level
	floor     slog.Level // lowest threshold of def and all levels
	comp      string     // component set via top-level WithAttrs
	hasComp   bool
	groupOpen bool
}

// NewComponentLevels wraps next with per-component thresholds. A record's
// component (a top-level attribute, from With or the record itself, the
// record winning) selects its threshold from levels; anything else uses def.
// The levels map is copied. Enabled is a cheap prefilter using the lowest
// threshold, and next's own Enabled still applies. See README, Part 5.
func NewComponentLevels(next slog.Handler, def slog.Level, levels map[string]slog.Level) slog.Handler {
	h := &componentLevels{next: next, def: def, levels: maps.Clone(levels), floor: def}
	for _, lv := range h.levels {
		h.floor = min(h.floor, lv)
	}
	return h
}

// Enabled is false below the lowest configured threshold.
func (h *componentLevels) Enabled(ctx context.Context, lv slog.Level) bool {
	return lv >= h.floor && h.next.Enabled(ctx, lv)
}

// Handle drops records below their component's threshold and forwards the rest.
func (h *componentLevels) Handle(ctx context.Context, r slog.Record) error {
	comp, has := h.comp, h.hasComp
	if !h.groupOpen {
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "component" {
				if v := a.Value.Resolve(); v.Kind() == slog.KindString {
					comp, has = v.String(), true
				}
				return false
			}
			return true
		})
	}
	threshold := h.def
	if has {
		if lv, ok := h.levels[comp]; ok {
			threshold = lv
		}
	}
	if r.Level < threshold {
		return nil
	}
	return h.next.Handle(ctx, r)
}

// WithAttrs forwards the attributes and remembers a top-level component.
func (h *componentLevels) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := *h
	nh.next = h.next.WithAttrs(attrs)
	if !h.groupOpen {
		for _, a := range attrs {
			if a.Key != "component" {
				continue
			}
			if v := a.Value.Resolve(); v.Kind() == slog.KindString {
				nh.comp, nh.hasComp = v.String(), true
			}
		}
	}
	return &nh
}

// WithGroup forwards the group; later component attributes are nested and ignored.
func (h *componentLevels) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := *h
	nh.next = h.next.WithGroup(name)
	nh.groupOpen = true
	return &nh
}
