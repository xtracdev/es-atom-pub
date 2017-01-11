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

## Health check inspection

To troubleshoot the container health check, use docker inspect, e.g.

<pre>
docker inspect --format "{{json .State.Health }}" container-name
</pre>

For example: `docker inspect --format "{{json .State.Health }}" devcenter_nginxproxy_1`


## Contributing

To contribute, you must certify you agree with the [Developer Certificate of Origin](http://developercertificate.org/)
by signing your commits via `git -s`. To create a signature, configure your user name and email address in git.
Sign with your real name, do not use pseudonyms or submit anonymous commits.


In terms of workflow:

0. For significant changes or improvement, create an issue before commencing work.
1. Fork the respository, and create a branch for your edits.
2. Add tests that cover your changes, unit tests for smaller changes, acceptance test
for more significant functionality.
3. Run gofmt on each file you change before committing your changes.
4. Run golint on each file you change before committing your changes.
5. Make sure all the tests pass before committing your changes.
6. Commit your changes and issue a pull request.

## License

(c) 2016 Fidelity Investments
Licensed under the Apache License, Version 2.0