package sematext

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/dat1041988/ssp-backend/server/common"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	genericAPIError    = "Fehler beim Aufruf der Sematext-API. Bitte erstelle ein Ticket."
	sematextRoleActive = "ACTIVE"
	sematextRoleAdmin  = "ADMIN"
	noAccessError      = "Du hast keinen Zugriff auf diese Sematext-Anwendung"
)

func getLogseneAppsHandler(c *gin.Context) {
	mail := common.GetUserMail(c)
	username := common.GetUserName(c)

	fmt.Sprintf("User %v listed all his sematext logsene apps", username)

	if appList, err := getAllLogseneAppsForUser(mail); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
	} else {
		c.JSON(http.StatusOK, appList)
	}
}

func getLogseneDiscountcodeHandler(c *gin.Context) {
	c.JSON(http.StatusOK, os.Getenv("LOGSENE_DISCOUNTCODE"))
}

func getLogsenePlansHandler(c *gin.Context) {
	if plans, err := getAllLogsenePlans(); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
	} else {
		c.JSON(http.StatusOK, plans)
	}
}

func updateLogsenePlanAndLimitHandler(c *gin.Context) {
	username := common.GetUserName(c)
	mail := common.GetUserMail(c)
	appId, err := strconv.Atoi(c.Param("appId"))

	if err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
		return
	}

	var data common.EditSematextPlanCommand
	if c.BindJSON(&data) == nil {
		if err := validateLogsenePlanAndLimitEdit(mail, appId, data.PlanId, data.Limit); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
			return
		}

		if err := updateLogsenePlanAndLimit(username, data.PlanId, data.Limit, appId); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		} else {
			c.JSON(http.StatusOK, common.ApiResponse{
				Message: "Der neue Plan & Limite wurden gespeichert.",
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
	}
}

func updateLogseneBillingHandler(c *gin.Context) {
	username := common.GetUserName(c)
	mail := common.GetUserMail(c)
	appId, err := strconv.Atoi(c.Param("appId"))

	if err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
		return
	}

	var data common.EditBillingDataCommand
	if c.BindJSON(&data) == nil {
		if err := validateLogseneBillingEdit(mail, appId, data.Project, data.Billing); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
			return
		}

		if err := updateLogseneBilling(username, data.Billing, data.Project, appId); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		} else {
			c.JSON(http.StatusOK, common.ApiResponse{
				Message: fmt.Sprintf("Die Kontierungsdaten (%v / %v) wurden gespeichert.", data.Billing, data.Project),
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
	}
}

func createLogseneAppHandler(c *gin.Context) {
	username := common.GetUserName(c)
	mail := common.GetUserMail(c)

	var data common.CreateLogseneAppCommand
	if c.BindJSON(&data) == nil {
		if err := validateNewLogseneApp(data.AppName, data.PlanId, data.Limit, data.Project, data.Billing); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
			return
		}

		if err := createLogseneAppAndInviteUser(username, mail, data); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		} else {
			c.JSON(http.StatusOK, common.ApiResponse{
				Message: fmt.Sprintf("Die Logsene App (%v) wurde erstellt. %v wurde als Administrator eingeladen.", data.AppName, mail),
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
	}
}

func validateNewLogseneApp(appName string, planId int, limit int, project string, billing string) error {
	if len(appName) == 0 {
		return errors.New("App-Name muss angegeben werden!")
	}

	if planId <= 0 {
		return errors.New("Plan muss angegeben werden!")
	}

	if limit <= 0 {
		return errors.New("Die Tageslimite muss muss angegeben werden!")
	}

	if len(project) == 0 {
		return errors.New("Name vom Projekt muss angegeben werden!")
	}

	if len(billing) == 0 {
		return errors.New("Kontierungsnummer muss angegeben werden!")
	}

	return nil
}

func validateLogseneBillingEdit(mail string, appId int, project string, billing string) error {
	// Check permissions
	err := validateLogseneAppPermissions(mail, appId)
	if err != nil {
		return err
	}

	// Check values
	if len(project) == 0 {
		return errors.New("Das Projekt muss angegeben werden!")
	}

	if len(billing) == 0 {
		return errors.New("Die Kontierungsnummer muss angegeben werden!")
	}

	return nil
}

func validateLogsenePlanAndLimitEdit(mail string, appId int, planId int, limit int) error {
	// Check permissions
	err := validateLogseneAppPermissions(mail, appId)
	if err != nil {
		return err
	}

	// Check values
	if planId <= 0 {
		return errors.New("Der neue Plan muss angegeben werden!")
	}

	if limit <= 0 {
		return errors.New("Die Tageslimite muss muss angegeben werden!")
	}

	return nil
}

func validateLogseneAppPermissions(mail string, appId int) error {
	userApps, err := getAllLogseneAppsForUser(mail)

	if err != nil {
		return err
	}

	for _, a := range userApps {
		if a.AppId == appId {
			return nil
		}
	}

	return errors.New(noAccessError)
}

func getAllLogseneAppsForUser(userMail string) ([]common.SematextAppList, error) {
	appData, err := getAllLogseneApps()
	if err != nil {
		return nil, err
	}

	// Filter apps where user has an active role
	allApps, err := appData.Path("data.apps").Children()
	if err != nil {
		log.Println("error getting data inside json", err.Error())
		return nil, errors.New(genericAPIError)
	}

	userApps := []common.SematextAppList{}
	for _, app := range allApps {
		appName := app.Path("name").Data().(string)
		userRoles, err := app.Path("userRoles").Children()
		if err != nil {
			log.Println("userRoles not found for current app: ", appName)
			continue
		}

		for _, userRole := range userRoles {
			mail := userRole.S("userEmail").Data().(string)
			role := userRole.S("role").Data().(string)
			status := userRole.S("roleStatus").Data().(string)

			if strings.ToLower(mail) == strings.ToLower(userMail) &&
				status == sematextRoleActive {

				u := common.SematextAppList{
					AppId:         int(app.Path("id").Data().(float64)),
					Name:          appName,
					PlanName:      app.Path("plan.name").Data().(string),
					UserRole:      role,
					IsFree:        app.Path("plan.free").Data().(bool),
					PricePerMonth: round(30*app.Path("plan.pricePerDay").Data().(float64), 0.05),
				}

				if d, ok := app.Path("description").Data().(string); ok {
					u.BillingInfo = d
				}

				userApps = append(userApps, u)
			}
		}
	}

	return userApps, nil
}

func getAllLogsenePlans() ([]common.SematextLogsenePlan, error) {
	client, req := getSematextHTTPClient("GET", "users-web/api/v3/billing/availablePlans?appType=Logsene", nil)

	resp, err := client.Do(req)

	if err != nil {
		log.Println("Error from Sematext API: ", err.Error())
		return nil, errors.New(genericAPIError)
	}

	defer resp.Body.Close()

	json, err := gabs.ParseJSONBuffer(resp.Body)
	if err != nil {
		log.Println("error parsing body of response:", err)
		return nil, errors.New(genericAPIError)
	}

	// Map response
	allPlans, err := json.Path("data.availablePlans").Children()
	if err != nil {
		log.Println("error getting data inside json", err.Error())
		return nil, errors.New(genericAPIError)
	}

	plans := []common.SematextLogsenePlan{}
	for _, plan := range allPlans {
		plans = append(plans, common.SematextLogsenePlan{
			PlanId: int(plan.Path("id").Data().(float64)),
			Name:   plan.Path("name").Data().(string),
			IsFree: plan.Path("free").Data().(bool),
			DefaultDailyMaxLimitSizeMb: plan.Path("defaultDailyMaxLimitSizeMb").Data().(float64),
			PricePerMonth:              round(30*plan.Path("pricePerDay").Data().(float64), 0.05),
		})
	}

	return plans, nil
}

func round(x, unit float64) float64 {
	return float64(int64(x/unit+0.5)) * unit
}

func getAllLogseneApps() (*gabs.Container, error) {
	client, req := getSematextHTTPClient("GET", "users-web/api/v3/apps/users", nil)

	resp, err := client.Do(req)

	if err != nil {
		log.Println("Error from Sematext API: ", err.Error())
		return nil, errors.New(genericAPIError)
	}

	defer resp.Body.Close()

	json, err := gabs.ParseJSONBuffer(resp.Body)
	if err != nil {
		log.Println("error parsing body of response:", err)
		return nil, errors.New(genericAPIError)
	}

	return json, nil
}

func createLogseneAppAndInviteUser(username string, mail string, data common.CreateLogseneAppCommand) error {
	appId, err := createLogseneApp(username, data)
	if err != nil {
		return err
	}

	if err := updateLogsenePlanAndLimit(username, data.PlanId, data.Limit, appId); err != nil {
		return err
	}

	if err := updateLogseneBilling(username, data.Billing, data.Project, appId); err != nil {
		return err
	}

	if err := inviteUserToApp(mail, appId); err != nil {
		return err
	}

	return nil
}

func createLogseneApp(username string, data common.CreateLogseneAppCommand) (int, error) {
	fmt.Sprintf("User %v creates a new logsene app, name: %v, planId: %v, limit: %v, project: %v, billing: %v",
		username, data.AppName, data.PlanId, data.Limit, data.Project, data.Billing)

	j := gabs.New()
	j.Set(data.AppName, "name")
	j.Set(data.PlanId, "initialPlanId")
	j.Set(data.DiscountCode, "discountCode")
	j.Set("Logsene", "appType")

	client, req := getSematextHTTPClient("POST", "logsene-reports/api/v3/apps", bytes.NewReader(j.Bytes()))
	resp, err := client.Do(req)

	if err != nil {
		log.Println("Error from Sematext API: ", err.Error())
		return -1, errors.New(genericAPIError)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		resJson, err := gabs.ParseJSONBuffer(resp.Body)
		if err != nil {
			log.Println("Error parsing app creation response from sematext: ", err.Error())
			return -1, errors.New(genericAPIError)
		}

		newApp, err := resJson.Path("data.apps").Children()
		if err != nil {
			log.Println("Error getting data inside json", err.Error())
			return -1, errors.New(genericAPIError)
		}

		return int(newApp[0].Path("id").Data().(float64)), nil
	} else {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Println("CreateLogseneApp: Sematext response status code was: ", resp.StatusCode, string(bodyBytes))

		if strings.Contains(string(bodyBytes), "alreadyExist") {
			return -1, errors.New("Eine Anwendung mit diesem Namen existiert bereits")
		}
	}

	return -1, errors.New(genericAPIError)
}

func inviteUserToApp(mail string, appId int) error {
	fmt.Sprintf("Inviting %v to logsene app %v.", mail, appId)

	j := gabs.New()
	j.Set(mail, "inviteeEmail")
	j.Set(sematextRoleAdmin, "inviteeRole")

	newAppId := gabs.New()
	newAppId.Set(appId, "id")
	j.Array("apps")
	j.ArrayAppend(newAppId.Data(), "apps")

	client, req := getSematextHTTPClient("POST", "users-web/api/v3/apps/guests", bytes.NewReader(j.Bytes()))
	resp, err := client.Do(req)

	if err != nil {
		log.Println("Error from Sematext API: ", err.Error())
		return errors.New(genericAPIError)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	log.Println("InviteUserToApp: Sematext response status code was: ", resp.StatusCode, string(bodyBytes))

	return errors.New(genericAPIError)
}

func updateLogseneBilling(username string, billing string, project string, appId int) error {
	fmt.Sprintf("User %v updated logsene app billing to %v / %v.", username, billing, project)

	j := gabs.New()
	j.Set(billing+" / "+project, "description")

	client, req := getSematextHTTPClient("PUT", "users-web/api/v3/apps/"+strconv.Itoa(appId), bytes.NewReader(j.Bytes()))
	resp, err := client.Do(req)

	if err != nil {
		log.Println("Error from Sematext API: ", err.Error())
		return errors.New(genericAPIError)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	log.Println("UpdateLogseneBilling: Sematext response status code was: ", resp.StatusCode, string(bodyBytes))

	return errors.New(genericAPIError)
}

func updateLogsenePlanAndLimit(username string, planId int, limit int, appId int) error {
	if err := updateLogsenePlan(username, planId, appId); err != nil {
		return err
	}

	if err := updateLogseneLimit(username, limit, appId); err != nil {
		return err
	}

	return nil
}

func updateLogseneLimit(username string, limit int, appId int) error {
	fmt.Sprintf("User %v updated logsene app limit to: %v", username, limit)

	j := gabs.New()
	j.Set(limit, "maxLimitMB")

	client, req := getSematextHTTPClient("PUT", "users-web/api/v3/apps/"+strconv.Itoa(appId), bytes.NewReader(j.Bytes()))
	resp, err := client.Do(req)

	if err != nil {
		log.Println("Error from Sematext API: ", err.Error())
		return errors.New(genericAPIError)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	log.Println("UpdateLogseneLimit: Sematext response status code was: ", resp.StatusCode, string(bodyBytes))

	return errors.New(genericAPIError)
}

func updateLogsenePlan(username string, planId int, appId int) error {
	fmt.Sprintf("User %v updated logsene app plan to planId: %v", username, planId)

	j := gabs.New()
	j.Set(planId, "planId")

	client, req := getSematextHTTPClient("PUT", "users-web/api/v3/billing/info/"+strconv.Itoa(appId), bytes.NewReader(j.Bytes()))
	resp, err := client.Do(req)

	if err != nil {
		log.Println("Error from Sematext API: ", err.Error())
		return errors.New(genericAPIError)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	log.Println("UpdateLogsenePlan: Sematext response status code was: ", resp.StatusCode, string(bodyBytes))

	return errors.New(genericAPIError)
}
