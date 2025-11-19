package tfe

import (
	"log/slog"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/storage"
)

func requiresSandbox(unit *storage.UnitMetadata) bool {
	if unit == nil || unit.TFEExecutionMode == nil {
		slog.Info("🔍 requiresSandbox: unit has no execution mode set, defaulting to local",
			slog.String("unit_id", func() string {
				if unit != nil {
					return unit.ID
				}
				return "nil"
			}()))
		return false
	}
	mode := strings.TrimSpace(strings.ToLower(*unit.TFEExecutionMode))
	result := mode == "remote"
	slog.Info("🔍 requiresSandbox: execution mode check",
		slog.String("unit_id", unit.ID),
		slog.String("unit_name", unit.Name),
		slog.String("execution_mode", mode),
		slog.Bool("requires_sandbox", result))
	return result
}

func terraformVersionForUnit(unit *storage.UnitMetadata) string {
	if unit == nil || unit.TFETerraformVersion == nil {
		return ""
	}
	return strings.TrimSpace(*unit.TFETerraformVersion)
}

func engineForUnit(unit *storage.UnitMetadata) string {
	if unit == nil {
		slog.Warn("🔍 engineForUnit: unit is nil, defaulting to terraform")
		return "terraform"
	}
	if unit.TFEEngine == nil {
		slog.Warn("🔍 engineForUnit: TFEEngine is nil, defaulting to terraform",
			slog.String("unit_id", unit.ID),
			slog.String("unit_name", unit.Name))
		return "terraform" // Default to terraform
	}
	engine := strings.TrimSpace(strings.ToLower(*unit.TFEEngine))
	slog.Info("🔍 engineForUnit: engine determined",
		slog.String("unit_id", unit.ID),
		slog.String("unit_name", unit.Name),
		slog.String("raw_engine", *unit.TFEEngine),
		slog.String("normalized_engine", engine))
	if engine == "tofu" || engine == "opentofu" {
		return "tofu"
	}
	return "terraform"
}

func workingDirectoryForUnit(unit *storage.UnitMetadata) string {
	if unit == nil || unit.TFEWorkingDirectory == nil {
		return ""
	}
	return strings.TrimSpace(*unit.TFEWorkingDirectory)
}
