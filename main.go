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
	if !present {
		log.Fatalf("No required environment variable: %s", key)
		return ""
	}
	return value
}

func newRequest(method, path string, payload interface{}) *http.Request {
	reqBody := bytes.NewBuffer(nil)
	// Create a request specific headers map.
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
	// Set the request specific headers.
	for k, v := range reqHeaders {
		req.Header[k] = v
	}
	return req
}

func doHealthCheck(containerID *string) error {
	// TODO: use ping endpoint
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
		return fmt.Errorf("Healthcheck error: %s", resp.Status)
	}
	log.Println("Container is ready for use")
	return nil
}

func doCreate() error {
	log.Println("Creating revizor container...")
	req := newRequest("POST", "/api/containers/", &map[string]interface{}{
		"skip_ui": true,
	})
	client := &http.Client{Timeout: 600 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, _ := ioutil.ReadAll(resp.Body)
	var cont container
	err = json.Unmarshal(respBody, &cont)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Created container %s", cont.ID)
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
	return fmt.Errorf("Container %s is unavilable", cont.ID)
}

func doDelete(containerID string) error {
	log.Println("Deleting revizor container...")
	req := newRequest("DELETE", fmt.Sprintf("/api/containers/%s/", containerID), nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 202 {
		return fmt.Errorf("Unable to delete the container, status code %d", resp.StatusCode)
	}
	log.Printf("Container %s successfully deleted", containerID)
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
		err := doDelete(flag.Arg(1))
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("No such command: %s", cmd)
	}

}
