
package agent

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
)
var(
	buf bytes.Buffer
)

func TestAgent_Start(t *testing.T) {
	q := url.Values{}
	q.Set("service", "profile_service")
	q.Set("labels", "version=1.0.0")
	q.Set("type", TypeCPU.String())

	ctx:= context.Background()
	surl := "http://localhost:8081" + "/api/0/profiles?" + q.Encode()

	pprofData, err := ioutil.ReadFile("collector_cpu_1.prof")
	if err != nil {
		fmt.Println("unable to read test file",err)
		return
	}

	buf.Write(pprofData)

	req, err := http.NewRequest(http.MethodPost, surl, &buf)

	req = req.WithContext(ctx)

	client  := http.DefaultClient
	resp, err :=  client.Do(req)
	if err, ok := err.(*url.Error); ok && err.Err == context.Canceled {
		fmt.Println(err)
		return
	}
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("unexpected respose %s: %v", resp.Status, err)
		}
		if resp.StatusCode >= 500 {
			err = fmt.Errorf("unexpected respose from collector %s: %s", resp.Status, respBody)
		}
		err = fmt.Errorf("bad request: collector responded with %s: %s", resp.Status, respBody)
		fmt.Println(err)
		return
	}
}