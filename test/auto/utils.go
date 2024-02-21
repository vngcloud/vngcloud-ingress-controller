package test

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
)

const SERVICE_UNAVAILABLE = "<html><body><h1>503 Service Unavailable</h1>\nNo server is available to handle this request.\n</body></html>\n"

func DoCommand(com string) {
	arg := strings.Split(com, " ")
	cmd := exec.Command(arg[0], arg[1:]...)
	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		logrus.Errorf("error do command: %s, err: %s", com, err.Error())
	}
	fmt.Printf("cmd: %s\n", com)
	fmt.Printf("out: --------------------------------\n%s\n--------------------------------------\n", outb.String())
	fmt.Printf("err: --------------------------------\n%s\n--------------------------------------\n", errb.String())
}

func MakeRequest(url, host string, header http.Header) string {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, _ := http.NewRequest("GET", url, nil)
	if host != "" {
		req.Host = host
	}
	req.Header = header
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	return string(b)
}
