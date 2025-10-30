package cmd

import (
	"context"
	"fmt"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/deeprpa/fuck-gpu/version"
	"github.com/spf13/cobra"
	"github.com/ygpkg/yg-go/logs"
)

var (
	showVer bool
	cfgFile string
	cfg     *config.GlobalConfig
	ctx     = context.Background()
	rootCmd = &cobra.Command{
		Use:   "fuck-gpu",
		Short: "fuck-gpu",
		// PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 	loadConfigCmd(cmd, args)
		// },
		Run: func(cmd *cobra.Command, args []string) {
			if showVer {
				printVersion()
				return
			}
		},
	}
)

// Execute .
func Execute() {
	rootCmd.Flags().BoolVarP(&showVer, "version", "v", false, "Print version information")
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./config/example.yaml", "Configuration file path")

	// rootCmd.AddCommand(tmpCmd())
	rootCmd.Execute()
}

func loadConfigCmd(cmd *cobra.Command, args []string) (err error) {
	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		logs.ErrorContextf(ctx, "load config file from %s failed, %s", cfgFile, err)
		return err
	}
	if cfg.LogConfig != nil {
		logs.InitLoggerConfig(*cfg.LogConfig)
	}
	return nil
}

func printVersion() {
	fmt.Println(version.SimpleVersion())
}
