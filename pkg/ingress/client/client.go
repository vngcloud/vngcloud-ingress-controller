package client

type (
	AuthOpts struct {
		IdentityURL  string `gcfg:"identity-url" mapstructure:"identity-url" name:"identity-url"`
		VServerURL   string `gcfg:"vserver-url" mapstructure:"vserver-url" name:"vserver-url"`
		ClientID     string `gcfg:"client-id" mapstructure:"client-id" name:"client-id"`
		ClientSecret string `gcfg:"client-secret" mapstructure:"client-secret" name:"client-secret"`
	}
)
