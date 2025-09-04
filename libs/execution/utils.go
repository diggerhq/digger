package execution

import (
	"fmt"
	"log/slog"
	"regexp"
)

func stringToRegex(input *string) (*regexp.Regexp, error) {
	var regEx *regexp.Regexp
	if input != nil {
		slog.Debug("using regex for filter", "regex", *input)
		var err error
		regEx, err = regexp.Compile(*input)
		if err != nil {
			slog.Error("invalid regex for filter",
				"regex", *input,
				"error", err)
			return nil, fmt.Errorf("regex for filter is invalid: %v", err)
		}
	} else {
		slog.Debug("no regex for filter")
		regEx = nil
	}
	return regEx, nil
}
