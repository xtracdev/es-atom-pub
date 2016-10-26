package main

import (
	"fmt"
	"github.com/xtracdev/tlsconfig"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) < 5 {
		fmt.Println("Need four args: client key file, client cert file, ca cert file, emdpoint")
		os.Exit(1)
	}

	log.Println("get tls config")
	config, err := tlsconfig.GetTLSConfiguration(os.Args[1], os.Args[2], os.Args[3])
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("create client with tls transport config")
	tr := &http.Transport{
		TLSClientConfig: config,
	}

	client := http.Client{Transport: tr}

	log.Println("create request")
	req, err := http.NewRequest("GET", os.Args[4], nil)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Println("Get /")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err.Error())
	}

	defer resp.Body.Close()

	log.Println("Read response")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err.Error())
	}

	log.Printf("status: %d", resp.StatusCode)
	log.Printf("response: %s", string(body))

}
