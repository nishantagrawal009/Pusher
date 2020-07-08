package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"runtime/pprof"
	"strings"
	"sync"
	"time"
)

const(
	defaultDuration     = 10 * time.Second
	defaultTickInterval = 1 *time.Minute
	defaultProfileType = TypeCPU
)
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type  Agent struct {
CPUProfile          bool
HeapProfile         bool
BlockProfile        bool
MutexProfile        bool
GoroutineProfile    bool
ThreadcreateProfile bool
CPUProfileDuration  time.Duration
rawClient     httpClient
service   string
rawLabels strings.Builder
collectorAddr string
tick time.Duration
stop chan struct{} // signals the beginning of stop
done chan struct{} // closed when stopping is done
}

func New(addr, service string) *Agent {
	a := &Agent{
		CPUProfile :	   true,
		HeapProfile :        true,
		BlockProfile :        true,
		MutexProfile:        true,
		GoroutineProfile :        true,
		ThreadcreateProfile :        true,
		collectorAddr: addr,
		CPUProfileDuration:defaultDuration,
		service:       service,
		tick: defaultTickInterval,
		rawClient: http.DefaultClient,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	return a
}

func (a *Agent) Start(ctx context.Context) error {
	if a.collectorAddr == "" {
		return fmt.Errorf("failed to start agent: collector address is empty")
	}

	go a.collectAndSend(ctx)

	return nil
}

func (a *Agent) Stop() error {
	close(a.stop)
	<-a.done
	return nil
}

func (a *Agent) collectAndSend(ctx context.Context) {
	defer close(a.done)


	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-a.stop
		cancel()
	}()

	var (
		ptype = a.nextProfileType(TypeUnknown)
		timer = time.NewTimer(tickInterval(0))

		buf bytes.Buffer
	)

	for {
		select {
		case <-a.stop:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			if err := a.collectProfile(ctx, ptype, &buf); err != nil {
				fmt.Errorf("failed to collectProfile: %v", err)
			} else if err := a.sendProfile(ctx, ptype, &buf); err != nil {
				fmt.Errorf("failed to sendProfile: %v", err)
			}

			buf.Reset()

			ptype = a.nextProfileType(ptype)

			var tick time.Duration
			if ptype == defaultProfileType {
				// we took the full set of profiles, sleep for the whole tick
				tick = a.tick
			}

			timer.Reset(tickInterval(tick))
		}
	}
}

func (a *Agent) nextProfileType(ptype ProfileType) ProfileType {
	// special case to choose initial profile type on the first call
	if ptype == TypeUnknown {
		return defaultProfileType
	}

	for {
		switch ptype {
		case TypeCPU:
			ptype = TypeHeap
			if a.HeapProfile {
				return ptype
			}
		case TypeHeap:
			ptype = TypeBlock
			if a.BlockProfile {
				return ptype
			}
		case TypeBlock:
			ptype = TypeMutex
			if a.MutexProfile {
				return ptype
			}
		case TypeMutex:
			ptype = TypeGoroutine
			if a.GoroutineProfile {
				return ptype
			}
		case TypeGoroutine:
			ptype = TypeThreadcreate
			if a.ThreadcreateProfile {
				return ptype
			}
		case TypeThreadcreate:
			ptype = TypeCPU
			if a.CPUProfile {
				return ptype
			}
		}
	}
}
func (a *Agent) collectProfile(ctx context.Context, ptype ProfileType, buf *bytes.Buffer) error {
	switch ptype {
	case TypeCPU:
		err := pprof.StartCPUProfile(buf)
		if err != nil {
			return fmt.Errorf("failed to start CPU profile: %v", err)
		}
		sleep(a.CPUProfileDuration, ctx.Done())
		pprof.StopCPUProfile()
	case TypeHeap:
		err := pprof.WriteHeapProfile(buf)
		if err != nil {
			return fmt.Errorf("failed to write heap profile: %v", err)
		}
	case TypeBlock,
		TypeMutex,
		TypeGoroutine,
		TypeThreadcreate:

		p := pprof.Lookup(ptype.String())
		if p == nil {
			return fmt.Errorf("unknown profile type %v", ptype)
		}
		err := p.WriteTo(buf, 0)
		if err != nil {
			return fmt.Errorf("failed to write %s profile: %v", ptype, err)
		}
	default:
		return fmt.Errorf("unknown profile type %v", ptype)
	}

	return nil
}

func (a *Agent) sendProfile(ctx context.Context, ptype ProfileType, buf *bytes.Buffer) error {
	// sending the testing data
	q := url.Values{}
	q.Set("service", a.service)
	q.Set("labels", "version=1.0.0")
	q.Set("type", ptype.String())

	surl := a.collectorAddr + "/api/0/profiles?" + q.Encode()

	req, err := http.NewRequest(http.MethodPost, surl, buf)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	err = a.doRequest(req, nil)
	if err != nil {
		fmt.Println("error while sending the file",err)
	}
	return  nil
}
func (a *Agent) doRequest(req *http.Request, v io.Writer) error {
	resp, err := a.rawClient.Do(req)
	if err, ok := err.(*url.Error); ok && err.Err == context.Canceled {
		return err
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("unexpected respose %s: %v", resp.Status, err)
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("unexpected respose from collector %s: %s", resp.Status, respBody)
		}
		return fmt.Errorf("bad request: collector responded with %s: %s", resp.Status, respBody)
	}

	if v != nil {
		_, err := io.Copy(v, resp.Body)
		return err
	}

	return nil
}

func tickInterval(d time.Duration) time.Duration {
	// add up to extra 10 seconds to sleep to dis-align profiles of different instances
	noise := time.Second + time.Duration(rand.Intn(9))*time.Second
	return d + noise
}

var timersPool = sync.Pool{}

func sleep(d time.Duration, cancel <-chan struct{}) {
	timer, _ := timersPool.Get().(*time.Timer)
	if timer == nil {
		timer = time.NewTimer(d)
	} else {
		timer.Reset(d)
	}

	select {
	case <-timer.C:
	case <-cancel:
		if !timer.Stop() {
			<-timer.C
		}
	}

	timersPool.Put(timer)
}