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
		assert.Nil(T, readErr)

		assert.True(T, len(feedData) > 0, "Empty feed data returned unexpectedly")
	})

}
