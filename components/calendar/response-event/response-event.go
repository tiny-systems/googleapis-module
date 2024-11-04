package response_event

import (
	"context"
	"fmt"
	"github.com/tiny-systems/googleapis-module/components/etc"
	"github.com/tiny-systems/module/module"
	"github.com/tiny-systems/module/registry"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	ComponentName = "response_event"
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
	Context            Context          `json:"context" title:"Context" configurable:"true"`
	Config             etc.ClientConfig `json:"config" title:"Config"  required:"true" description:"Client Config"`
	Token              etc.Token        `json:"token" required:"true" title:"Auth Token"`
	CalendarID         string           `json:"calendarID" title:"Calendar ID" required:"true"`
	EventID            string           `json:"eventID" title:"Event ID" required:"true"`
	EventAttendeeEmail string           `json:"eventAttendeeEmail" title:"Event Attendee Email" required:"true"`
	ResponseStatus     string           `json:"responseStatus" title:"Response Status" required:"true" enum:"accepted,declined,tentative"`
}

type Response struct {
	Context Context `json:"context"`
}

type Error struct {
	Context Context `json:"context"`
	Error   string  `json:"error"`
}

func (g *Component) GetInfo() module.ComponentInfo {
	return module.ComponentInfo{
		Name:        ComponentName,
		Description: "Response event",
		Info:        "Response to calendar event",
		Tags:        []string{"google", "calendar", "auth"},
	}
}

func (g *Component) Handle(ctx context.Context, output module.Handler, port string, msg interface{}) error {
	if port == module.SettingsPort {
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

	err := g.responseEvent(ctx, in)
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
		Context: in.Context,
	})
}

func (c *Component) responseEvent(ctx context.Context, req Request) error {

	config, err := google.ConfigFromJSON([]byte(req.Config.Credentials), req.Config.Scopes...)
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %v", err)
	}

	client := config.Client(ctx, &oauth2.Token{
		AccessToken:  req.Token.AccessToken,
		RefreshToken: req.Token.RefreshToken,
		Expiry:       req.Token.Expiry,
		TokenType:    req.Token.TokenType,
	})

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("unable to retrieve calendar client: %v", err)
	}

	event, err := srv.Events.Get(req.CalendarID, req.EventID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("unable to retrieve event: %v", err)
	}
	//

	for _, a := range event.Attendees {
		if a.Email != req.EventAttendeeEmail {
			continue
		}
		a.ResponseStatus = req.ResponseStatus
	}

	_, err = srv.Events.Update(req.CalendarID, req.EventID, event).Context(ctx).Do()

	return err
}

func (g *Component) Ports() []module.Port {
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
	if !g.settings.EnableErrorPort {
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

func (g *Component) Instance() module.Component {
	return &Component{}
}

var _ module.Component = (*Component)(nil)

func init() {
	registry.Register(&Component{})
}
