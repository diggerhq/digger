package main

import (
	"encoding/json"
	"fmt"
	spec2 "github.com/diggerhq/digger/cli/pkg/spec"
	"github.com/diggerhq/digger/cli/pkg/usage"
	"github.com/diggerhq/digger/ee/cli/pkg/policy"
	comment_summary "github.com/diggerhq/digger/libs/comment_utils/summary"
	lib_spec "github.com/diggerhq/digger/libs/spec"
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
		viperRunSpec.Unmarshal(&runSpecConfig)
		var spec lib_spec.Spec
		err := json.Unmarshal([]byte(runSpecConfig.Spec), &spec)
		if err != nil {
			usage.ReportErrorAndExit("", fmt.Sprintf("could not load spec json: %v", err), 1)
		}
		err = spec2.RunSpec(
			spec,
			lib_spec.VCSProviderBasic{},
			lib_spec.JobSpecProvider{},
			lib_spec.LockProvider{},
			lib_spec.ReporterProvider{},
			lib_spec.BackendApiProvider{},
			policy.AdvancedPolicyProvider{},
			lib_spec.PlanStorageProvider{},
			lib_spec.VariablesProvider{},
			comment_summary.CommentUpdaterProviderBasic{},
		)
		if err != nil {
			usage.ReportErrorAndExit(spec.VCS.Actor, fmt.Sprintf("error running spec: %v", err), 1)
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
		viperRunSpec.BindPFlag(flag.Name, runSpecCmd.Flags().Lookup(flag.Name))
	}

	rootCmd.AddCommand(runSpecCmd)
}
