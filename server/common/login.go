package common

import (
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jtblin/go-ldap-client"
	"github.com/patrickmn/go-cache"
	"gopkg.in/appleboy/gin-jwt.v2"
)

var userCache *cache.Cache

// GetAuthMiddleware returns a gin middleware for JWT with cookie based auth
func GetAuthMiddleware() *jwt.GinJWTMiddleware {
	key := os.Getenv("SESSION_KEY")

	if len(key) == 0 {
		log.Fatal("Env variable 'SESSION_KEY' must be specified")
	}

	// Initialize the user cache
	userCache = cache.New(10*time.Minute, 24*time.Hour)

	return &jwt.GinJWTMiddleware{
		Realm:         "CLOUD_SSP",
		Key:           []byte(key),
		Timeout:       time.Hour,
		MaxRefresh:    time.Hour,
		Authenticator: ldapAuthenticator,
		Authorizator: func(userId interface{}, c *gin.Context) bool {
			return true
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
		PayloadFunc: userPayloadFunc,
		TokenLookup: "header:Authorization",
		TimeFunc:    time.Now,
	}
}

func userPayloadFunc(userID interface{}) jwt.MapClaims {
	if mail, ok := userCache.Get(userID.(string)); ok {
		res := make(map[string]interface{})
		res["mail"] = mail
		return res
	} else {
		log.Println("Error, could not find mail for user: " + userID.(string) + " - Sematext functions will not work correctly")
		return nil
	}
}

func ldapAuthenticator(userID string, password string,c *gin.Context) (interface{}, bool) {
	ldapHost := os.Getenv("LDAP_URL")
	ldapBind := os.Getenv("LDAP_BIND_DN")
	ldapBindPw := os.Getenv("LDAP_BIND_CRED")
	ldapFilter := os.Getenv("LDAP_FILTER")
	ldapSearchBase := os.Getenv("LDAP_SEARCH_BASE")

	client := &ldap.LDAPClient{
		Attributes:   []string{"givenName", "sn", "mail", "uid"},
		Base:         ldapSearchBase,
		Host:         ldapHost,
		Port:         389,
		UseSSL:       false,
		SkipTLS:      true,
		BindDN:       ldapBind,
		BindPassword: ldapBindPw,
		UserFilter:   ldapFilter,
	}

	// It is the responsibility of the caller to close the connection
	defer client.Close()

	ok, user, err := client.Authenticate(userID, password)
	if err != nil {
		log.Printf("Error authenticating user %s: %+v", userID, err)
	}
	if !ok {
		log.Printf("Authenticating failed for user %s", userID)
	}

	// Put the user mail address in the cache for the payload function
	userCache.Add(userID, user["mail"], cache.DefaultExpiration)

	return userID, ok
}
