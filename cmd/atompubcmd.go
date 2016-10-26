package main

import (
	"crypto/tls"
	"database/sql"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	atompub "github.com/xtracdev/es-atom-pub"
	"github.com/xtracdev/oraconn"
	"github.com/xtracdev/tlsconfig"
	"net/http"
	"os"
)

type AtomFeedPubConfig struct {
	linkhost            string
	listenerHostAndPort string
	secure              bool
	privateKey          string
	certfificate        string
	caCert              string
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

	//Load the TLS config from the envrionment. If INSECURE_PUBLISHER is present
	//in the environment and set to 1 we create a non secured transport, otherwise
	//if is assumed that a secure transport is desired and the rest of the
	//associated config will be present in the environment.
	insecurePublisher := os.Getenv("INSECURE_PUBLISHER")
	if insecurePublisher == "1" {
		config.secure = false
	} else {
		config.secure = true

		config.privateKey = os.Getenv("PRIVATE_KEY")
		if config.privateKey == "" {
			log.Println("Missing PRIVATE_KEY environment variable value - required for secure config")
			configErr = true
		}

		config.certfificate = os.Getenv("CERTIFICATE")
		if config.certfificate == "" {
			log.Println("Missing CERTIFICATE environment variable value - required for secure config")
			configErr = true
		}

		config.caCert = os.Getenv("CACERT")
		if config.caCert == "" {
			log.Println("Missing CACERT environment variable value - required for secure config")
			configErr = true
		}

	}

	//Finally, if there were configuration errors, we're finished as we can't start with partial or
	//malformed configuration
	if configErr {
		log.Fatal("Error reading configuration from environment")
	}

	return config
}

func main() {

	//Read atom pub config
	log.Info("Reading config from the environment")
	feedConfig := NewAtomFeedPubConfig()

	//Read db connection config
	config, err := oraconn.NewEnvConfig()
	if err != nil {
		log.Fatalf("Missing environment configuration: %s", err.Error())
	}

	//Connect to DB
	log.Info("Open SQL driver")
	db, err := sql.Open("oci8", config.ConnectString())
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Info("Ping database")
	err = db.Ping()
	if err != nil {
		log.Fatal(err.Error())
	}

	//Create handlers
	log.Info("Create and register handlers")
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

	var server *http.Server

	if feedConfig.secure {
		log.Info("Configure secure server")
		log.Info("Read key and certs; form TLC config")
		tlsConfig, err := tlsconfig.GetTLSConfiguration(
			feedConfig.privateKey,
			feedConfig.certfificate,
			feedConfig.caCert,
		)

		if err != nil {
			log.Fatal(err.Error())
		}

		log.Info("Config read, build server")

		server = &http.Server{
			Handler:      r,
			Addr:         feedConfig.listenerHostAndPort,
			TLSConfig:    tlsConfig,
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
		}

		//Listen up...
		log.Info("Start server")
		log.Fatal(server.ListenAndServeTLS(feedConfig.certfificate, feedConfig.privateKey))

	} else {
		//Config server
		log.Info("Configure non secure server")
		server = &http.Server{
			Handler: r,
			Addr:    feedConfig.listenerHostAndPort,
		}

		//Listen up...
		log.Info("Start server")
		log.Fatal(server.ListenAndServe())
	}

}
