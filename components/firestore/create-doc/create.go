package create_doc

import (
	"cloud.google.com/go/firestore"
	"context"
	firebase "firebase.google.com/go"
	"fmt"
	"github.com/tiny-systems/googleapis-module/components/etc"
	"github.com/tiny-systems/module/module"
	"github.com/tiny-systems/module/registry"
	"google.golang.org/api/option"
)

const (
	ComponentName = "firestore_create_doc"
	RequestPort   = "request"
	ResponsePort  = "response"
	ErrorPort     = "error"
)

type Context any

type Settings struct {
	EnableErrorPort    bool `json:"enableErrorPort" required:"true" title:"Enable Error Port" description:"If request may fail, error port will emit an error message"`
	EnableResponsePort bool `json:"enableResponsePort" required:"true" title:"Enable Response Port" description:""`
}

type Component struct {
	settings Settings
}

type Request struct {
	Context    Context                `json:"context,omitempty" title:"Context" configurable:"true"`
	Config     etc.ClientConfig       `json:"config" title:"Config" required:"true" description:"Client Config"`
	Collection string                 `json:"collection" title:"Collection" required:"true"`
	RefID      string                 `json:"refID,omitempty" title:"RefID" description:"Optional"`
	Document   map[string]interface{} `json:"document" configurable:"true" title:"Document" required:"true"`
}

type Response struct {
	Context Context `json:"context" title:"Context"`
	RefID   string  `json:"refID"`
	RefPath string  `json:"refPath"`
}

type Error struct {
	Context Context `json:"context"`
	Error   string  `json:"error"`
}

func (g *Component) GetInfo() module.ComponentInfo {
	return module.ComponentInfo{
		Name:        ComponentName,
		Description: "Firestore Create Record",
		Info:        "Creates a new record in a collection",
		Tags:        []string{"google", "firestore", "db"},
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

	req, ok := msg.(Request)
	if !ok {
		return fmt.Errorf("invalid request")
	}

	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(req.Config.Credentials)), option.WithScopes(req.Config.Scopes...))
	if err != nil {
		// check err port
		if !g.settings.EnableErrorPort {
			return err
		}
		return output(ctx, ErrorPort, Error{
			Context: req.Context,
			Error:   err.Error(),
		})
	}

	db, err := app.Firestore(ctx)

	if err != nil {
		// check err port
		if !g.settings.EnableErrorPort {
			return err
		}
		return output(ctx, ErrorPort, Error{
			Context: req.Context,
			Error:   err.Error(),
		})
	}

	col := db.Collection(req.Collection)

	var ref *firestore.DocumentRef

	if req.RefID != "" {
		ref = col.Doc(req.RefID)
		_, err = ref.Set(ctx, req.Document)
	} else {
		ref, _, err = col.Add(ctx, req.Document)
	}

	if err != nil {
		// check err port
		if !g.settings.EnableErrorPort {
			return err
		}
		return output(ctx, ErrorPort, Error{
			Context: req.Context,
			Error:   err.Error(),
		})
	}

	if !g.settings.EnableResponsePort {
		return nil
	}

	resp := Response{
		Context: req.Context,
	}

	if ref != nil {
		resp.RefID = ref.ID
		resp.RefPath = ref.Path
	}

	return output(ctx, ResponsePort, resp)
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
	}

	//
	if g.settings.EnableResponsePort {
		ports = append(ports, module.Port{
			Source:        false,
			Name:          ResponsePort,
			Label:         "Response",
			Position:      module.Right,
			Configuration: Response{},
		})
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
