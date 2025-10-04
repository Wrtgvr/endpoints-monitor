package monitor

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/wrtgvr/websites-monitor/internal/domain"
)

func endpointPing(ep *domain.EndpointInfo, timeout time.Duration) *domain.EndpointStatus {
	client := &http.Client{
		Timeout: timeout,
	}

	var status string
	pingedAt := time.Now()

	resp, err := client.Get(ep.URL)
	if err != nil {
		status = "Error: check logs"
		log.Printf("Error pinging, err=%v", err)
	} else {
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			status = fmt.Sprintf("Success: %d", resp.StatusCode)
		} else {
			status = fmt.Sprintf("Failure: %d", resp.StatusCode)
		}
	}

	respTime := time.Since(pingedAt)

	return &domain.EndpointStatus{
		ID:           ep.ID,
		Status:       status,
		LastChecked:  pingedAt.Format(time.RFC3339),
		ResponseTime: pingedAt.Add(respTime).Format(time.RFC3339),
	}
}
