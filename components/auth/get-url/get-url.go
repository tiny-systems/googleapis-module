package get_url

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
	ComponentName = "get_auth_url"
	RequestPort   = "request"
	ResponsePort  = "response"
	ErrorPort     = "error"
)

type Context any

type Request struct {
	Context       Context          `json:"context,omitempty" title:"Context" configurable:"true"`
	Config        etc.ClientConfig `json:"config" required:"true" title:"Client credentials"`
	AccessType    string           `json:"accessType" title:"Type of access" enum:"offline,online" enumTitles:"Offline,Online" required:"true"`
	ApprovalForce bool             `json:"approvalForce" title:"ApprovalForce" required:"true"`
}

type Settings struct {
	EnableErrorPort bool `json:"enableErrorPort" required:"true" title:"Enable Error Port" description:"If request may fail, error port will emit an error message"`
}

type Error struct {
	Context Context `json:"context"`
	Error   string  `json:"error"`
}

type Response struct {
	Context Context `json:"context"`
	AuthUrl string  `json:"authUrl" format:"uri"`
}

type Component struct {
	settings Settings
}

func (a *Component) GetInfo() module.ComponentInfo {
	return module.ComponentInfo{
		Name:        ComponentName,
		Description: "Get Auth URL",
		Info:        "Gets Auth URL which later may be used fot auth redirect",
		Tags:        []string{"google", "auth"},
	}
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
	//
	in, ok := msg.(Request)
	if !ok {
		return fmt.Errorf("invalid input message")
	}
	url, err := getAuthUrl(ctx, in)

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
		AuthUrl: url,
	})
}

func getAuthUrl(_ context.Context, in Request) (string, error) {

	config, err := google.ConfigFromJSON([]byte(in.Config.Credentials), in.Config.Scopes...)
	if err != nil {
		return "", fmt.Errorf("unable to parse client secret file to config: %v", err)
	}
	var opts []oauth2.AuthCodeOption
	if in.ApprovalForce {
		opts = append(opts, oauth2.ApprovalForce)
	}
	if in.AccessType == "online" {
		opts = append(opts, oauth2.AccessTypeOnline)
	} else {
		opts = append(opts, oauth2.AccessTypeOffline)
	}
	return config.AuthCodeURL("state-token", opts...), nil
}

func (a *Component) Ports() []module.Port {
	ports := []module.Port{
		{
			Source:   true,
			Name:     RequestPort,
			Label:    "Request",
			Position: module.Left,
			Configuration: Request{
				AccessType:    "offline",
				ApprovalForce: true,
			},
		},
		{
			Name:          module.SettingsPort,
			Label:         "Settings",
			Configuration: Settings{},
			Source:        true,
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
		Name:          ErrorPort,
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
