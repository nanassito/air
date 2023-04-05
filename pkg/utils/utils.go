package utils

import (
	"os"
	"path/filepath"

	"golang.org/x/exp/slog"
)

var Logger = slog.New(slog.HandlerOptions{
	AddSource: true,
	ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		if a.Key == slog.SourceKey {
			a.Value = slog.StringValue(filepath.Base(a.Value.String()))
		}
		return a
	},
}.NewTextHandler(os.Stdout))
