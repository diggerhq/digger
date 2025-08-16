package digger

import (
	"fmt"
	"log/slog"
)

const (
	ColorGreen = "\033[32m"
	ColorRed   = "\033[31m"
	ColorReset = "\033[0m"
)

func LogGreen(msg string, args ...interface{}) {
	formattedMsg := fmt.Sprintf(msg, args...)
	slog.Info(fmt.Sprintf("%s%s%s", ColorGreen, formattedMsg, ColorReset))
}

func LogRed(msg string, args ...interface{}) {
	formattedMsg := fmt.Sprintf(msg, args...)
	slog.Error(fmt.Sprintf("%s%s%s", ColorRed, formattedMsg, ColorReset))
}
