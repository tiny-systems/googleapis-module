package get_calendars

import (
	"context"
	"fmt"
	"github.com/tiny-systems/googleapis-module/components/etc"
	"github.com/tiny-systems/module/api/v1alpha1"
	"github.com/tiny-systems/module/module"
	"github.com/tiny-systems/module/registry"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	ComponentName = "get_calendars"
	RequestPort   = "request"
	ResponsePort  = "response"
	ErrorPort     = "error"
)

type Context any

type Settings struct {
	EnableErrorPort bool `json:"enableErrorPort" required:"true" title:"Enable Error Port" description:"If request may fail, error port will emit an error message"`
}

type Component struct {
	settings Settings
}

type Request struct {
	Context Context          `json:"context" title:"Context" configurable:"true"`
	Config  etc.ClientConfig `json:"config" title:"Config"  required:"true" description:"Client Config"`
	Token   etc.Token        `json:"token" required:"true" title:"Auth Token"`
}

type Response struct {
	Context   Context                       `json:"context" title:"Context" configurable:"true"`
	Calendars []*calendar.CalendarListEntry `json:"calendars"`
}

type Error struct {
	Context Context `json:"context"`
	Error   string  `json:"error"`
}

func (g *Component) GetInfo() module.ComponentInfo {
	return module.ComponentInfo{
		Name:        ComponentName,
		Description: "Get Calendar list",
		Info:        "Gets list of calendars",
		Tags:        []string{"google", "calendar", "auth"},
	}
}

func (g *Component) Handle(ctx context.Context, output module.Handler, port string, msg interface{}) any {
	if port == v1alpha1.SettingsPort {
		in, ok := msg.(Settings)
		if !ok {
			return fmt.Errorf("invalid settings")
		}
		g.settings = in
		return nil
	}

	if port != RequestPort {
		return fmt.Errorf("unknown port %s", port)
	}

	in, ok := msg.(Request)
	if !ok {
		return fmt.Errorf("invalid input message")
	}

	calendars, err := g.getCalendars(ctx, in)
	if err != nil {
		// check err port
		if !g.settings.EnableErrorPort {
			return err
		}
		return output(ctx, ErrorPort, Error{
			Context: in.Context,
			Error:   err.Error(),
		})
	}

	return output(ctx, ResponsePort, Response{
		Context:   in.Context,
		Calendars: calendars,
	})
}

func (c *Component) getCalendars(ctx context.Context, req Request) ([]*calendar.CalendarListEntry, error) {

	config, err := google.ConfigFromJSON([]byte(req.Config.Credentials), req.Config.Scopes...)
	if err != nil {
		return nil, fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	client := config.Client(ctx, &oauth2.Token{
		AccessToken:  req.Token.AccessToken,
		RefreshToken: req.Token.RefreshToken,
		Expiry:       req.Token.Expiry,
		TokenType:    req.Token.TokenType,
	})

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve calendar client: %v", err)
	}

	list, err := srv.CalendarList.List().Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (g *Component) Ports() []module.Port {
	ports := []module.Port{
		{
			Name:          v1alpha1.SettingsPort,
			Label:         "Settings",
			Configuration: Settings{},
		},
		{
			Name:          RequestPort,
			Label:         "Request",
			Position:      module.Left,
			Configuration: Request{},
		},
		{
			Source:        true,
			Name:          ResponsePort,
			Label:         "Response",
			Position:      module.Right,
			Configuration: Response{},
		},
	}
	if !g.settings.EnableErrorPort {
		return ports
	}

	return append(ports, module.Port{
		Position:      module.Bottom,
		Name:          ErrorPort,
		Label:         "Error",
		Source:        true,
		Configuration: Error{},
	})
}

func (g *Component) Instance() module.Component {
	return &Component{}
}

var _ module.Component = (*Component)(nil)

func init() {
	registry.Register(&Component{})
}
