package atompubsvc

import (
	"database/sql"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	atomdata "github.com/xtracdev/es-atom-data"
	"golang.org/x/tools/blog/atom"
	"net/http"
	"time"
)

var ErrBadDBConnection = errors.New("Nil db passed to factory method")
var ErrBytePayloadsOnly = errors.New("Only []byte payloads are supported for atom feed")

func addItemsToFeed(feed *atom.Feed, events []atomdata.TimestampedEvent, linkhostport string) error {

	for _, event := range events {

		payload, ok := event.Payload.([]byte)
		if !ok {
			return ErrBytePayloadsOnly
		}

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

	return nil
}

func NewRecentHandler(db *sql.DB, linkhostport string) (func(rw http.ResponseWriter, req *http.Request), error) {
	if db == nil {
		return nil, ErrBadDBConnection
	}

	return func(rw http.ResponseWriter, req *http.Request) {
		events, err := atomdata.RetrieveRecent(db)
		if err != nil {
			log.Warnf("Error retrieving recent items", err.Error())
			http.Error(rw, "Error retrieving feed items", http.StatusInternalServerError)
			return
		}

		latestFeed, err := atomdata.RetrieveLastFeed(db)
		if err != nil {
			log.Warnf("Error retrieving last feed id", err.Error())
			http.Error(rw, "Error retrieving feed id", http.StatusInternalServerError)
			return
		}

		feed := atom.Feed{
			Title:   "Event store feed",
			ID:      "recent",
			Updated: atom.TimeStr(time.Now().Truncate(time.Hour).Format(time.RFC3339)),
		}

		self := atom.Link{
			Href: fmt.Sprintf("http://%s/notifications/recent", linkhostport),
			Rel:  "self",
		}

		via := atom.Link{
			Href: fmt.Sprintf("http://%s/notifications/recent", linkhostport),
			Rel:  "related",
		}

		previous := atom.Link{
			Href: fmt.Sprintf("http://%s/notifications/%s", linkhostport, latestFeed),
			Rel:  "prev-archive",
		}

		feed.Link = append(feed.Link, self)
		feed.Link = append(feed.Link, via)
		feed.Link = append(feed.Link, previous)

		err = addItemsToFeed(&feed, events, latestFeed)
		if err != nil {
			log.Warnf("Error building feed items: %s", err.Error())
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

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
