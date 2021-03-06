package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/cloudfoundry-community/go-cfenv"
	"github.com/garyburd/redigo/redis"
)

func checkStatus(err error) {
	if err != nil {
		log.Fatalf("error: %s", err.Error())
	}
}

func writeError(w http.ResponseWriter, err error) {
	log.Printf("There was an error, %s\n", err.Error())
	message, _ := json.Marshal(map[string]string{
		"error": err.Error(),
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	w.Write(message)
}

func newConnection() redis.Conn {
	// Get redis32-multinode credentials
	env, _ := cfenv.Current()
	services, _ := env.Services.WithLabel("redis32")
	if len(services) != 1 {
		log.Fatal("redis32 service not found")
	}
	creds := services[0].Credentials
	newClient, err := redis.Dial("tcp", fmt.Sprintf("%s:%s", creds["hostname"], creds["port"]))
	checkStatus(err)

	_, err = newClient.Do("AUTH", creds["password"].(string))
	checkStatus(err)

	return newClient
}

func testSetGetDelete(w http.ResponseWriter, r *http.Request) {
	client := newConnection()
	defer client.Close()

	// Set and check value
	_, err := client.Do("SET", "test", "test")
	if err != nil {
		writeError(w, err)
		return
	}

	value, err := redis.String(client.Do("GET", "test"))
	if err != nil {
		writeError(w, err)
		return
	}
	if value != "test" {
		writeError(w, fmt.Errorf("incorrect value: %s", value))
		return
	}

	_, err = client.Do("DEL", "test")
	if err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func info(w http.ResponseWriter, r *http.Request) {
	client := newConnection()
	defer client.Close()

	parameter := r.URL.Query().Get("s")

	if parameter == "" {
		parameter = "all"
	}

	infoString, err := redis.String(client.Do("INFO", parameter))

	if err != nil {
		writeError(w, err)
		return
	}

	infoMap := make(map[string]string)

	for _, line := range strings.Split(infoString, "\r\n") {
		part := strings.Split(line, ":")
		if len(part) == 2 {
			infoMap[part[0]] = part[1]
		}
	}

	jresp, err := json.Marshal(infoMap)

	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jresp)

}

func configGet(w http.ResponseWriter, r *http.Request) {
	client := newConnection()
	defer client.Close()

	parameter := r.URL.Query().Get("p")

	if parameter == "" {
		parameter = "*"
	}

	primaryConfig, err := redis.StringMap(client.Do("CONFIG", "GET", parameter))

	if err != nil {
		writeError(w, err)
		return
	}

	jresp, err := json.Marshal(primaryConfig)

	if err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(jresp)

}

func main() {
	// Serve HTTP
	http.HandleFunc("/", testSetGetDelete)
	http.HandleFunc("/config-get", configGet)
	http.HandleFunc("/info", info)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("PORT")), nil))
}
