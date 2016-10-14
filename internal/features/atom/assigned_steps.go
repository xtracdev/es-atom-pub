package atom

import (
	. "github.com/gucumber/gucumber"
	"github.com/xtracdev/orapub"
	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	atomdata "github.com/xtracdev/es-atom-data"
	atompub "github.com/xtracdev/es-atom-pub"
	"os"
	"github.com/xtracdev/goes"
	"net/http/httptest"
	"net/http"
	"github.com/gorilla/mux"
	"fmt"
	"encoding/xml"
	"golang.org/x/tools/blog/atom"
	"io/ioutil"
)

func init() {
	var initFailed bool
	var atomProcessor orapub.EventProcessor
	var feedData []byte
	var feed atom.Feed

	log.Info("Init test envionment")
	_, db, err := initializeEnvironment()
	if err != nil {
		log.Warnf("Failed environment init: %s", err.Error())
		initFailed = true
	}

	Given(`^a single feed with events assigned to it$`, func() {
		log.Info("check init")
		if initFailed {
			assert.False(T, initFailed, "Test env init failure")
			return
		}

		atomProcessor = atomdata.NewESAtomPubProcessor()
		err := atomProcessor.Initialize(db)
		assert.Nil(T, err, "Failed to initialize atom publisher")

		log.Info("clean out tables")
		_, err = db.Exec("delete from atom_event")
		assert.Nil(T, err)
		_, err = db.Exec("delete from feed")
		assert.Nil(T, err)

		os.Setenv("FEED_THRESHOLD", "2")
		atomdata.ReadFeedThresholdFromEnv()
		assert.Equal(T, 2, atomdata.FeedThreshold)

		log.Info("add some events")
		eventPtr := &goes.Event{
			Source:   "agg1",
			Version:  1,
			TypeCode: "foo",
			Payload:  []byte("ok"),
		}

		err = atomProcessor.Processor(db, eventPtr)
		assert.Nil(T, err)

		eventPtr = &goes.Event{
			Source:   "agg2",
			Version:  1,
			TypeCode: "bar",
			Payload:  []byte("ok ok"),
		}

		err = atomProcessor.Processor(db, eventPtr)
		assert.Nil(T, err)

	})

	When(`^I do a get on the feed resource id$`, func() {
		feedid,err := atomdata.RetrieveLastFeed(db)
		assert.Nil(T,err)
		log.Infof("get feed it %s",feedid)

		archiveHandler, err := atompub.NewArchiveHandler(db, "server:12345")
		if !assert.Nil(T, err) {
			return
		}

		router := mux.NewRouter()
		router.HandleFunc("/notifications/{feedid}", archiveHandler)

		r,err := http.NewRequest("GET", fmt.Sprintf("/notifications/%s", feedid), nil)
		assert.Nil(T,err)
		w := httptest.NewRecorder()

		router.ServeHTTP(w,r)
		assert.Equal(T, http.StatusOK,w.Result().StatusCode)

		var readErr error
		feedData, readErr = ioutil.ReadAll(w.Body)
		assert.Nil(T, readErr)

		assert.True(T, len(feedData) > 0, "Empty feed data returned unexpectedly")

	})

	Then(`^all the events associated with the feed are returned$`, func() {
		err = xml.Unmarshal(feedData, &feed)
		if assert.Nil(T, err) {
			assert.Equal(T, 2, len(feed.Entry))
		}

	})

	And(`^there is no previous feed link relationship$`, func() {
		prev := getLink("prev-archive", &feed)
		assert.Nil(T,prev)
	})

	And(`^the next link relationship is recent$`, func() {
		next := getLink("next-archive", &feed)
		if assert.NotNil(T,next) {
			assert.Equal(T, "http://server:12345/notifications/recent", *next)
		}
	})
}
