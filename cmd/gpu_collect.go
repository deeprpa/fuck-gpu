package cmd

import (
	"fmt"

	"github.com/deeprpa/fuck-gpu/pkgs/gpucollect"
	"github.com/spf13/cobra"
	"github.com/ygpkg/yg-go/logs"
)

func init() {
	rootCmd.AddCommand(gpuCollectCmd())
}

func gpuCollectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gpu-collect",
		Aliases: []string{"gc"},
		Short:   "collect gpu memory",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			gpus, err := gpucollect.GetNvidiaGPUMemory()
			if err != nil {
				logs.ErrorContextf(ctx, "get gpu memory failed, %s", err)
				return
			}

			for _, gpu := range gpus {
				fmt.Printf("gpu %d: %s, total: %s, free: %s, used: %s\n", gpu.Index, gpu.Name, gpu.MemoryTotal, gpu.MemoryFree, gpu.MemoryUsed)
			}

		},
	}
	return cmd
}
