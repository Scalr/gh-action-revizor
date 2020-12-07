package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type container struct {
	ID string `json:"container_id"`
}

const teBaseURL = "test-env.scalr.com"

var (
	revizorBaseURL = getEnv("REVIZOR_URL")
	revizorToken   = getEnv("REVIZOR_TOKEN")
	scalrToken     = getEnv("SCALR_TOKEN")
)

func getEnv(key string) string {
	value, present := os.LookupEnv(key)
	if !present || len(value) == 0 {
		log.Fatalf("No required environment variable: %s", key)
		return ""
	}
	return value
}

func newRequest(method, path string, payload interface{}) *http.Request {
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

func doHealthCheck(containerID *string) error {
	// TODO: use ping endpoint within stable profile
	url := fmt.Sprintf("https://%s.%s/api/iacp/v3/environments", *containerID, teBaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", scalrToken))
	req.Header.Set("Prefer", "profile=preview")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return fmt.Errorf("The healthcheck error: %s", resp.Status)
	}
	log.Printf("The container %s is ready for use", *containerID)
	return nil
}

func doCreate() error {
	log.Println("Creating the container...")
	createOptions := make(map[string]interface{})
	// Provider tests do not require an UI,
	// so we can speed up the container creation.
	createOptions["skip_ui"] = true

	// Setup revizor container branches
	branch := os.Getenv("BRANCH")
	apiBranch := os.Getenv("API_BRANCH")
	dbBranch := os.Getenv("DB_BRANCH")
	if len(apiBranch) != 0 {
		b, err := strconv.ParseBool(apiBranch)
		if err != nil {
			log.Fatal("Cannot parse API_BRANCH value")
		}
		if b {
			createOptions["fatmouse_branch"] = branch
			log.Printf("The container will be created from %s API branch", branch)
		}
	}
	if len(dbBranch) != 0 {
		b, err := strconv.ParseBool(dbBranch)
		if err != nil {
			log.Fatal("Cannot parse DB_BRANCH value")
		}
		if b {
			createOptions["scalr_branch"] = branch
			log.Printf("The container will be created from %s DB branch", branch)
		}
	}
	req := newRequest("POST", "/api/containers/", &createOptions)
	// Such a large timeout due to the fact that sometimes
	// the result of creating a container cannot be obtained
	// for a long time on the server side.
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != 201 {
		log.Fatalf("Cannot create the container: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	var cont container
	err = json.Unmarshal(respBody, &cont)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("The container %s has been created", cont.ID)
	// set-output: GitHub Action mechanism that sets the output parameter.
	fmt.Printf("::set-output name=container_id::%s\n", cont.ID)
	fmt.Printf("::set-output name=hostname::%s.%s\n", cont.ID, teBaseURL)
	for i := 1; i <= 10; i++ {
		err := doHealthCheck(&cont.ID)
		if err != nil {
			log.Println(err)
			time.Sleep(1 * time.Second)
		} else {
			return nil
		}
	}
	err = doDelete(cont.ID)
	if err != nil {
		log.Fatal(err)
	}
	return fmt.Errorf("The container %s was unavailable and was deleted", cont.ID)
}

func doDelete(containerID string) error {
	log.Printf("Deleting the container %s...", containerID)
	req := newRequest("DELETE", fmt.Sprintf("/api/containers/%s/", containerID), nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 202 {
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
		err := doCreate()
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
