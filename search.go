package go_splunk_rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	log "golang.org/x/exp/slog"
)

const DEFAULT_MAX_COUNT = 10000
const SEARCH_WAIT = 5
const TIME_FORMAT = "01/02/06:15:04:05"
const PARTITION_COUNT = 5

// hold options that can be passed to a search job
// more details can be found here:
// https://docs.splunk.com/Documentation/Splunk/9.1.0/RESTREF/RESTsearch#search.2Fjobs
type SearchOptions struct {
	// max records, defaults to DEFAULT_MAX_COUNT
	MaxCount int

	// Sets the earliest (inclusive), respectively, time bounds for the search.
	// use time format %m/%d/%Y:%H:%M:%S
	UseEarliestTime bool
	EarliestTime    time.Time
	// Sets the latest (exclusive), respectively, time bounds for the search.
	// use time format %m/%d/%Y:%H:%M:%S
	UseLatestTime bool
	LatestTime    time.Time

	// In the Search function ; for searches which hit the maxCount,
	// to recursively create new searches on reduced time ranges
	// (by using shrinking earliest and latest time fields)
	// and combine the results at the end
	AllowPartition bool
}

type SearchJobStatus struct {
	Messages []struct {
		Type    string `json:"type"`
		Message string `json:"text"`
	}
	Entry []struct {
		Content struct {
			IsDone   bool `json:"isDone"`
			IsFailed bool `json:"isFailed"`
		} `json:"content"`
	} `json:"entry"`
}

func (s SearchJobStatus) IsDone() (bool, error) {
	if len(s.Entry) > 0 {
		if s.Entry[0].Content.IsDone && !s.Entry[0].Content.IsFailed {
			return true, nil
		}

		if s.Entry[0].Content.IsFailed {
			errorMsg := ""
			for _, e := range s.Messages {
				errorMsg = fmt.Sprintf("%s: %s\n", e.Type, e.Message)
			}
			return true, fmt.Errorf("%s", errorMsg)
		}
	}

	return false, nil
}

func (c Connection) SearchJobCreate(searchQuery string, searchOptions SearchOptions) (string, error) {
	data := make(url.Values)
	data.Add("search", searchQuery)
	data.Add("output_mode", "json")

	if searchOptions.MaxCount == 0 {
		searchOptions.MaxCount = DEFAULT_MAX_COUNT
	}

	data.Add("max_count", fmt.Sprintf("%d", searchOptions.MaxCount))

	if searchOptions.UseEarliestTime {
		data.Add("earliest_time", searchOptions.EarliestTime.Format(TIME_FORMAT))
	}

	if searchOptions.UseLatestTime {
		data.Add("latest_time", searchOptions.LatestTime.Format(TIME_FORMAT))
	}

	headers := map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	resp, respCode, err := c.httpCall("POST", "/services/search/jobs", headers, []byte(data.Encode()))
	if err != nil || respCode != http.StatusCreated {
		return "", fmt.Errorf("unable to create search job %s %d %s", err, respCode, string(resp))
	}

	respStruct := struct {
		Sid string `json:"sid"`
	}{}
	if err = json.Unmarshal(resp, &respStruct); err != nil {
		return "", fmt.Errorf("unable to parse sid from splunk: %s | response: %s", err, string(resp))
	}

	return respStruct.Sid, nil
}

func (c Connection) SearchJobStatus(jobID string) (SearchJobStatus, error) {
	data := make(url.Values)
	data.Add("output_mode", "json")

	resp, respCode, err := c.httpCall("GET", fmt.Sprintf("/services/search/jobs/%s", jobID), map[string]string{}, []byte(data.Encode()))
	if err != nil || respCode != http.StatusOK {
		return SearchJobStatus{}, fmt.Errorf("unable to create search job %s", err)
	}

	var respStruct SearchJobStatus
	if err = json.Unmarshal(resp, &respStruct); err != nil {
		return SearchJobStatus{}, fmt.Errorf("unable to parse sid from splunk: %s | response: %s", err, string(resp))
	}

	return respStruct, nil
}

func (c Connection) SearchJobResults(jobID string) ([]map[string]interface{}, error) {
	data := make(url.Values)
	data.Add("output_mode", "json")

	resp, respCode, err := c.httpCall("GET", fmt.Sprintf("/services/search/jobs/%s/results", jobID), map[string]string{}, []byte(data.Encode()))
	if err != nil || respCode != http.StatusOK {
		return []map[string]interface{}{}, fmt.Errorf("unable to create search job %s", err)
	}

	respStruct := struct {
		Results []map[string]interface{} `json:"results"`
	}{}
	if err = json.Unmarshal(resp, &respStruct); err != nil {
		return []map[string]interface{}{}, fmt.Errorf("unable to parse sid from splunk: %s | response: %s", err, string(resp))
	}

	return respStruct.Results, nil
}

// Blocking Search function
// this will queue a search job, and wait in SEARCH_WAIT increments to check
// search-job status, and then return the result records
func (c Connection) Search(searchQuery string, searchOptions SearchOptions) ([]map[string]interface{}, error) {

	sid, err := c.SearchJobCreate(searchQuery, searchOptions)
	if err != nil {
		return []map[string]interface{}{}, err
	}

	waiting := true
	for waiting {
		jobStatus, err := c.SearchJobStatus(sid)
		if err != nil {
			return []map[string]interface{}{}, err
		}

		isDone, err := jobStatus.IsDone()
		if err != nil {
			return []map[string]interface{}{}, err
		}

		if isDone {
			waiting = false
			break
		}

		time.Sleep(SEARCH_WAIT * time.Second)
	}

	results, err := c.SearchJobResults(sid)
	if err != nil {
		return []map[string]interface{}{}, err
	}

	if len(results) == searchOptions.MaxCount {

		log.Warn("number of records returned equal to max count")
		if searchOptions.AllowPartition &&
			searchOptions.UseEarliestTime &&
			searchOptions.UseLatestTime {
			// max count of returned results
			// partition the search time range
			d := (searchOptions.LatestTime.Sub(searchOptions.EarliestTime).Seconds()) / PARTITION_COUNT

			startT := searchOptions.EarliestTime
			endT := searchOptions.EarliestTime

			var wg sync.WaitGroup

			partitionedResults := make(map[int][]map[string]interface{})
			partitionedErr := make(map[int]error)
			for i := 0; i < PARTITION_COUNT; i++ {
				endT = startT.Add(time.Duration(d) * time.Second)

				partitionSearchOptions := searchOptions

				partitionSearchOptions.LatestTime = startT
				partitionSearchOptions.EarliestTime = endT

				wg.Add(1)
				go func() {
					defer wg.Done()

					rec, err := c.Search(searchQuery, partitionSearchOptions)
					partitionedErr[i] = err
					partitionedResults[i] = rec
				}()

				startT = endT
			}

			// wait for partitioned searches to be completed
			wg.Wait()

			results = make([]map[string]interface{}, 0, PARTITION_COUNT*searchOptions.MaxCount)
			for idx, res := range partitionedResults {
				if partitionedErr[idx] != nil {
					return results, partitionedErr[idx]
				}

				results = append(results, res...)
			}
		}
	}

	return results, nil
}

// Stub function making it easier to search in an Async fashion as a goroutine
func (c Connection) SearchAndExec(searchQuery string, searchOptions SearchOptions,
	onSuccess func([]map[string]interface{}) error,
	onError func(error),
) {
	results, err := c.Search(searchQuery, searchOptions)
	if err != nil {
		onError(err)
		return
	}

	err = onSuccess(results)
	if err != nil {
		onError(err)
		return
	}
}
