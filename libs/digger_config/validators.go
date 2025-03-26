package digger_config

import "fmt"

func ValidateAutomergeStrategy(strategy string) error {
	switch strategy {
	case string(AutomergeStrategySquash), string(AutomergeStrategyMerge), string(AutomergeStrategyRebase):
		return nil
	default:
		return fmt.Errorf("invalid merge strategy: %v, valid values are: %v, %v, %v", strategy, AutomergeStrategySquash, AutomergeStrategyRebase, AutomergeStrategyRebase)
	}
}
