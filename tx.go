package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	log "github.com/sirupsen/logrus"
)

var (
	// log    *logger.Logger
	fInfo  = flag.Bool("info", false, "Display INFO messages")
	fDebug = flag.Bool("debug", false, "Dispay DEBUG messages")
	fNoTx  = flag.Bool("no-tx", false, "Use a dummy command rather than broadcasting")
)

func main() {
	flag.Parse()
	log.SetLevel(log.WarnLevel)
	log.SetReportCaller(true)
	mode := gin.ReleaseMode

	if *fDebug {
		log.SetLevel(log.DebugLevel)
		mode = gin.DebugMode
	} else if *fInfo {
		log.SetLevel(log.InfoLevel)
	}

	gin.SetMode(mode)
	router := gin.New()
	router.LoadHTMLGlob("templates/*")
	router.StaticFile("/favicon.ico", "./assets/favicon.ico")
	router.Static("/assets", "./assets")

	ctx, cancel := context.WithCancel(context.Background())
	var disp RadioDisplay
	disp, err := NewOLEDDisplay()

	if err != nil {
		log.Error("Using null display")
		disp = new(NullDisplay)
	}
	defer disp.Close()
	r := NewRadio(ctx, "/home/fsf/go/src/fsf/tx/state.json", disp)

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"on":          r.State.On,
			"callsign":    r.State.Dial.Selected,
			"txFrequency": r.State.TxFrequency,
		})
	})

	router.GET("/radio", func(c *gin.Context) {
		c.JSON(200, r.State)
	})

	router.POST("/radio", func(c *gin.Context) {
		var update RadioState
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		r.Update(ctx, &update)
		c.JSON(200, r.State)
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %s", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()

	// Three second grace period before forceful teardown
	timeout, teardown := context.WithTimeout(context.Background(), 3*time.Second)
	defer teardown()
	if err := srv.Shutdown(timeout); err != nil {
		log.Fatal("Server forced to terminate: ", err)
	}

	log.Println("Server exiting")
}
