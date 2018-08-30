package sematext

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/dat1041988/ssp-backend/server/common"
	"github.com/gin-gonic/gin"
	"strings"
)

const (
	wrongAPIUsageError = "Invalid api call - parameters did not match to method definition"
)

func RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/sematext/plans", getLogsenePlansHandler)
	r.GET("/sematext/discountcode", getLogseneDiscountcodeHandler)
	r.GET("/sematext/logsene", getLogseneAppsHandler)
	r.POST("/sematext/logsene", createLogseneAppHandler)
	r.POST("/sematext/logsene/:appId", updateLogseneBillingHandler)
	r.POST("/sematext/logsene/:appId/plan", updateLogsenePlanAndLimitHandler)
}

func getSematextHTTPClient(method string, urlPart string, body io.Reader) (*http.Client, *http.Request) {
	token := os.Getenv("SEMATEXT_API_TOKEN")
	baseUrl := os.Getenv("SEMATEXT_BASE_URL")
	if len(token) == 0 || len(baseUrl) == 0 {
		log.Fatal("Env variables 'SEMATEXT_API_TOKEN' and 'SEMATEXT_BASE_URL' must be specified")
	}

	if !strings.HasSuffix(baseUrl, "/") {
		baseUrl += "/"
	}

	client := &http.Client{}
	req, _ := http.NewRequest(method, baseUrl+urlPart, body)

	if common.DebugMode() {
		log.Println("Calling ", req.URL.String())
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "apiKey "+token)

	return client, req
}
