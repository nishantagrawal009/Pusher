package agent

import "context"

func StartAgent() {

	agent := New("http://localhost:8081","collector")

	agent.Start(context.Background())

	defer agent.Stop()
}
