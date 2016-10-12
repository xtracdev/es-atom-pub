package atom

import (
	. "github.com/gucumber/gucumber"
	log "github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/xtracdev/orapub"
	"os"
	ad "github.com/xtracdev/es-atom-data"
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


	Given(`^some events not yet assigned to a feed$`, func() {
		log.Info("check init")
		if initFailed {
			assert.False(T, initFailed, "Test env init failure")
			return
		}

		atomProcessor = ad.NewESAtomPubProcessor()
		err := atomProcessor.Initialize(db)
		assert.Nil(T, err, "Failed to initialize atom publisher")

		log.Info("clean out tables")
		_, err = db.Exec("delete from atom_event")
		assert.Nil(T, err)
		_, err = db.Exec("delete from feed")
		assert.Nil(T, err)

		os.Setenv("FEED_THRESHOLD", "2")
		ad.ReadFeedThresholdFromEnv()
		assert.Equal(T, 2, ad.FeedThreshold)

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
}