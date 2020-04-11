package main

import (
	"os"
	"os/signal"

	"github.com/devplayer0/cryptochat/pkg/server"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func main() {
	srv := server.NewServer()
	srv.Start()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGINT, unix.SIGTERM)

	go func() {
		log.Info("Starting server...")
		if err := srv.Start(); err != nil {
			log.WithField("error", err).Fatal("Failed to start server")
		}
	}()

	<-sigs
	log.Info("Shutting down...")
	srv.Stop()
}
