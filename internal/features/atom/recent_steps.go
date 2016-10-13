package atom

import (
	log "github.com/Sirupsen/logrus"
	. "github.com/gucumber/gucumber"
	"github.com/stretchr/testify/assert"
	atomdata "github.com/xtracdev/es-atom-data"
	atompub "github.com/xtracdev/es-atom-pub"
	"github.com/xtracdev/goes"
	"github.com/xtracdev/orapub"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"golang.org/x/tools/blog/atom"
	"encoding/xml"
	"fmt"
	"encoding/base64"
	"time"
)

func init() {
	var initFailed bool
	var atomProcessor orapub.EventProcessor

	log.Info("Init test envionment")
	_, db, err := initializeEnvironment()
	if err != nil {
		log.Warnf("Failed environment init: %s", err.Error())
		initFailed = true
	}

	var feedData []byte
	var feed atom.Feed
	var cacheControl string


	Given(`^some events not yet assigned to a feed$`, func() {
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
	})

	And(`^no feeds exist$`, func() {
		//Get this for free given one event created above
	})

	When(`^I retrieve the recent resource$`, func() {
		//Create a test server
		recentHandler, err := atompub.NewRecentHandler(db, "server:12345")
		if !assert.Nil(T, err) {
			return
		}

		ts := httptest.NewServer(http.HandlerFunc(recentHandler))
		defer ts.Close()

		res, err := http.Get(ts.URL)
		if !assert.Nil(T, err) {
			return
		}

		var readErr error
		feedData, readErr = ioutil.ReadAll(res.Body)
		res.Body.Close()
		cacheControl = res.Header.Get("Cache-Control")
		assert.Nil(T, readErr)

		assert.True(T, len(feedData) > 0, "Empty feed data returned unexpectedly")

		err = xml.Unmarshal(feedData,&feed)
		assert.Nil(T,err)
	})

	Then(`^the events not yet assigned to a feed are returned$`, func() {
		if assert.Equal(T, len(feed.Entry), 1) {
			feedEntry := feed.Entry[0]
			assert.Equal(T, "event", feedEntry.Title)
			assert.Equal(T, fmt.Sprintf("urn:esid:%s:%d", "agg1", 1),feedEntry.ID)
			assert.Equal(T, "foo", feedEntry.Content.Type)
			assert.Equal(T, base64.StdEncoding.EncodeToString([]byte("ok")), feedEntry.Content.Body)
			_,err = time.Parse(time.RFC3339Nano, string(feedEntry.Published))
			assert.Nil(T,err)
		}
	})

	And(`^there is no previous link relationship$`, func() {
		assert.Nil(T, getLink("prev-archive", &feed))
	})

	And(`^there is no next link relationship$`, func() {
		assert.Nil(T, getLink("next-archive", &feed))
	})

	And(`^cache headers indicate the resource is not cacheable$`, func() {
		assert.Equal(T, cacheControl, "no-store")
	})
}
