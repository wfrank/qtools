package qradar

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

type ReferenceSet struct {
	Name             string `json:"name"`
	ElementType      string `json:"element_type"`
	TimeoutType      string `json:"timeout_type"`
	TimeToLive       string `json:"time_to_live"`
	CreationTime     int    `json:"creation_time,omitempty"`
	NumberOfElements int    `json:"number_of_elements,omitempty"`
}

type DeletionTask struct {
	ID        int    `json:"id"`
	Created   int    `json:"created"`
	Started   int    `json:"started"`
	Modified  int    `json:"modified"`
	Completed int    `json:"completed"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	CreatedBy string `json:"created_by"`
}

type Client struct {
	BaseURL    string
	SecToken   string
	httpClient *http.Client
}

func NewClient(baseURL string, secToken string) *Client {

	return &Client{
		BaseURL:  baseURL,
		SecToken: secToken,
		httpClient: &http.Client{
			Timeout: time.Second * 30,
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout: 3 * time.Second,
				}).Dial,
				TLSHandshakeTimeout: 5 * time.Second,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}
}

func (c *Client) ReferenceSets() ([]ReferenceSet, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/api/reference_data/sets", nil)
	req.Header.Add("Version", "9.1")
	req.Header.Add("SEC", c.SecToken)
	req.Header.Add("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending http request: %v", err)
	}
	defer resp.Body.Close()

	var refSets []ReferenceSet
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&refSets); err != nil {
		return nil, fmt.Errorf("error decoding response body: %v", err)
	}

	return refSets, nil
}

func (c *Client) CreateReferenceSet(name string) (*ReferenceSet, error) {
	url := fmt.Sprintf("%s/api/reference_data/sets?name=%s&element_type=IP", c.BaseURL, url.QueryEscape(name))
	req, err := http.NewRequest("POST", url, nil)
	req.Header.Add("Version", "9.1")
	req.Header.Add("SEC", c.SecToken)
	req.Header.Add("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending http request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return nil, fmt.Errorf("unexpected response, code: %d, body: %b", resp.StatusCode, body)
	}

	refSet := new(ReferenceSet)
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(refSet); err != nil {
		return nil, fmt.Errorf("error decoding response body: %v", err)
	}
	return refSet, nil
}

func (c *Client) BulkLoadReferenceSet(name string, servers []string) (*ReferenceSet, error) {
	url := fmt.Sprintf("%s/api/reference_data/sets/bulk_load/%s", c.BaseURL, url.QueryEscape(name))

	body, err := json.Marshal(servers)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Add("Version", "9.1")
	req.Header.Add("SEC", c.SecToken)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending http request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		io.Copy(os.Stdout, resp.Body)
		return nil, fmt.Errorf("unexpected response, code: %d, body: %b", resp.StatusCode, body)
	}

	refSet := new(ReferenceSet)
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(refSet); err != nil {
		return nil, fmt.Errorf("error decoding response body: %v", err)
	}
	return refSet, nil
}

func (c *Client) DeleteReferenceSet(name string, purgeOnly bool) (*DeletionTask, error) {
	url := fmt.Sprintf("%s/api/reference_data/sets/%s?purge_only=%v",
		c.BaseURL, url.QueryEscape(name), purgeOnly)

	req, err := http.NewRequest("DELETE", url, nil)
	req.Header.Add("Version", "9.1")
	req.Header.Add("SEC", c.SecToken)
	req.Header.Add("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending http request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 202 {
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return nil, fmt.Errorf("unexpected response, code: %d, body: %b", resp.StatusCode, body)
	}

	var task DeletionTask
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&task); err != nil {
		return nil, fmt.Errorf("error decoding response body: %v", err)
	}
	return &task, nil
}

func (c *Client) DeleteReferenceSetTaskStatus(taskID int) (*DeletionTask, error) {
	url := fmt.Sprintf("%s/api/reference_data/set_delete_tasks/%d", c.BaseURL, taskID)

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Version", "9.1")
	req.Header.Add("SEC", c.SecToken)
	req.Header.Add("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending http request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var body bytes.Buffer
		io.Copy(&body, resp.Body)
		return nil, fmt.Errorf("unexpected response, code: %d, body: %b", resp.StatusCode, body)
	}

	var task DeletionTask
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&task); err != nil {
		return nil, fmt.Errorf("error decoding response body: %v", err)
	}
	return &task, nil
}
