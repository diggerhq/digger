package digger_config

import (
	"log/slog"
	"path"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
)

func GetPatternsRelativeToRepo(projectPath string, patterns []string) ([]string, error) {
	res := make([]string, 0)
	for _, pattern := range patterns {
		res = append(res, path.Join(projectPath, pattern))
	}
	return res, nil
}

func NormalizeFileName(fileName string) string {
	res, err := filepath.Abs(path.Join("/", fileName))
	if err != nil {
		slog.Error("failed to convert path to absolute", "fileName", fileName, "error", err)
		panic(err)
	}
	return res
}

func MatchIncludeExcludePatternsToFile(fileToMatch string, includePatterns []string, excludePatterns []string) bool {
	fileToMatch = NormalizeFileName(fileToMatch)
	for i := range includePatterns {
		includePatterns[i] = NormalizeFileName(includePatterns[i])
	}
	for i := range excludePatterns {
		excludePatterns[i] = NormalizeFileName(excludePatterns[i])
	}

	matching := false
	for _, ipattern := range includePatterns {
		isMatched, err := doublestar.PathMatch(ipattern, fileToMatch)
		if err != nil {
			slog.Error("failed to match modified files with include pattern",
				"file", fileToMatch,
				"pattern", ipattern,
				"error", err)
			panic(err)
		}
		if isMatched {
			matching = true
			break
		}
	}

	for _, epattern := range excludePatterns {
		excluded, err := doublestar.PathMatch(epattern, fileToMatch)
		if err != nil {
			slog.Error("failed to match modified files with exclude pattern",
				"file", fileToMatch,
				"pattern", epattern,
				"error", err)
			panic(err)
		}
		if excluded {
			matching = false
			break
		}
	}

	return matching
}
