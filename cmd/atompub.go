package main

import (
	"database/sql"
	"github.com/gorilla/mux"
	atompub "github.com/xtracdev/es-atom-pub"
	"github.com/xtracdev/oraconn"
	"log"
	"net/http"
	"os"
)


type AtomFeedPubConfig struct {
	linkhost string
	listenerHostAndPort string
}

func NewAtomFeedPubConfig() *AtomFeedPubConfig {
	var configErr bool
	config := new(AtomFeedPubConfig)
	config.linkhost = os.Getenv("LINKHOST")
	if config.linkhost == "" {
		log.Println("Missing LINKHOST environment variable value")
		configErr = true
	}

	config.listenerHostAndPort = os.Getenv("LISTENADDR")
	if config.listenerHostAndPort == "" {
		log.Println("Missing LISTENADDR environment variable value")
		configErr = true
	}

	if configErr {
		log.Fatal("Error reading configuration from environment")
	}

	return config
}

func main() {

	//Read atom pub config
	feedConfig := NewAtomFeedPubConfig()

	//Read db connection config
	config, err := oraconn.NewEnvConfig()
	if err != nil {
		log.Fatalf("Missing environment configuration: %s", err.Error())
	}

	//Connect to DB
	db, err := sql.Open("oci8", config.ConnectString())
	if err != nil {
		log.Fatal(err.Error())
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err.Error())
	}

	//Create handlers
	recentHandler, err := atompub.NewRecentHandler(db, feedConfig.linkhost)
	if err != nil {
		log.Fatal(err.Error())
	}

	archiveHandler, err := atompub.NewArchiveHandler(db, feedConfig.linkhost)
	if err != nil {
		log.Fatal(err.Error())
	}

	retrieveHandler, err := atompub.NewEventRetrieveHandler(db)
	if err != nil {
		log.Fatal(err.Error())
	}

	r := mux.NewRouter()
	r.HandleFunc("/notifications/recent", recentHandler)
	r.HandleFunc("/notifications/{feedid}", archiveHandler)
	r.HandleFunc("/notifications/{aggregate_id}/{version}", retrieveHandler)

	//Config server
	srv := &http.Server{
		Handler: r,
		Addr:    feedConfig.listenerHostAndPort,
	}

	//Listen up...
	log.Fatal(srv.ListenAndServe())
}
