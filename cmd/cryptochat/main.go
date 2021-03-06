package main

import (
	"flag"
	"os"
	"os/signal"

	"github.com/devplayer0/cryptochat/pkg/server"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	logLevel = flag.String("log", "info", "log level")
	dbPath   = flag.String("db", "data.db", "path to sqlite database file")
	addr     = flag.String("addr", ":0", "api listen address")
	uiAddr   = flag.String("uiaddr", "127.0.0.1:9080", "ui listen address")
)

func main() {
	flag.Parse()

	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.WithError(err).Fatal("Failed to parse log level")
	}
	log.SetLevel(level)

	srv, err := server.NewServer(*dbPath)
	if err != nil {
		log.WithError(err).Fatal("Failed to start server")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, unix.SIGINT, unix.SIGTERM)

	go func() {
		log.Info("Starting server...")
		if err := srv.Listen(*addr, *uiAddr); err != nil {
			log.WithError(err).Fatal("Failed to start server")
		}
	}()

	<-sigs
	log.Info("Shutting down...")
	srv.Close()
}
