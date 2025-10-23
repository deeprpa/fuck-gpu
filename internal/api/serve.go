package api

import (
	"net"
	"net/http"
	"time"

	"github.com/deeprpa/fuck-gpu/internal/daemon"
	"github.com/gin-gonic/gin"
)

func ListenAndServe(l net.Listener, d *daemon.Daemon) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.GET("ping", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "hi %s", time.Now())
	})
	router.GET("status", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, d.Status())
	})
	router.GET("exit_spare", func(ctx *gin.Context) {
		d.App().ExitSpare()
		ctx.Writer.WriteHeader(http.StatusNoContent)
	})
	router.GET("restart", func(ctx *gin.Context) {
		if err := d.App().Restart(); err != nil {
			ctx.Writer.WriteString(err.Error())
			ctx.Writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		ctx.Writer.WriteHeader(http.StatusNoContent)
	})

	http.Serve(l, router)
}
