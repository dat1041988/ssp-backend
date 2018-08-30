package openshift

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"fmt"

	"github.com/Jeffail/gabs"
	"github.com/dat1041988/ssp-backend/server/common"
	"github.com/gin-gonic/gin"
)

func newProjectHandler(c *gin.Context) {
	username := common.GetUserName(c)

	var data common.NewProjectCommand
	if c.BindJSON(&data) == nil {
		if err := validateNewProject(data.Project, data.Billing, false); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
			return
		}

		if err := createNewProject(data.Project, username, data.Billing, data.MegaId, false); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		} else {
			c.JSON(http.StatusOK, common.ApiResponse{
				Message: fmt.Sprintf("Das Projekt %v wurde erstellt", data.Project),
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
	}
}

func newTestProjectHandler(c *gin.Context) {
	username := common.GetUserName(c)

	var data common.NewTestProjectCommand
	if c.BindJSON(&data) == nil {
		// Special values for a test project
		billing := "keine-verrechnung"
		data.Project = username + "-" + data.Project

		if err := validateNewProject(data.Project, billing, true); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
			return
		}

		if err := createNewProject(data.Project, username, billing, "", true); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		} else {
			c.JSON(http.StatusOK, common.ApiResponse{
				Message: fmt.Sprintf("Das Test-Projekt %v wurde erstellt", data.Project),
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
	}
}

func getProjectAdminsHandler(c *gin.Context) {
	username := common.GetUserName(c)
	project := c.Param("project")

	log.Printf("%v has queried all the admins of project %v", username, project)

	if admins, _, err := getProjectAdminsAndOperators(project); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
	} else {
		c.JSON(http.StatusOK, common.AdminList{
			Admins: admins,
		})
	}
}

func getBillingHandler(c *gin.Context) {
	username := common.GetUserName(c)
	project := c.Param("project")

	if err := validateAdminAccess(username, project); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		return
	}

	if billingData, err := getProjectBillingInformation(project); err != nil {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
	} else {
		c.JSON(http.StatusOK, common.ApiResponse{
			Message: fmt.Sprintf("Aktuelle Verrechnungsdaten für Projekt %v: %v", project, billingData),
		})
	}
}

func updateBillingHandler(c *gin.Context) {
	username := common.GetUserName(c)

	var data common.EditBillingDataCommand
	if c.BindJSON(&data) == nil {
		if err := validateBillingInformation(data.Project, data.Billing, username); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
			return
		}

		if err := createOrUpdateMetadata(data.Project, data.Billing, "", username, false); err != nil {
			c.JSON(http.StatusBadRequest, common.ApiResponse{Message: err.Error()})
		} else {
			c.JSON(http.StatusOK, common.ApiResponse{
				Message: fmt.Sprintf("Die Verrechnungsdaten wurden gespeichert: %v", data.Billing),
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, common.ApiResponse{Message: wrongAPIUsageError})
	}
}

func validateNewProject(project string, billing string, testProject bool) error {
	if len(project) == 0 {
		return errors.New("Projektname muss angegeben werden")
	}

	if !testProject && len(billing) == 0 {
		return errors.New("Kontierungsnummer muss angegeben werden")
	}

	return nil
}

func validateAdminAccess(username string, project string) error {
	if len(project) == 0 {
		return errors.New("Projektname muss angegeben werden")
	}

	// Validate permissions
	if err := checkAdminPermissions(username, project); err != nil {
		return err
	}

	return nil
}

func validateBillingInformation(project string, billing string, username string) error {
	if len(project) == 0 {
		return errors.New("Projektname muss angegeben werden")
	}

	if len(billing) == 0 {
		return errors.New("Kontierungsnummer muss angegeben werden")
	}

	// Validate permissions
	if err := checkAdminPermissions(username, project); err != nil {
		return err
	}

	return nil
}

func createNewProject(project string, username string, billing string, megaid string, testProject bool) error {
	project = strings.ToLower(project)
	p := newObjectRequest("ProjectRequest", project)

	client, req := getOseHTTPClient("POST",
		"oapi/v1/projectrequests",
		bytes.NewReader(p.Bytes()))

	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
	}

	if resp.StatusCode == http.StatusCreated {
		log.Printf("%v created a new project: %v", username, project)

		if err := changeProjectPermission(project, username); err != nil {
			return err
		}

		if err := createOrUpdateMetadata(project, billing, megaid, username, testProject); err != nil {
			return err
		}
		return nil
	}
	if resp.StatusCode == http.StatusConflict {
		return errors.New("Das Projekt existiert bereits")
	}

	errMsg, _ := ioutil.ReadAll(resp.Body)
	log.Println("Error creating new project:", err, resp.StatusCode, string(errMsg))

	return errors.New(genericAPIError)
}

func changeProjectPermission(project string, username string) error {
	// Get existing policybindings
	policyBindings, err := getPolicyBindings(project)

	if policyBindings == nil {
		return err
	}

	children, err := policyBindings.S("roleBindings").Children()
	if err != nil {
		log.Println("Unable to parse roleBindings", err.Error())
		return errors.New(genericAPIError)
	}
	for _, v := range children {
		if v.Path("name").Data().(string) == "admin" {
			v.ArrayAppend(strings.ToLower(username), "roleBinding", "userNames")
			v.ArrayAppend(strings.ToUpper(username), "roleBinding", "userNames")
		}
	}

	// Update the policyBindings on the api
	client, req := getOseHTTPClient("PUT",
		"oapi/v1/namespaces/"+project+"/policybindings/:default",
		bytes.NewReader(policyBindings.Bytes()))

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error from server: ", err.Error())
		return errors.New(genericAPIError)
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Print(username + " is now admin of " + project)
		return nil
	}

	errMsg, _ := ioutil.ReadAll(resp.Body)
	log.Println("Error updating project permissions:", err, resp.StatusCode, string(errMsg))
	return errors.New(genericAPIError)
}

func getProjectBillingInformation(project string) (string, error) {
	client, req := getOseHTTPClient("GET", "api/v1/namespaces/"+project, nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error from server: ", err.Error())
		return "", errors.New(genericAPIError)
	}

	defer resp.Body.Close()

	json, err := gabs.ParseJSONBuffer(resp.Body)
	if err != nil {
		log.Println("error decoding json:", err, resp.StatusCode)
		return "", errors.New(genericAPIError)
	}

	billing := json.Path("metadata.annotations").S("openshift.io/kontierung-element").Data()
	if billing != nil {
		return billing.(string), nil
	} else {
		return "Keine Daten hinterlegt", nil
	}
}

func createOrUpdateMetadata(project string, billing string, megaid string, username string, testProject bool) error {
	client, req := getOseHTTPClient("GET", "api/v1/namespaces/"+project, nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error from server: ", err.Error())
		return errors.New(genericAPIError)
	}

	defer resp.Body.Close()

	json, err := gabs.ParseJSONBuffer(resp.Body)
	if err != nil {
		log.Println("error decoding json:", err, resp.StatusCode)
		return errors.New(genericAPIError)
	}

	annotations := json.Path("metadata.annotations")
	annotations.Set(billing, "openshift.io/kontierung-element")
	annotations.Set(username, "openshift.io/requester")

	if testProject {
		annotations.Set(testProjectDeletionDays, "openshift.io/testproject-daystodeletion")
		annotations.Set(fmt.Sprintf("Dieses Testprojekt wird in %v Tagen automatisch gelöscht!", testProjectDeletionDays), "openshift.io/description")
	}

	if len(megaid) > 0 {
		annotations.Set(megaid, "openshift.io/MEGAID")
	}

	client, req = getOseHTTPClient("PUT",
		"api/v1/namespaces/"+project,
		bytes.NewReader(json.Bytes()))

	resp, err = client.Do(req)

	if resp.StatusCode == http.StatusOK {
		resp.Body.Close()
		log.Println("User "+username+" changed changed config of project project "+project+". Kontierungsnummer: "+billing, ", MegaID: "+megaid)
		return nil
	}

	errMsg, _ := ioutil.ReadAll(resp.Body)
	log.Println("Error updating project config:", err, resp.StatusCode, string(errMsg))

	return errors.New(genericAPIError)
}
