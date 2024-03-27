package client

type (
	AuthOpts struct {
		IdentityURL  string `gcfg:"identityURL" mapstructure:"identityURL" name:"identityURL"`
		VServerURL   string `gcfg:"vserverURL" mapstructure:"vserverURL" name:"vserverURL"`
		ClientID     string `gcfg:"clientID" mapstructure:"clientID" name:"clientID"`
		ClientSecret string `gcfg:"clientSecret" mapstructure:"clientSecret" name:"clientSecret"`
	}
)
