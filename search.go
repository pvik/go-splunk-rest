package go_splunk_rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

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

func (c Connection) SearchJobCreate(searchQuery string) (string, error) {
	data := make(url.Values)
	data.Add("search", searchQuery)
	data.Add("output_mode", "json")

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
// this will queue a search job, and wait in 10sec increments to check
// search-job status, and then return the result records
func (c Connection) Search(searchQuery string) ([]map[string]interface{}, error) {

	sid, err := c.SearchJobCreate(searchQuery)
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

		time.Sleep(10 * time.Second)
	}

	results, err := c.SearchJobResults(sid)
	if err != nil {
		return []map[string]interface{}{}, err
	}

	return results, nil
}

// Stub function making it easier to search in an Async fashion as a goroutine
func (c Connection) SearchAndExec(searchQuery string,
	onSuccess func([]map[string]interface{}) error,
	onError func(error),
) {
	results, err := c.Search(searchQuery)
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
