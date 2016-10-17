package atompubsvc

import (
	"database/sql/driver"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/blog/atom"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetrieve(t *testing.T) {

	ts := time.Now()

	var retrieveTests = []struct {
		testName       string
		nilDB          bool
		aggregateId    string
		version        string
		expectedStatus int
		colNames       []string
		rowCols        []driver.Value
		queryError     error
		expectedEvent  *EventStoreContent
	}{
		{
			"retrieve no error",
			false,
			"1234567",
			"1",
			http.StatusOK,
			[]string{"event_time", "typecode", "payload"},
			[]driver.Value{ts, "foo", []byte("yeah ok")},
			nil,
			&EventStoreContent{
				AggregateId: "1234567",
				TypeCode:    "foo",
				Version:     1,
				Content:     base64.StdEncoding.EncodeToString([]byte("yeah ok")),
				Published:   ts,
			},
		},
		{
			"handler with nill db",
			true,
			"1234567",
			"1",
			http.StatusBadRequest,
			[]string{},
			[]driver.Value{},
			nil,
			nil,
		},
		{
			"retrieve with malformed version",
			false,
			"1234567",
			"x",
			http.StatusBadRequest,
			[]string{},
			[]driver.Value{},
			nil,
			nil,
		},
		{
			"retrieve with no rows found",
			false,
			"1234567",
			"1",
			http.StatusNotFound,
			[]string{"event_time", "typecode", "payload"},
			[]driver.Value{},
			nil,
			nil,
		},
		{
			"retrieve with sql error",
			false,
			"1234567",
			"1",
			http.StatusInternalServerError,
			[]string{"event_time", "typecode", "payload"},
			[]driver.Value{},
			errors.New("kaboom"),
			nil,
		},
	}

	for _, test := range retrieveTests {
		t.Run(test.testName, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()

			rows := sqlmock.NewRows(test.colNames)
			if len(test.rowCols) > 0 {
				t.Log("add test row data")
				rows = rows.AddRow(test.rowCols...)
			}

			var query *sqlmock.ExpectedQuery
			if len(test.colNames) > 0 {
				query = mock.ExpectQuery("select")
				query = query.WillReturnRows(rows)
			}

			if test.queryError != nil {
				if query == nil {
					query = mock.ExpectQuery("select")
				}
				query = query.WillReturnError(test.queryError)
			}

			var eventHandler func(http.ResponseWriter, *http.Request)
			if test.nilDB == false {
				eventHandler, err = NewEventRetrieveHandler(db, "myhost:12345")
				assert.Nil(t, err)
			} else {
				eventHandler, err = NewEventRetrieveHandler(nil, "myhost:12345")
				assert.NotNil(t, err)
				return
			}

			router := mux.NewRouter()
			router.HandleFunc("/notifications/{aggregate_id}/{version}", eventHandler)

			r, err := http.NewRequest("GET", fmt.Sprintf("/notifications/%s/%s", test.aggregateId, test.version), nil)
			assert.Nil(t, err)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			//Validate status code
			assert.Equal(t, test.expectedStatus, w.Result().StatusCode)

			if test.expectedEvent != nil {
				//Read the response
				eventData, err := ioutil.ReadAll(w.Body)
				assert.Nil(t, err)

				var event EventStoreContent
				err = xml.Unmarshal(eventData, &event)
				if assert.Nil(t, err) {
					assert.Equal(t, test.expectedEvent.AggregateId, event.AggregateId)
					assert.Equal(t, test.expectedEvent.TypeCode, event.TypeCode)
					assert.Equal(t, test.expectedEvent.Version, event.Version)
					assert.Equal(t, test.expectedEvent.Content, event.Content)
					assert.Equal(t, test.expectedEvent.Published, event.Published)
				}

				//Validate cache headers
				cc := w.Header().Get("Cache-Control")
				assert.Equal(t, "max-age=2592000", cc)

				etag := w.Header().Get("ETag")
				assert.Equal(t, "1234567:1", etag)

				//Validate content type
				assert.Equal(t, "application/xml", w.Header().Get("Content-Type"))
			}

			err = mock.ExpectationsWereMet()
			assert.Nil(t, err)
		})
	}
}

func getLink(linkRelationship string, feed *atom.Feed) *string {
	for _, l := range feed.Link {
		if l.Rel == linkRelationship {
			return &l.Href
		}
	}

	return nil
}

func TestRecentFeedHandler(t *testing.T) {

	ts := time.Now()

	var recentTests = []struct {
		testName       string
		nilDB          bool
		expectedStatus int
		colNamesEvents       []string
		rowColsEvents        []driver.Value
		eventsQueryError error
		colNamesFeed	[]string
		rowColsFeed []driver.Value
		feedQueryErr     error
		expectedPrev string
		expectedSelf string
	}{
		{
			"rectrieve recent ok",
			false,
			http.StatusOK,
			[]string{"event_time", "aggregate_id", "version", "typecode", "payload"},
			[]driver.Value{ts, "1x2x333", 3, "foo", []byte("yeah ok")},
			nil,
			[]string{"feedid"}, []driver.Value{"feed-xxx"}, nil,
			"http://testhost:12345/notifications/feed-xxx",
			"http://testhost:12345/notifications/recent",
		},
		{
			"retrieve recent events query error",
			false,
			http.StatusInternalServerError,
			[]string{"event_time", "aggregate_id", "version", "typecode", "payload"},
			[]driver.Value{},
			errors.New("kaboom"),
			[]string{}, []driver.Value{}, nil,
			"", "",
		},
		{
			"retrieve recent feed query error",
			false,
			http.StatusInternalServerError,
			[]string{"event_time", "aggregate_id", "version", "typecode", "payload"},
			[]driver.Value{ts, "1x2x333", 3, "foo", []byte("yeah ok")},
			nil,
			[]string{"feedid"}, []driver.Value{}, errors.New("kaboom"),
			"", "",
		},
		{
			"retrieve recent nil db error",
			true,
			http.StatusInternalServerError,
			[]string{}, []driver.Value{}, nil,
			[]string{}, []driver.Value{}, nil,
			"","",
		},
	}

	for _,test := range recentTests {
		t.Run(test.testName, func(t *testing.T){

			//Create mock db
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()

			//Set up rows and query for event data
			eventRows := sqlmock.NewRows(test.colNamesEvents)
			if len(test.rowColsEvents) > 0 {
				eventRows = eventRows.AddRow(test.rowColsEvents...)
			}

			var eventsQuery *sqlmock.ExpectedQuery
			if len(test.colNamesEvents) > 0 {
				eventsQuery = mock.ExpectQuery("select event_time")
				eventsQuery = eventsQuery.WillReturnRows(eventRows)
			}

			if test.eventsQueryError != nil {
				if eventsQuery == nil {
					eventsQuery = mock.ExpectQuery("select event_time")
				}
				eventsQuery = eventsQuery.WillReturnError(test.eventsQueryError)
			}

			//Set up row and query for feed data
			feedRows := sqlmock.NewRows(test.colNamesFeed)
			if len(test.rowColsFeed) > 0 {
				feedRows = feedRows.AddRow(test.rowColsFeed...)
			}

			var feedQuery *sqlmock.ExpectedQuery
			if len(test.colNamesFeed) > 0 {
				feedQuery = mock.ExpectQuery("select feedid")
				feedQuery = feedQuery.WillReturnRows(feedRows)
			}

			if test.feedQueryErr != nil {
				if feedQuery == nil {
					feedQuery = mock.ExpectQuery("select feedid")
				}
				feedQuery = feedQuery.WillReturnError(test.feedQueryErr)
			}

			//Instantiate the handler
			var eventHandler func(http.ResponseWriter,*http.Request)
			if test.nilDB == false {
				eventHandler, err = NewRecentHandler(db, "testhost:12345")
				assert.Nil(t, err)
			} else {
				eventHandler, err = NewRecentHandler(nil, "testhost:12345")
				assert.NotNil(t, err)
				return
			}

			//Set up the router, route the request
			router := mux.NewRouter()
			router.HandleFunc("/notifications/recent", eventHandler)

			r, err := http.NewRequest("GET", "/notifications/recent", nil)
			assert.Nil(t, err)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, r)

			//Check the status code
			assert.Equal(t, test.expectedStatus, w.Result().StatusCode)

			if test.expectedPrev != "" {
				eventData, err := ioutil.ReadAll(w.Body)
				assert.Nil(t, err)

				var feed atom.Feed
				err = xml.Unmarshal(eventData, &feed)
				if assert.Nil(t, err) {
					assert.Equal(t, "recent", feed.ID)
					_, err := time.Parse(time.RFC3339, string(feed.Updated))
					assert.Nil(t, err)
					prev := getLink("prev-archive", &feed)
					if assert.NotNil(t, prev) {
						assert.Equal(t, test.expectedPrev, *prev)
					}
					self := getLink("self", &feed)
					if assert.NotNil(t, self) {
						assert.Equal(t, test.expectedSelf, *self)
					}

					related := getLink("related", &feed)
					if assert.NotNil(t, related) {
						assert.Equal(t, *self, *related)
					}
				}
			}

			err = mock.ExpectationsWereMet()
			assert.Nil(t, err)
		})
	}
}
