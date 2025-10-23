package cmd

import (
	"net"

	"github.com/deeprpa/fuck-gpu/internal/api"
	"github.com/deeprpa/fuck-gpu/internal/daemon"
	"github.com/deeprpa/fuck-gpu/pkgs/logs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/ygpkg/yg-go/lifecycle"
)

func init() {
	rootCmd.AddCommand(daemonCmd())
}
func daemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "daemon",
		Aliases: []string{"d"},
		Short:   "start daemon",
		PreRunE: loadConfigCmd,
		Run: func(cmd *cobra.Command, args []string) {
			lc := lifecycle.New()
			d, err := daemon.NewDaemon(lc, cfg)
			if err != nil {
				logrus.Error("start daemon failed, %s", err)
				return
			}
			if err := d.Run(); err != nil {
				logrus.Error("daemon run failed, %s", err)
				return
			}

			{
				l, err := net.Listen("tcp", api.ServeAddr)
				if err != nil {
					logrus.Errorf("start internal API server failed, %s", err)
				} else {
					lc.AddCloser(l)
					go api.ListenAndServe(l, d)
				}
			}
			lc.WaitExit()
			logs.DebugContextf(ctx, "exit 0;")
		},
	}
	return cmd
}
