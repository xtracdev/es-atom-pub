package atompubsvc

import (
	"database/sql/driver"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
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
		})
	}
}
