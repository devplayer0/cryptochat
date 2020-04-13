package main

import (
	"flag"
	"os"
	"os/signal"

	"github.com/devplayer0/cryptochat/pkg/server"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func main() {
	dbPath := flag.String("db", "data.db", "path to sqlite database file")
	addr := flag.String("addr", ":9443", "api listen address")
	flag.Parse()

	srv, err := server.NewServer(*dbPath)
	if err != nil {
		log.WithError(err).Fatal("Failed to start server")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGINT, unix.SIGTERM)

	go func() {
		log.Info("Starting server...")
		if err := srv.Listen(*addr); err != nil {
			log.WithError(err).Fatal("Failed to start server")
		}
	}()

	<-sigs
	log.Info("Shutting down...")
	srv.Close()
}