package cmd

import (
	"github.com/deeprpa/fuck-gpu/pkgs/logs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	debug   bool
	showVer bool
	cfgFile string
	cfg     *config.GlobalConfig
	rootCmd = &cobra.Command{
		Use:   "ecdn-edge-supervisor",
		Short: "Process life cycle management",
		PersistentPreRun: func(ctx *cobra.Command, args []string) {
			if debug {
				logs.SetLevel(logs.DebugLevel)
			}
		},
		Run: func(ctx *cobra.Command, args []string) {
			if debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			if showVer {
				printVersion()
			}
		},
	}
)

// Execute .
func Execute() {
	rootCmd.Flags().BoolVarP(&showVer, "version", "v", false, "Print version information")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "D", false, "Print debug information")
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./config/example.yaml", "Configuration file path")

	// rootCmd.AddCommand(tmpCmd())
	rootCmd.Execute()
}

func loadConfigCmd(ctx *cobra.Command, args []string) (err error) {
	cfg, err = config.LoadConfig(cfgFile)
	if err != nil {
		logrus.Fatalf("load config file from %s failed, %s", cfgFile, err)
		return err
	}
	switch cfg.LogConfig.LogLevel {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	}
	if cfg.LogConfig.File != nil {
		logrus.SetOutput(cfg.LogConfig.File)
	}

	return nil
}

func tmpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tmp",
		PreRunE: loadConfigCmd,
		Run: func(ctx *cobra.Command, args []string) {
			lf := cfg.LogConfig.File

			logrus.SetOutput(lf)

			logrus.Errorf("hihi")
			logrus.Debug("hihi")

		},
	}

	return cmd
}
