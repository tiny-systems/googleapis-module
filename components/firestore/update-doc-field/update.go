package create

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
	ComponentName = "firestore_update_doc_field"
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

type Update struct {
	Path  string      `json:"path" title:"Path" required:"true"`
	Value interface{} `json:"value" configurable:"true" title:"Value"`
}

type Request struct {
	Context    Context          `json:"context,omitempty" title:"Context" configurable:"true"`
	Config     etc.ClientConfig `json:"config" title:"Config"  required:"true" description:"Client Config"`
	Collection string           `json:"collection" title:"Collection" required:"true"`
	RefID      string           `json:"refID" required:"true"  title:"Ref ID"`
	Updates    []Update         `json:"updates" title:"Updates" required:"true"`
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
		Description: "Firestore Update Document Fields",
		Info:        "Updates fields in the existing record",
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

	var err error

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

	ref := db.Collection(req.Collection).Doc(req.RefID)
	//
	var updates []firestore.Update
	for _, update := range req.Updates {
		updates = append(updates, firestore.Update{
			Path:  update.Path,
			Value: update.Value,
		})
	}
	_, err = ref.Update(ctx, updates)

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
	return output(ctx, ResponsePort, Response{
		Context: req.Context,
		RefID:   ref.ID,
		RefPath: ref.Path,
	})
}

func (g *Component) Ports() []module.Port {
	ports := []module.Port{
		{
			Name:          module.SettingsPort,
			Label:         "Settings",
			Configuration: Settings{},
		},
		{
			Name:          RequestPort,
			Label:         "Request",
			Position:      module.Left,
			Configuration: Request{},
		},
	}

	//
	if g.settings.EnableResponsePort {
		ports = append(ports, module.Port{
			Source:        true,
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
