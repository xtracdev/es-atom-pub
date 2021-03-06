package atompubsvc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/armon/go-metrics"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/gorilla/mux"
	atomdata "github.com/xtracdev/es-atom-data"
	"golang.org/x/tools/blog/atom"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

var ErrBadDBConnection = errors.New("Nil db passed to factory method")

//URIs assumed by handlers - these are fixed as they embed references relative to the URIs
//used in this package
const (
	PingURI                = "/ping"
	RecentHandlerURI       = "/notifications/recent"
	ArchiveHandlerURI      = "/notifications/{feedId}"
	RetrieveEventHanderURI = "/events/{aggregateId}/{version}"
	KeyAliasRoot           = "alias/"
	KeyAlias               = "KEY_ALIAS"
	LinkProto	       = "LINK_PROTO"
)

//Used to serialize event store content when directly retrieving using aggregate id and version
type EventStoreContent struct {
	XMLName     xml.Name  `xml:"http://github.com/xtracdev/goes event"`
	AggregateId string    `xml:"aggregateId"`
	Version     int       `xml:"version"`
	Published   time.Time `xml:"published"`
	TypeCode    string    `xml:"typecode"`
	Content     string    `xml:"content"`
}

//KMS service
var kmsSvc *kms.KMS

//Link proto - http or https
var linkProto string

func CheckKMSConfig() error {
	keyAlias := KeyAliasRoot + os.Getenv(KeyAlias)
	if keyAlias == KeyAliasRoot {
		return nil
	}

	params := &kms.GenerateDataKeyInput{
		KeyId:   aws.String(keyAlias), // Required
		KeySpec: aws.String("AES_256"),
	}

	_, err := kmsSvc.GenerateDataKey(params)
	return err
}

func init() {
	keyAlias := KeyAliasRoot + os.Getenv(KeyAlias)

	if keyAlias != "" {
		log.Infof("Key alias specified: %s", keyAlias)
		log.Infof("AWS_REGION: %s", os.Getenv("AWS_REGION"))
		log.Infof("AWS_PROFILE: %s", os.Getenv("AWS_PROFILE"))

		sess, err := session.NewSession()
		if err == nil {
			kmsSvc = kms.New(sess)

			err = CheckKMSConfig()
			if err != nil {
				log.Errorf("Error instantiating AWS session: %s. Exiting.", err.Error())
				os.Exit(1)
			}
		} else {
			log.Infof("Error instantiating AWS session: %s. Exiting.", err.Error())
			os.Exit(1)
		}

	}

	linkProto = os.Getenv(LinkProto)
	if linkProto == "" {
		log.Infof("No %s from the environment - defaulting to https", LinkProto)
		linkProto = "https"
	}

}

//Encrypt from cryptopasta commit bc3a108a5776376aa811eea34b93383837994340
//used via the CC0 license. See https://github.com/gtank/cryptopasta
func Encrypt(plaintext []byte, key *[32]byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

//Add the retrieved events for a given feed to the atom feed structure
func addItemsToFeed(feed *atom.Feed, events []atomdata.TimestampedEvent, linkhostport, proto string) {

	for _, event := range events {

		encodedPayload := base64.StdEncoding.EncodeToString(event.Payload.([]byte))

		content := &atom.Text{
			Type: event.TypeCode,
			Body: encodedPayload,
		}

		entry := &atom.Entry{
			Title:     "event",
			ID:        fmt.Sprintf("urn:esid:%s:%d", event.Source, event.Version),
			Published: atom.TimeStr(event.Timestamp.Format(time.RFC3339Nano)),
			Content:   content,
		}

		link := atom.Link{
			Rel:  "self",
			Href: fmt.Sprintf("%s://%s/events/%s/%d", proto, linkhostport, event.Source, event.Version),
		}

		entry.Link = append(entry.Link, link)

		feed.Entry = append(feed.Entry, entry)

	}

}

//Configure where telemery data does. Currently this can be send via UDP to a listener, or can be buffered
//internally and dumped via a signal.
func ConfigureStatsD() {
	statsdEndpoint := os.Getenv("STATSD_ENDPOINT")
	log.Infof("STATSD_ENDPOINT: %s", statsdEndpoint)

	if statsdEndpoint != "" {

		log.Info("Using vanilla statsd client to send telemetry to ", statsdEndpoint)
		sink, err := metrics.NewStatsdSink(statsdEndpoint)
		if err != nil {
			log.Warn("Unable to configure statds sink", err.Error())
			return
		}
		metrics.NewGlobal(metrics.DefaultConfig(statsdEndpoint), sink)
	} else {
		log.Info("Using in memory metrics accumulator - dump via USR1 signal")
		inm := metrics.NewInmemSink(10*time.Second, 5*time.Minute)
		metrics.DefaultInmemSignal(inm)
		metrics.NewGlobal(metrics.DefaultConfig("xavi"), inm)
	}
}

//Update counters and stats for timings, discriminating errors from non-errors
func logTimingStats(svc string, start time.Time, err error) {
	duration := time.Now().Sub(start)
	go func(svc string, duration time.Duration, err error) {
		ms := float32(duration.Nanoseconds()) / 1000.0 / 1000.0
		if err != nil {
			key := []string{"es-atom-pub", fmt.Sprintf("%s-error", svc)}
			metrics.AddSample(key, float32(ms))
			metrics.IncrCounter(key, 1)
		} else {
			key := []string{"es-atom-pub", svc}
			metrics.AddSample(key, float32(ms))
			metrics.IncrCounter(key, 1)
		}
	}(svc, duration, err)
}

//Encrypt output encrypts the output is indicated by the configuration settings, e.g.
//KEY_ALIAS set to something. Here we obtain the encryption key from KMS, and append the
//encrypted version of the key to the encoded output.
func encryptOutput(svc *kms.KMS, out []byte) ([]byte, error) {
	keyAlias := KeyAliasRoot + os.Getenv(KeyAlias)
	if keyAlias == KeyAliasRoot {
		return out, nil
	}

	//Get the encryption keys
	params := &kms.GenerateDataKeyInput{
		KeyId:   aws.String(keyAlias), // Required
		KeySpec: aws.String("AES_256"),
	}

	resp, err := svc.GenerateDataKey(params)
	if err != nil {
		return nil, err
	}

	key := [32]byte{}
	copy(key[:], resp.Plaintext[0:32])

	//Encrypt the output
	encrypted, err := Encrypt(out, &key)
	if err != nil {
		return nil, err
	}

	//Purge the key from memory
	key = [32]byte{}
	resp.Plaintext = nil

	//Encode the output
	encodedOut := base64.StdEncoding.EncodeToString(encrypted)

	//Encode the encryptedKey - this will have to be decrypted using the KMS
	//CMK before the payload can be decrypted with it
	encodedKey := base64.StdEncoding.EncodeToString(resp.CiphertextBlob)

	keyPlusText := fmt.Sprintf("%s::%s", encodedKey, encodedOut)

	return []byte(keyPlusText), nil
}

//NewRecentHandler instantiates the handler for retrieve recent notifications, which are those that have not
//yet been assigned a feed id. This will be served up at /notifications/recent
//The linkhostport argument is used to set the host and port in the link relations URL. This is useful
//when proxying the feed, in which case the link relation URLs can reflect the proxied URLs, not the
//direct URL.
func NewRecentHandler(db *sql.DB, linkhostport string) (func(rw http.ResponseWriter, req *http.Request), error) {
	if db == nil {
		return nil, ErrBadDBConnection
	}

	return func(rw http.ResponseWriter, req *http.Request) {
		svc := "notifications-recent"
		start := time.Now()
		events, err := atomdata.RetrieveRecent(db)
		if err != nil {
			logTimingStats(svc, start, err)
			log.Warnf("Error retrieving recent items: %s", err.Error())
			http.Error(rw, "Error retrieving feed items", http.StatusInternalServerError)
			return
		}

		latestFeed, err := atomdata.RetrieveLastFeed(db)
		if err != nil {
			logTimingStats(svc, start, err)
			log.Warnf("Error retrieving last feed id: %s", err.Error())
			http.Error(rw, "Error retrieving feed id", http.StatusInternalServerError)
			return
		}

		feed := atom.Feed{
			Title:   "Event store feed",
			ID:      "recent",
			Updated: atom.TimeStr(time.Now().Format(time.RFC3339)),
		}

		self := atom.Link{
			Href: fmt.Sprintf("%s://%s/notifications/recent", linkProto, linkhostport),
			Rel:  "self",
		}

		via := atom.Link{
			Href: fmt.Sprintf("%s://%s/notifications/recent", linkProto, linkhostport),
			Rel:  "related",
		}

		feed.Link = append(feed.Link, self)
		feed.Link = append(feed.Link, via)

		if latestFeed != "" {
			previous := atom.Link{
				Href: fmt.Sprintf("%s://%s/notifications/%s", linkProto, linkhostport, latestFeed),
				Rel:  "prev-archive",
			}
			feed.Link = append(feed.Link, previous)
		}

		addItemsToFeed(&feed, events, linkhostport, linkProto)

		out, err := xml.Marshal(&feed)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			logTimingStats(svc, start, err)
			return
		}

		encodedOut, err := encryptOutput(kmsSvc, out)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			logTimingStats(svc, start, err)
			return
		}

		rw.Header().Add("Cache-Control", "no-store")
		rw.Header().Add("Content-Type", "application/atom+xml")
		rw.Write(encodedOut)
		logTimingStats(svc, start, nil)
	}, nil
}

//NewArchiveHandler instantiates a handler for retrieving feed archives, which is a set of events
//associated with a specific feed id. This will be served up at /notifications/{feedId}
//The linkhostport argument is used to set the host and port in the link relations URL. This is useful
//when proxying the feed, in which case the link relation URLs can reflect the proxied URLs, not the
//direct URL.
func NewArchiveHandler(db *sql.DB, linkhostport string) (func(rw http.ResponseWriter, req *http.Request), error) {
	if db == nil {
		return nil, ErrBadDBConnection
	}

	return func(rw http.ResponseWriter, req *http.Request) {
		svc := "notifications-archive"
		start := time.Now()
		feedID := mux.Vars(req)["feedId"]
		if feedID == "" {
			logTimingStats(svc, start, errors.New("no feed in uri"))
			http.Error(rw, "No feed id in uri", http.StatusBadRequest)
			return
		}

		log.Infof("processing request for feed %s", feedID)

		//Retrieve events for the given feed id.
		latestFeed, err := atomdata.RetrieveArchive(db, feedID)
		if err != nil {
			logTimingStats(svc, start, err)
			log.Warnf("Error retrieving last feed id: %s", err.Error())
			http.Error(rw, "Error retrieving feed id", http.StatusInternalServerError)
			return
		}

		//Did we get any events? We should not have a feed other than recent with no events, therefore
		//if there are no events then the feed id does not exist.
		if len(latestFeed) == 0 {
			logTimingStats(svc, start, nil)
			log.Infof("No data found for feed %s", feedID)
			http.Error(rw, "", http.StatusNotFound)
			return
		}

		previousFeed, err := atomdata.RetrievePreviousFeed(db, feedID)
		if err != nil {
			logTimingStats(svc, start, err)
			log.Warnf("Error retrieving previous feed id: %s", err.Error())
			http.Error(rw, "Error retrieving previous feed id", http.StatusInternalServerError)
			return
		}

		nextFeed, err := atomdata.RetrieveNextFeed(db, feedID)
		if err != nil {
			logTimingStats(svc, start, err)
			log.Warnf("Error retrieving next feed id: %s", err.Error())
			http.Error(rw, "Error retrieving next feed id", http.StatusInternalServerError)
			return
		}

		feed := atom.Feed{
			Title: "Event store feed",
			ID:    feedID,
		}

		self := atom.Link{
			Href: fmt.Sprintf("%s://%s/notifications/%s", linkProto, linkhostport, feedID),
			Rel:  "self",
		}

		feed.Link = append(feed.Link, self)

		if previousFeed.Valid {
			feed.Link = append(feed.Link, atom.Link{
				Href: fmt.Sprintf("%s://%s/notifications/%s", linkProto, linkhostport, previousFeed.String),
				Rel:  "prev-archive",
			})
		}

		var next string
		if (nextFeed.Valid == true && nextFeed.String == "") || !nextFeed.Valid {
			next = "recent"
		} else {
			next = nextFeed.String
		}

		feed.Link = append(feed.Link, atom.Link{
			Href: fmt.Sprintf("%s://%s/notifications/%s", linkProto, linkhostport, next),
			Rel:  "next-archive",
		})

		addItemsToFeed(&feed, latestFeed, linkhostport, linkProto)

		out, err := xml.Marshal(&feed)
		if err != nil {
			logTimingStats(svc, start, err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		encodedOut, err := encryptOutput(kmsSvc, out)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			logTimingStats(svc, start, err)
			return
		}

		//For all feeds except recent, we can indicate the page can be cached for a long time,
		//e.g. 30 days. The recent page is mutable so we don't indicate caching for it. We could
		//potentially attempt to load it from this method via link traversal.
		if feedID != "recent" {
			log.Infof("setting Cache-Control max-age=2592000 for ETag %s", feedID)
			rw.Header().Add("Cache-Control", "max-age=2592000") //Contents are immutable, cache for a month
			rw.Header().Add("ETag", feedID)
		} else {
			rw.Header().Add("Cache-Control", "no-store")
		}

		rw.Header().Add("Content-Type", "application/atom+xml")
		rw.Write(encodedOut)

		logTimingStats(svc, start, nil)

	}, nil
}

//NewRetrieveHandler instantiates a handler for the retrieval of specific events by aggregate id
//and version. This will be served at /notifications/{aggregateId}/{version}
func NewEventRetrieveHandler(db *sql.DB) (func(rw http.ResponseWriter, req *http.Request), error) {
	if db == nil {
		return nil, ErrBadDBConnection
	}

	return func(rw http.ResponseWriter, req *http.Request) {
		svc := "retrieve-event"
		start := time.Now()
		aggregateID := mux.Vars(req)["aggregateId"]
		versionParam := mux.Vars(req)["version"]

		log.Infof("Retrieving event %s %s", aggregateID, versionParam)

		version, err := strconv.Atoi(versionParam)
		if err != nil {
			logTimingStats(svc, start, err)
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

		event, err := atomdata.RetrieveEvent(db, aggregateID, version)
		if err != nil {
			logTimingStats(svc, start, err)
			switch err {
			case sql.ErrNoRows:
				http.Error(rw, "", http.StatusNotFound)
			default:
				log.Warnf("Error retrieving event: %s", err.Error())
				http.Error(rw, "Error retrieving event", http.StatusInternalServerError)
			}

			return
		}

		eventContent := EventStoreContent{
			AggregateId: aggregateID,
			Version:     version,
			TypeCode:    event.TypeCode,
			Published:   event.Timestamp,
			Content:     base64.StdEncoding.EncodeToString(event.Payload.([]byte)),
		}

		marshalled, err := xml.Marshal(&eventContent)
		if err != nil {
			logTimingStats(svc, start, err)
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		encodedOut, err := encryptOutput(kmsSvc, marshalled)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			logTimingStats(svc, start, err)
			return
		}

		rw.Header().Add("Content-Type", "application/xml")
		rw.Header().Add("ETag", fmt.Sprintf("%s:%d", aggregateID, version))
		rw.Header().Add("Cache-Control", "max-age=2592000")

		rw.Write(encodedOut)
		logTimingStats(svc, start, nil)

	}, nil
}

func PingHandler(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusOK)
}
