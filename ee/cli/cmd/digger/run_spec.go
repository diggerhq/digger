package main

import (
	"encoding/json"
	"fmt"
	spec2 "github.com/diggerhq/digger/cli/pkg/spec"
	"github.com/diggerhq/digger/cli/pkg/usage"
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

func RunSpecFromString(specStr string) (string, error) {
	var spec lib_spec.Spec
	err := json.Unmarshal([]byte(specStr), &spec)
	if err != nil {
		return "", fmt.Errorf("could not load spec json: %v", err)
	}
	err = spec2.RunSpec(
		spec,
		lib_spec.VCSProvider{},
		lib_spec.JobSpecProvider{},
		lib_spec.LockProvider{},
		lib_spec.ReporterProvider{},
		lib_spec.BackendApiProvider{},
		lib_spec.PolicyProvider{},
		lib_spec.PlanStorageProvider{},
		comment_summary.CommentUpdaterProviderBasic{},
	)
	return spec.VCS.Actor, err
}

var runSpecCmd = &cobra.Command{
	Use:   "run_spec [flags]",
	Short: "run a spec",
	Long:  `run a spec`,
	Run: func(cmd *cobra.Command, args []string) {
		var runSpecConfig RunSpecConfig
		viperRunSpec.Unmarshal(&runSpecConfig)

		actor, err := RunSpecFromString(runSpecConfig.Spec)
		if err != nil {
			usage.ReportErrorAndExit(actor, fmt.Sprintf("error running spec: %v", err), 1)
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
