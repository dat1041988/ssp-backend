package aws

import (
	"errors"
	"log"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"

	"github.com/dat1041988/ssp-backend/server/common"
)

const (
	genericUserCreationError = "Es ist ein Fehler bei der Erstellung des Benutzers aufgetreten"
)

// PolicyDocument IAM Policy Document
type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

// StatementEntry IAM Statement Entry
type StatementEntry struct {
	Effect   string
	Action   []string
	Resource string
}

func validateNewS3User(username string, bucketname string, newuser string, stage string) error {
	if len(username) == 0 {
		return errors.New("Benutzername muss angegeben werden")
	}
	if len(bucketname) == 0 {
		return errors.New("Bucket Name muss angegeben werden")
	}
	if len(newuser) == 0 {
		return errors.New("Bucket Benutzername muss angegeben werden")
	}

	if (len(newuser) + len(bucketname)) > 63 {
		// http://docs.aws.amazon.com/IAM/latest/UserGuide/reference_iam-limits.html
		return errors.New("Generierter Benutzername '" + bucketname + "-" + newuser + "' ist zu lang")
	}
	validName := regexp.MustCompile(`^[a-zA-Z0-9\-]+$`).MatchString
	if !validName(bucketname) {
		return errors.New("Benutzername kann nur alphanumerische Zeichen und Bindestriche enthalten")
	}

	svc, err := GetIAMClient(stage)
	if err != nil {
		return err
	}
	result, err := svc.ListUsers(nil)
	if err != nil {
		log.Print("Error while trying to create a new user (ListUsers call): " + err.Error())
		return errors.New(genericUserCreationError)
	}
	// Loop over existing users
	for _, u := range result.Users {
		if *u.UserName == newuser {
			log.Printf("Error, user %v already exists", newuser)
			return errors.New("Fehler: IAM-Benutzer " + newuser + " existiert bereits")
		}
	}

	// Make sure the user is allowed to create new IAM users for this bucket
	myBuckets, _ := listS3BucketByUsername(username)
	for _, mybucket := range myBuckets.Buckets {
		if bucketname == mybucket.Name {
			// Everything OK
			return nil
		}
	}
	return errors.New("Es gibt diesen Bucket " + bucketname + " nicht. Oder du darfst für den Bucket keine Benutzer erstellen")
}

func createNewS3User(bucketname string, s3username string, stage string, isReadonly bool) (*common.S3CredentialsResponse, error) {
	generatedName := bucketname + "-" + s3username

	svc, err := GetIAMClient(stage)
	if err != nil {
		return nil, err
	}
	usr, err := svc.GetUser(&iam.GetUserInput{
		UserName: aws.String(generatedName),
	})

	if usr != nil && usr.User != nil {
		return nil, errors.New("Der Benutzer existiert bereits")
	}

	cred := common.S3CredentialsResponse{
		Username: generatedName,
	}
	if errAws, ok := err.(awserr.Error); ok && errAws.Code() == iam.ErrCodeNoSuchEntityException {
		_, err := svc.CreateUser(&iam.CreateUserInput{
			UserName: aws.String(generatedName),
		})

		if err != nil {
			log.Println("CreateUser error in createNewS3User: " + err.Error())
			return nil, errors.New(genericUserCreationError)
		}

		// Create access key
		result, err := svc.CreateAccessKey(&iam.CreateAccessKeyInput{
			UserName: aws.String(generatedName),
		})
		cred.AccessKeyID = *result.AccessKey.AccessKeyId
		cred.SecretKey = *result.AccessKey.SecretAccessKey
	} else {
		log.Println("Failed to create used: ", err.Error())
		return nil, errors.New(genericUserCreationError)
	}

	policy := bucketname
	if isReadonly {
		policy += bucketReadPolicy
	} else {
		policy += bucketWritePolicy
	}

	err = attachIAMPolicyToUser(policy, generatedName, stage)
	if err != nil {
		log.Print("Error while calling attachIAMPolicyToUser: " + err.Error())
		return &cred, errors.New(genericUserCreationError)
	}

	return &cred, nil
}

func attachIAMPolicyToUser(policyName string, username string, stage string) error {
	svc, err := GetIAMClient(stage)
	if err != nil {
		return err
	}

	// First, figure out the AWS account ID
	result, err := svc.GetUser(nil)
	var accountNumber string
	if err != nil {
		return errors.New("GetUser error in attachIAMPolicyToUser() while trying to determine account ID: " + err.Error())
	}
	re := regexp.MustCompile("[0-9]+")
	accountNumber = re.FindString(*result.User.Arn)

	// Then, attach the policy given to the user
	input := &iam.AttachUserPolicyInput{
		PolicyArn: aws.String("arn:aws:iam::" + accountNumber + ":policy/" + policyName),
		UserName:  aws.String(username),
	}
	_, err = svc.AttachUserPolicy(input)
	if err != nil {
		return errors.New("AttachUserPolicy error: " + err.Error())
	}

	return nil
}
