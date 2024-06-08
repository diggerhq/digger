package main

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"strings"
)

var viperRunSpec *viper.Viper

type RunSpecConfig struct {
	Spec string `mapstructure:"spec"`
}

var runSpecCmd = &cobra.Command{
	Use:   "run_spec [flags]",
	Short: "run a spec",
	Long:  `run a spec`,
	Run: func(cmd *cobra.Command, args []string) {
		var runSpecConfig RunSpecConfig
		vipApply.Unmarshal(&runSpecConfig)
		var spec spec.Spec
		err := json.Unmarshal([]byte(runSpecConfig.Spec), &spec)
		if err != nil {
			usage.ReportErrorAndExit("", fmt.Sprintf("could not load spec json: %v", err), 1)
		}

	},
}

func init() {
	flags := []pflag.Flag{
		{Name: "spec", Usage: "string representing the json of a run spec"},
	}

	viperRunSpec = viper.New()
	viperRunSpec.SetEnvPrefix("DIGGER")
	viperRunSpec.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viperRunSpec.AutomaticEnv()

	for _, flag := range flags {
		runSpecCmd.Flags().String(flag.Name, "", flag.Usage)
		vipApply.BindPFlag(flag.Name, runSpecCmd.Flags().Lookup(flag.Name))
	}

	rootCmd.AddCommand(runSpecCmd)
}
