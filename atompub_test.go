package atompubsvc

import (
	"encoding/base64"
	"encoding/xml"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetrieveEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	ts := time.Now()
	rows := sqlmock.NewRows([]string{"event_time", "typecode", "payload"}).AddRow(
		ts, "foo", []byte("yeah ok"))

	mock.ExpectQuery("select event_time").WillReturnRows(rows)

	eventHandler, err := NewEventRetrieveHandler(db, "myhost:12345")
	assert.Nil(t, err)

	router := mux.NewRouter()
	router.HandleFunc("/notifications/{aggregate_id}/{version}", eventHandler)

	r, err := http.NewRequest("GET", "/notifications/1234567/1", nil)
	assert.Nil(t, err)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, r)

	//Validate status code
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)

	//Read the response
	eventData, err := ioutil.ReadAll(w.Body)
	assert.Nil(t, err)

	//Validate content
	var event EventStoreContent
	err = xml.Unmarshal(eventData, &event)
	if assert.Nil(t, err) {
		assert.Equal(t, "1234567", event.AggregateId)
		assert.Equal(t, "foo", event.TypeCode)
		assert.Equal(t, 1, event.Version)
		assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("yeah ok")), event.Content)
		assert.Equal(t, ts, event.Published)
	}

	//Validate cache headers
	cc := w.Header().Get("Cache-Control")
	assert.Equal(t, "max-age=2592000", cc)

	etag := w.Header().Get("ETag")
	assert.Equal(t, "1234567:1", etag)

	//Validate cache headers

}
