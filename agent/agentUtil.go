package agent

import  (
	"context"
)

func StartAgent(callback func()) {

	agent := New("http://localhost:8081","pusher-service")

	agent.Start(context.Background())

	callback()
}
