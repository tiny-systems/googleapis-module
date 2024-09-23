package get_events

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
	"time"
)

const (
	ComponentName = "calendar_get_events"
	RequestPort   = "request"
	ResponsePort  = "response"
	ErrorPort     = "error"
)

type Context any

type Request struct {
	Context     Context          `json:"context,omitempty" configurable:"true" title:"Context" description:"Arbitrary message to be send further"`
	Config      etc.ClientConfig `json:"config" required:"true" title:"Client credentials"`
	CalendarId  string           `json:"calendarId" required:"true" default:"primary" minLength:"1" title:"Calendar ID"`
	StartDate   time.Time        `json:"startDate" title:"Start date"`
	EndDate     time.Time        `json:"endDate" title:"End date"`
	Token       etc.Token        `json:"token" required:"true" title:"Auth Token"`
	SyncToken   string           `json:"syncToken,omitempty" title:"Sync Token"`
	PageToken   string           `json:"pageToken,omitempty" title:"Page Token"`
	ShowDeleted bool             `json:"showDeleted,omitempty" title:"Show deleted events" default:"true"`
}

type Error struct {
	Context Context `json:"context"`
	Error   string  `json:"error"`
}

type Response struct {
	Context Context         `json:"context"`
	Results calendar.Events `json:"results"`
}

type Component struct {
	settings Settings
}

type Settings struct {
	EnableErrorPort bool `json:"enableErrorPort" default:"false" required:"true" title:"Enable Error Port" description:"If request may fail, error port will emit an error message"`
}

func (c *Component) GetInfo() module.ComponentInfo {
	return module.ComponentInfo{
		Name:        ComponentName,
		Description: "Calendar Get Events",
		Info:        "Calendar Get Events",
		Tags:        []string{"Google", "Calendar"},
	}
}

func (c *Component) Handle(ctx context.Context, handler module.Handler, port string, msg interface{}) error {
	if port == module.SettingsPort {
		in, ok := msg.(Settings)
		if !ok {
			return fmt.Errorf("invalid settings")
		}
		c.settings = in
		return nil
	}

	if port != RequestPort {
		return fmt.Errorf("unknown port %s", port)
	}

	req, ok := msg.(Request)
	if !ok {
		return fmt.Errorf("invalid message")
	}
	events, err := c.getEvents(ctx, req)
	if err != nil {
		if !c.settings.EnableErrorPort {
			return err
		}
		return handler(ctx, ErrorPort, Error{
			Context: req.Context,
			Error:   err.Error(),
		})
	}

	return handler(ctx, ResponsePort, Response{
		Context: req.Context,
		Results: *events,
	})
}

func (c *Component) getEvents(ctx context.Context, req Request) (*calendar.Events, error) {

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

	call := srv.Events.List(req.CalendarId).ShowDeleted(req.ShowDeleted).SingleEvents(true)

	if !req.StartDate.IsZero() {
		call.TimeMin(req.StartDate.Format(time.RFC3339))
	}
	if !req.EndDate.IsZero() {
		call.TimeMax(req.EndDate.Format(time.RFC3339))
	}

	if req.PageToken != "" {
		call.PageToken(req.PageToken)
	}
	if req.SyncToken != "" {
		call.SyncToken(req.SyncToken)
	}

	call.MaxResults(100).OrderBy("startTime")

	events, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve user's events: %v", err)
	}

	return events, nil
}

func (c *Component) Ports() []module.Port {
	ports := []module.Port{
		{
			Name:          module.SettingsPort,
			Label:         "Settings",
			Configuration: Settings{},
			Source:        true,
		},
		{
			Name:  RequestPort,
			Label: "Request",
			Configuration: Request{
				Config: etc.ClientConfig{
					Scopes: []string{"https://www.googleapis.com/auth/calendar.events.readonly"},
				},
				CalendarId: "SomeID",
				Token: etc.Token{
					TokenType: "Bearer",
				},
			},
			Source:   true,
			Position: module.Left,
		},
		{
			Name:          ResponsePort,
			Label:         "Response",
			Source:        false,
			Position:      module.Right,
			Configuration: Response{},
		}}

	if !c.settings.EnableErrorPort {
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

func (c *Component) Instance() module.Component {
	return &Component{}
}

var _ module.Component = (*Component)(nil)

func init() {
	registry.Register(&Component{})
}
