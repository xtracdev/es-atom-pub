package main

import (
	"database/sql"
	"github.com/alecthomas/kingpin"
	"github.com/gorilla/mux"
	atompub "github.com/xtracdev/es-atom-pub"
	"github.com/xtracdev/oraconn"
	"log"
	"net/http"
	"os"
)

var app = kingpin.New("Atompub", "Atom feed of the oraeventstore")
var linkhost = app.Flag("linkhost", "Base host:port for feed links (useful when proxying)").Required().String()

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	config, err := oraconn.NewEnvConfig()
	if err != nil {
		log.Fatalf("Missing environment configuration: %s", err.Error())
	}

	db, err := sql.Open("oci8", config.ConnectString())
	if err != nil {
		log.Fatal(err.Error())
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err.Error())
	}

	recentHandler, err := atompub.NewRecentHandler(db, *linkhost)
	if err != nil {
		log.Fatal(err.Error())
	}

	archiveHandler, err := atompub.NewArchiveHandler(db, *linkhost)
	if err != nil {
		log.Fatal(err.Error())
	}

	retrieveHandler, err := atompub.NewEventRetrieveHandler(db, *linkhost)
	if err != nil {
		log.Fatal(err.Error())
	}

	r := mux.NewRouter()
	r.HandleFunc("/notifications/recent", recentHandler)
	r.HandleFunc("/notifications/{feedid}", archiveHandler)
	r.HandleFunc("/notifications/{aggregate_id}/{version}", retrieveHandler)

	srv := &http.Server{
		Handler: r,
		Addr:    ":8000",
	}

	log.Fatal(srv.ListenAndServe())
}
