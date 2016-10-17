package atompubsvc

import (
	"database/sql"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	atomdata "github.com/xtracdev/es-atom-data"
	"golang.org/x/tools/blog/atom"
	"net/http"
	"strconv"
	"time"
)

var ErrBadDBConnection = errors.New("Nil db passed to factory method")

type EventStoreContent struct {
	XMLName     xml.Name  `xml:"http://github.com/xtracdev/goes event"`
	AggregateId string    `xml:"aggregateId"`
	Version     int       `xml:"version"`
	Published   time.Time `xml:"published"`
	TypeCode    string    `xml:"typecode"`
	Content     string    `xml:"content"`
}

func addItemsToFeed(feed *atom.Feed, events []atomdata.TimestampedEvent, linkhostport string) {

	for _, event := range events {

		//Here we can type assert without a check because the event array passed to this method
		//was scanned from a driver.Value, which constrings the types that can be scanned and
		//can convert those into []byte (or error out on the rows.Scan
		payload := event.Payload.([]byte)

		encodedPayload := base64.StdEncoding.EncodeToString(payload)

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
			Href: fmt.Sprintf("http://%s/notifications/%s/%d", linkhostport, event.Source, event.Version),
		}

		entry.Link = append(entry.Link, link)

		feed.Entry = append(feed.Entry, entry)

	}
}

func NewRecentHandler(db *sql.DB, linkhostport string) (func(rw http.ResponseWriter, req *http.Request), error) {
	if db == nil {
		return nil, ErrBadDBConnection
	}

	return func(rw http.ResponseWriter, req *http.Request) {
		events, err := atomdata.RetrieveRecent(db)
		if err != nil {
			log.Warnf("Error retrieving recent items: %s", err.Error())
			http.Error(rw, "Error retrieving feed items", http.StatusInternalServerError)
			return
		}

		latestFeed, err := atomdata.RetrieveLastFeed(db)
		if err != nil {
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
			Href: fmt.Sprintf("http://%s/notifications/recent", linkhostport),
			Rel:  "self",
		}

		via := atom.Link{
			Href: fmt.Sprintf("http://%s/notifications/recent", linkhostport),
			Rel:  "related",
		}

		feed.Link = append(feed.Link, self)
		feed.Link = append(feed.Link, via)

		if latestFeed != "" {
			previous := atom.Link{
				Href: fmt.Sprintf("http://%s/notifications/%s", linkhostport, latestFeed),
				Rel:  "prev-archive",
			}
			feed.Link = append(feed.Link, previous)
		}

		addItemsToFeed(&feed, events, latestFeed)

		out, err := xml.Marshal(&feed)
		if err != nil {
			log.Warnf("Error marshalling feed data: %s", err.Error())
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		rw.Header().Add("Cache-Control", "no-store")
		rw.Header().Add("Content-Type", "application/atom+xml")
		rw.Write(out)
	}, nil
}

func NewArchiveHandler(db *sql.DB, linkhostport string) (func(rw http.ResponseWriter, req *http.Request), error) {
	if db == nil {
		return nil, ErrBadDBConnection
	}

	return func(rw http.ResponseWriter, req *http.Request) {
		feedid := mux.Vars(req)["feedid"]
		if feedid == "" {
			http.Error(rw, "No feed id in uri", http.StatusInternalServerError)
			return
		}

		log.Infof("processing request for feed %s", feedid)

		latestFeed, err := atomdata.RetrieveArchive(db, feedid)
		if err != nil {
			log.Warnf("Error retrieving last feed id: %s", err.Error())
			http.Error(rw, "Error retrieving feed id", http.StatusInternalServerError)
			return
		}

		previousFeed, err := atomdata.RetrievePreviousFeed(db, feedid)
		if err != nil {
			log.Warnf("Error retrieving previous feed id: %s", err.Error())
			http.Error(rw, "Error retrieving previous feed id", http.StatusInternalServerError)
			return
		}

		nextFeed, err := atomdata.RetrieveNextFeed(db, feedid)
		if err != nil {
			log.Warnf("Error retrieving next feed id: %s", err.Error())
			http.Error(rw, "Error retrieving next feed id", http.StatusInternalServerError)
			return
		}

		feed := atom.Feed{
			Title: "Event store feed",
			ID:    feedid,
		}

		self := atom.Link{
			Href: fmt.Sprintf("http://%s/notifications/%s", linkhostport, feedid),
			Rel:  "self",
		}

		feed.Link = append(feed.Link, self)

		if previousFeed.Valid {
			feed.Link = append(feed.Link, atom.Link{
				Href: fmt.Sprintf("http://%s/notifications/%s", linkhostport, previousFeed.String),
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
			Href: fmt.Sprintf("http://%s/notifications/%s", linkhostport, next),
			Rel:  "next-archive",
		})

		addItemsToFeed(&feed, latestFeed, linkhostport)

		out, err := xml.Marshal(&feed)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		//For all feeds except recent, we can indicate the page can be cached for a long time,
		//e.g. 30 days. The recent page is mutable so we don't indicate caching for it. We could
		//potentially attempt to load it from this method via link traversal.
		if feedid != "recent" {
			log.Infof("setting Cache-Control max-age=2592000 for ETag %s", feedid)
			rw.Header().Add("Cache-Control", "max-age=2592000") //Contents are immutable, cache for a month
			rw.Header().Add("ETag", feedid)
		} else {
			rw.Header().Add("Cache-Control", "no-store")
		}

		rw.Header().Add("Content-Type", "application/atom+xml")
		rw.Write(out)

	}, nil
}

func NewEventRetrieveHandler(db *sql.DB, linkhostport string) (func(rw http.ResponseWriter, req *http.Request), error) {
	if db == nil {
		return nil, ErrBadDBConnection
	}

	return func(rw http.ResponseWriter, req *http.Request) {
		aggregateId := mux.Vars(req)["aggregate_id"]
		versionParam := mux.Vars(req)["version"]

		log.Infof("Retrieving event %s %s", aggregateId, versionParam)

		version, err := strconv.Atoi(versionParam)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}

		event, err := atomdata.RetrieveEvent(db, aggregateId, version)
		if err != nil {
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
			AggregateId: aggregateId,
			Version:     version,
			TypeCode:    event.TypeCode,
			Published:   event.Timestamp,
			Content:     base64.StdEncoding.EncodeToString(event.Payload.([]byte)),
		}

		marshalled, err := xml.Marshal(&eventContent)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		rw.Header().Add("Content-Type", "application/xml")
		rw.Header().Add("ETag", fmt.Sprintf("%s:%d", aggregateId, version))
		rw.Header().Add("Cache-Control", "max-age=2592000")

		rw.Write(marshalled)

	}, nil
}
