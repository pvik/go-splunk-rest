package go_splunk_rest

import "time"

type Connection struct {
	Host                string             `toml:"host"`
	AuthType            AuthenticationType `toml:"auth-type"` // basic, authorization-token, authentication-token
	Username            string             `toml:"username"`
	Password            string             `toml:"password"`
	AuthenticationToken string             `toml:"authentication-token"`
	MaxCount            int                `toml:"max-count"`

	sessionKey         string    `toml:"-"`
	sessionKeyLastUsed time.Time `toml:"-"` // sessionKey valid for one hour, and timer resets after every use
}
