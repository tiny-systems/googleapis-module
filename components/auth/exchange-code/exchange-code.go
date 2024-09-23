package exchange_code

import (
	"context"
	"fmt"
	"github.com/tiny-systems/googleapis-module/components/etc"
	"github.com/tiny-systems/module/module"
	"github.com/tiny-systems/module/registry"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	ComponentName = "exchange_auth_code"
	RequestPort   = "request"
	ResponsePort  = "response"
	ErrorPort     = "error"
)

type Context any

type Request struct {
	Context     Context          `json:"context,omitempty" title:"Context" configurable:"true"`
	Config      etc.ClientConfig `json:"config" title:"Config"  required:"true" description:"Client Config"`
	AuthCode    string           `json:"authCode" required:"true" title:"Authorisation code"`
	RedirectURL string           `json:"redirectUrl" title:"Redirect URL" description:"Overrides redirect URL from config"`
}

type Settings struct {
	EnableErrorPort bool `json:"enableErrorPort" required:"true" title:"Enable Error Port" description:"If request may fail, error port will emit an error message"`
}

type Response struct {
	Context Context   `json:"context" title:"Context"`
	Token   etc.Token `json:"token"`
}

type Error struct {
	Context Context `json:"context"`
	Error   string  `json:"error"`
}

///

type Component struct {
	settings Settings
}

func (a *Component) GetInfo() module.ComponentInfo {
	return module.ComponentInfo{
		Name:        ComponentName,
		Description: "Exchange Auth Code",
		Info:        "Exchanges Auth code to Auth token",
		Tags:        []string{"google", "auth"},
	}
}

func (a *Component) exchange(ctx context.Context, in Request) (*oauth2.Token, error) {

	config, err := google.ConfigFromJSON([]byte(in.Config.Credentials), in.Config.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	if in.RedirectURL != "" {
		config.RedirectURL = in.RedirectURL
	}
	return config.Exchange(ctx, in.AuthCode)
}

func (a *Component) Handle(ctx context.Context, output module.Handler, port string, msg interface{}) error {
	if port == module.SettingsPort {
		in, ok := msg.(Settings)
		if !ok {
			return fmt.Errorf("invalid settings")
		}
		a.settings = in
		return nil
	}

	if port != RequestPort {
		return fmt.Errorf("unknown port %s", port)
	}

	in, ok := msg.(Request)
	if !ok {
		return fmt.Errorf("invalid input message")
	}

	token, err := a.exchange(ctx, in)
	if err != nil {
		// check err port
		if !a.settings.EnableErrorPort {
			return err
		}
		return output(ctx, ErrorPort, Error{
			Context: in.Context,
			Error:   err.Error(),
		})
	}

	return output(ctx, ResponsePort, Response{
		Context: in.Context,
		Token: etc.Token{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			TokenType:    token.TokenType,
			Expiry:       token.Expiry,
		},
	})

}

func (a *Component) Ports() []module.Port {
	ports := []module.Port{
		{
			Name:          module.SettingsPort,
			Label:         "Settings",
			Configuration: Settings{},
			Source:        true,
		},
		{
			Source:        true,
			Name:          RequestPort,
			Label:         "Request",
			Position:      module.Left,
			Configuration: Request{},
		},
		{
			Source:        false,
			Name:          ResponsePort,
			Label:         "Response",
			Position:      module.Right,
			Configuration: Response{},
		},
	}

	if !a.settings.EnableErrorPort {
		return ports
	}

	return append(ports, module.Port{
		Position:      module.Bottom,
		Name:          "error",
		Label:         "Error",
		Source:        false,
		Configuration: Error{},
	})
}

func (a *Component) Instance() module.Component {
	return &Component{}
}

var _ module.Component = (*Component)(nil)

func init() {
	registry.Register(&Component{})
}
