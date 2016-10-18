# Event Store Atom Publisher

[![CircleCI](https://circleci.com/gh/xtracdev/es-atom-pub.svg?style=svg)](https://circleci.com/gh/xtracdev/es-atom-pub)

This project provides an atom feed of published events from the oraeventstore.

## Overview

For an event store, this project provides an atom feed of published events. The
events are organized into feeds based on a number of events per feed,
as processed by the [es-atom-data](https://github.com/xtracdev/es-atom-data)
project.

The most recent events are not associated with a feed, these may be
retrieved via the /notifications/recent resource. Events associated with
a feed may be retrieved via the /notifications/{feedid} resource.

Additionally, events may be retrieved individually via
/notifications/{aggregate_id}/{version}

Based on semantics associated with event stores (immutable events), 
cache headers are returned for feed pages and entities indicating
they may be cached for 30 days. The recent page is denoted as uncacheable
as new events may be added to it up the point it is archived by
associating the events with a specific feed id.