package channel_watch

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
	ComponentName = "calendar_channel_watch"
	RequestPort   = "request"
	ResponsePort  = "response"
	ErrorPort     = "error"
)

type Channel struct {
	ID          string `json:"id" required:"true" title:"ID" description:"A UUID or similar unique string that identifies this channel."`
	Type        string `json:"type" required:"true" title:"Type" enum:"web_hook" enumTitles:"Webhook" description:"The type of delivery mechanism used for this channel. Valid values are \"web_hook\" (or \"webhook\"). Both values refer to a channel where Http requests are used to deliver messages."`
	Address     string `json:"address" required:"true" title:"Address" description:"The address where notifications are delivered for this channel."`
	Expiration  int64  `json:"expiration" title:"Expiration" description:"Date and time of notification channel expiration, expressed as a Unix timestamp, in milliseconds."`
	ResourceId  string `json:"resourceId" title:"ResourceID" description:"An opaque ID that identifies the resource being watched on this channel. Stable across different API versions."`
	ResourceUri string `json:"resourceUri" title:"ResourceURI" description:"A version-specific identifier for the watched resource."`
	Token       string `json:"token" title:"Auth Token" description:"An arbitrary string delivered to the target address with each notification delivered over this channel."`
}

type Settings struct {
	EnableErrorPort bool `json:"enableErrorPort" required:"true" title:"Enable Error Port" description:"If request may fail, error port will emit an error message"`
}

type Context any

type Request struct {
	Context  Context          `json:"context,omitempty" configurable:"true" title:"Context" description:"Arbitrary message to be send further"`
	Calendar Calendar         `json:"calendar" required:"true" title:"Calendar"`
	Channel  Channel          `json:"channel" required:"true" title:"Channel"`
	Token    etc.Token        `json:"token" required:"true" title:"Token"`
	Config   etc.ClientConfig `json:"config" required:"true" title:"Client credentials"`
}

type Calendar struct {
	ID string `json:"id" required:"true" title:"Calendar ID" description:"Google Calendar ID to be watched"`
}

type WatchChannel struct {
	ID string `json:"id"`
}

type Response struct {
	Context Context      `json:"context"`
	Channel WatchChannel `json:"channel"`
}

type Error struct {
	Context Context `json:"context"`
	Error   string  `json:"error"`
}

type Component struct {
	settings Settings
}

func (h *Component) Instance() module.Component {
	return &Component{
		settings: Settings{},
	}
}

func (h *Component) GetInfo() module.ComponentInfo {
	return module.ComponentInfo{
		Name:        ComponentName,
		Description: "Watch Calendar Channel",
		Info:        "Register calendar watcher",
		Tags:        []string{"Google", "Calendar", "Watch"},
	}
}

func (h *Component) Handle(ctx context.Context, handler module.Handler, port string, msg interface{}) error {
	if port == module.SettingsPort {
		in, ok := msg.(Settings)
		if !ok {
			return fmt.Errorf("invalid settings")
		}
		h.settings = in
		return nil
	}

	if port != ResponsePort {
		return fmt.Errorf("unknown port %s", port)
	}

	req, ok := msg.(Request)
	if !ok {
		return fmt.Errorf("invalid message")
	}

	ch, err := h.watch(ctx, req)
	if err != nil {
		if !h.settings.EnableErrorPort {
			return err
		}
		return handler(ctx, ErrorPort, Error{
			Context: req.Context,
			Error:   err.Error(),
		})
	}

	return handler(ctx, ResponsePort, Response{
		Context: req.Context,
		Channel: WatchChannel{
			ID: ch.Id,
		},
	})
}

func (h *Component) watch(ctx context.Context, req Request) (*calendar.Channel, error) {
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

	return srv.Events.Watch(req.Calendar.ID, &calendar.Channel{
		Type:       req.Channel.Type,
		Address:    req.Channel.Address,
		Token:      req.Channel.Token,
		Id:         req.Channel.ID,
		Expiration: req.Channel.Expiration,
	}).Do()
}

func (h *Component) Ports() []module.Port {
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
				Channel: Channel{
					Type: "web_hook",
				},
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
		},
	}
	if !h.settings.EnableErrorPort {
		return ports
	}
	return append(ports, module.Port{
		Name:          ErrorPort,
		Label:         "Error",
		Source:        false,
		Position:      module.Bottom,
		Configuration: Error{},
	})
}

var _ module.Component = (*Component)(nil)

func init() {
	registry.Register(&Component{})
}
