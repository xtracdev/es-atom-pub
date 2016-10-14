package atom

import (
	"encoding/xml"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	. "github.com/gucumber/gucumber"
	"github.com/stretchr/testify/assert"
	atomdata "github.com/xtracdev/es-atom-data"
	atompub "github.com/xtracdev/es-atom-pub"
	"github.com/xtracdev/goes"
	"github.com/xtracdev/orapub"
	"golang.org/x/tools/blog/atom"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
)

func init() {
	var initFailed bool
	var atomProcessor orapub.EventProcessor
	var feedData []byte
	var feedID string
	var feed atom.Feed
	var cacheControl string
	var etag string

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
		var err error
		feedID, err = atomdata.RetrieveLastFeed(db)
		assert.Nil(T, err)
		log.Infof("get feed it %s", feedID)

		archiveHandler, err := atompub.NewArchiveHandler(db, "server:12345")
		if !assert.Nil(T, err) {
			return
		}

		router := mux.NewRouter()
		router.HandleFunc("/notifications/{feedid}", archiveHandler)

		r, err := http.NewRequest("GET", fmt.Sprintf("/notifications/%s", feedID), nil)
		assert.Nil(T, err)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, r)
		assert.Equal(T, http.StatusOK, w.Result().StatusCode)

		cacheControl = w.Header().Get("Cache-Control")
		etag = w.Header().Get("ETag")

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
		assert.Nil(T, prev)
	})

	And(`^the next link relationship is recent$`, func() {
		next := getLink("next-archive", &feed)
		if assert.NotNil(T, next) {
			assert.Equal(T, "http://server:12345/notifications/recent", *next)
		}
	})

	And(`^cache headers indicate the resource is cacheable$`, func() {
		if assert.True(T, cacheControl != "") {
			cc := strings.Split(cacheControl, "=")
			if assert.Equal(T, 2, len(cc)) {
				assert.Equal(T, "max-age", cc[0])
				assert.Equal(T, fmt.Sprintf("%d", 30*24*60*60), cc[1])
			}
		}

		assert.Equal(T, feedID, etag)
	})
}
