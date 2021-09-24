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

	if *fDebug {
		log.SetLevel(log.DebugLevel)
	} else if *fInfo {
		log.SetLevel(log.InfoLevel)
	}
	// log = logger.Init("log", *fDebug, false, ioutil.Discard)
	// logger.SetFlags(logger.S)
	startMsg := "Starting"
	if *fDebug {
		startMsg = startMsg + " in debug mode"
	}
	log.Info(startMsg)
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.StaticFile("/favicon.ico", "./assets/favicon.ico")
	router.Static("/assets", "./assets")
	r := New("/home/fsf/go/src/fsf/tx/state.json")
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
		r.Update(&update)
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
	signal.Notify(quit, syscall.SIGINT)
	<-quit
	r.Halt()

	// Three second grace period before forceful teardown
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
