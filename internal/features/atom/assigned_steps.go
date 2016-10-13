package atom

import (
	. "github.com/gucumber/gucumber"
	"github.com/xtracdev/orapub"
	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	atomdata "github.com/xtracdev/es-atom-data"
	"os"
	"github.com/xtracdev/goes"
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
	})
}
