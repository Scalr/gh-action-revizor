package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type container struct {
	ID string `json:"container_id"`
}

type createOptions struct {
	// Provider tests do not require an UI,
	// so we can speed up the container creation.
	SkipUI         bool   `json:"skip_ui"`
	FatmouseBranch string `json:"fatmouse_branch,omitempty"`
	ScalrBranch    string `json:"scalr_branch,omitempty"`
	Notes          string `json:"notes"`
	Python         bool   `json:"update_python"`
}

const (
	teBaseURL = "test-env.scalr.com"
	// Such a large timeout due to the fact that sometimes
	// the result of creating a container cannot be obtained
	// for a long time on the server side.
	createTimeout         = 800 * time.Second
	healthCheckMaxRetries = 30
	healthCheckRetryDelay = 3 * time.Second
)

var (
	revizorBaseURL = getEnv("REVIZOR_URL")
	revizorToken   = getEnv("REVIZOR_TOKEN")
	scalrToken     = getEnv("SCALR_TOKEN")
	upstreamID     = getEnv("UPSTREAM_ID")
)

func getEnv(key string) string {
	value, present := os.LookupEnv(key)
	if !present || len(value) == 0 {
		log.Fatalf("No required environment variable: %s", key)
		return ""
	}
	return value
}

func newRequest(method, path string, payload *createOptions) *http.Request {
	reqBody := bytes.NewBuffer(nil)
	reqHeaders := make(http.Header)
	reqHeaders.Set("Authorization", fmt.Sprintf("Token %s", revizorToken))
	if payload != nil {
		reqHeaders.Set("Content-Type", "application/json")
		jsonEncoded, err := json.Marshal(payload)
		if err != nil {
			log.Fatal(err)
		}
		reqBody.Write(jsonEncoded)
	}
	req, err := http.NewRequest(method, revizorBaseURL+path, reqBody)
	if err != nil {
		log.Fatal(err)
	}
	for k, v := range reqHeaders {
		req.Header[k] = v
	}
	return req
}

type HealthCheckError struct {
	StatusCode int
	Err        error
}

func (r *HealthCheckError) Error() string {
	return fmt.Sprintf("Health check errror. Status %d: %v", r.StatusCode, r.Err)
}

func doHealthCheck(containerID *string) error {
	url := fmt.Sprintf("https://%s.%s/api/iacp/v3/ping", *containerID, teBaseURL)
	req, err := http.NewRequest("GET", url, nil)
	statuses := [2]int{200, 204}
	var ok bool = false

	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", scalrToken))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return &HealthCheckError{
			Err: fmt.Errorf("ping %s %v", url, err),
		}
	}

	for _, x := range statuses {
		if resp.StatusCode == x {
			ok = true
			break
		}
	}
	if !ok {
		return &HealthCheckError{
			Err:        fmt.Errorf("ping %s not successful", url),
			StatusCode: resp.StatusCode,
		}
	}

	log.Printf("The container %s is ready for use", *containerID)
	return nil
}

func newCreateOptions() *createOptions {
	options := &createOptions{
		SkipUI: true,
		Notes:  fmt.Sprintf("Provider upstream #%s", upstreamID),
	}
	// Setup revizor container branches
	apiBranch := os.Getenv("API_BRANCH")
	dbBranch := os.Getenv("DB_BRANCH")
	python := os.Getenv("PY_REQ")
	if len(apiBranch) != 0 {
		options.FatmouseBranch = apiBranch
	}
	if len(dbBranch) != 0 {
		options.ScalrBranch = dbBranch
	}
	options.Python = python == "yes"
	return options
}

func setOutputsfromCreate(cont *container) {
	// set-output: GitHub Action mechanism that sets the output parameter.
	fmt.Printf("::set-output name=container_id::%s\n", cont.ID)
	fmt.Printf("::set-output name=hostname::%s.%s\n", cont.ID, teBaseURL)
}

func doCreate(options *createOptions, retry bool) error {
	optionsJSON, _ := json.Marshal(options)
	log.Printf("Creating the container with options %s", string(optionsJSON))

	req := newRequest("POST", "/api/containers/", options)
	client := &http.Client{Timeout: createTimeout}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		log.Fatalf("Cannot create the container due to %v", err)
	}
	if resp.StatusCode != 201 {
		log.Fatalf("Cannot create the container. Error status code: %d", resp.StatusCode)
	}
	if resp.Body == nil {
		log.Fatal("Invalid response. The response body is empty")
	}
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	var cont container
	err = json.Unmarshal(respBody, &cont)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("The container %s has been created", cont.ID)
	setOutputsfromCreate(&cont)

	var healthCheckErr *HealthCheckError
	for i := 1; i <= healthCheckMaxRetries; i++ {
		err := doHealthCheck(&cont.ID)
		if err != nil {
			healthCheckErr = err.(*HealthCheckError)
			log.Println(err)
			time.Sleep(healthCheckRetryDelay)
		} else {
			return nil
		}
	}
	err = doDelete(cont.ID)
	if err != nil {
		log.Fatal(err)
	}
	// For unknown reasons, sometimes we can't find the container we created,
	// so we try again to create a container.
	if retry && healthCheckErr.StatusCode == 404 {
		return doCreate(options, false)
	} else {
		return errors.New("Cannot create container")
	}
}

func doDelete(containerID string) error {
	log.Printf("Deleting the container %s...", containerID)
	req := newRequest("DELETE", fmt.Sprintf("/api/containers/%s/", containerID), nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp == nil {
		return fmt.Errorf("Failed to delete the container due to %v", err)
	}
	if resp.StatusCode != 202 {
		return fmt.Errorf("Failed to delete the container, status code %d", resp.StatusCode)
	}
	log.Printf("The container %s was successfully deleted", containerID)
	return nil
}

func main() {
	flag.Parse()
	cmd := flag.Arg(0)

	switch cmd {
	case "create":
		err := doCreate(newCreateOptions(), true)
		if err != nil {
			log.Fatal(err)
		}
	case "delete":
		containerID := flag.Arg(1)
		if len(containerID) == 0 {
			log.Fatal("The container ID not specified")
		}
		err := doDelete(containerID)
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("No such command: %s", cmd)
	}

}
