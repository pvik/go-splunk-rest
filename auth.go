package go_splunk_rest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type AuthenticationType string

const BasicAuth AuthenticationType = "basic"
const AuthenticationTokenAuth AuthenticationType = "authentication-token"
const AuthorizationTokenAuth AuthenticationType = "authorization-token"

func ParseAuthenticationType(s string) (c AuthenticationType, err error) {
	authenticationTypes := map[AuthenticationType]bool{
		BasicAuth:               true,
		AuthenticationTokenAuth: true,
		AuthorizationTokenAuth:  true,
	}

	authenticationType := AuthenticationType(s)
	_, ok := authenticationTypes[authenticationType]
	if !ok {
		return authenticationType, fmt.Errorf(`cannot parse:[%s] as AuthenticationType`, s)
	}
	return authenticationType, nil
}

func GetAllAuthenticationTypes() []AuthenticationType {
	return []AuthenticationType{
		BasicAuth,
		AuthenticationTokenAuth,
		AuthorizationTokenAuth,
	}
}

func (c Connection) getSessionKey() error {
	data := make(url.Values)
	data.Add("username", c.Username)
	data.Add("password", c.Password)
	data.Add("output_mode", "json")

	resp, respCode, err := c.httpCall("POST", "/services/auth/login", map[string]string{}, []byte(data.Encode()))
	if err != nil || respCode != http.StatusOK {
		return fmt.Errorf("unable to get sessionKey %s", err)
	}

	respStruct := struct {
		SessionKey string `json:"sessionKey"`
	}{}
	if err = json.Unmarshal(resp, &respStruct); err != nil {
		return fmt.Errorf("unable to parse sessionKey from splunk: %s | response: %s", err, string(resp))
	}

	c.sessionKey = respStruct.SessionKey
	c.sessionKeyLastUsed = time.Now()

	return nil
}

func (c Connection) wrapAuth(req *http.Request) error {
	if c.AuthType == BasicAuth {
		req.Header.Set("Authorization", "Basic "+
			base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.Username, c.Password))))
	} else if c.AuthType == AuthenticationTokenAuth {
		req.Header.Set("Authorization", "Bearer "+c.AuthenticationToken)
	} else if c.AuthType == AuthorizationTokenAuth {
		if c.sessionKey == "" || c.sessionKeyLastUsed.Add(time.Hour).Before(time.Now()) {
			err := c.getSessionKey()
			if err != nil {
				return err
			}
		}
		req.Header.Set("Authorization", "Splunk "+c.sessionKey)
	}

	return nil
}
