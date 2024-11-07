package listen_collection

import (
	"context"
	"errors"
	firebase "firebase.google.com/go"
	"fmt"
	"github.com/tiny-systems/googleapis-module/components/etc"
	"github.com/tiny-systems/googleapis-module/components/firestore/utils"
	"github.com/tiny-systems/module/module"
	"github.com/tiny-systems/module/registry"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ComponentName = "listen_collection"
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
	Context    Context          `json:"context,omitempty" title:"Context" configurable:"true"`
	Config     etc.ClientConfig `json:"config" title:"Config"  required:"true" description:"Client Config"`
	Collection string           `json:"collection" title:"Collection" required:"true"`
	Wheres     []utils.Where    `json:"wheres,omitempty" title:"Where" description:"Where to filter. Leave empty if you want to listen the entire collection."`
}

type Response struct {
	Context  Context                `json:"context" title:"Context"`
	RefID    string                 `json:"refID"`
	Document map[string]interface{} `json:"document" title:"Document"`
	Exists   bool                   `json:"exists" title:"Exists"`
}

type Error struct {
	Context Context `json:"context"`
	Error   string  `json:"error"`
}

func (g *Component) GetInfo() module.ComponentInfo {
	return module.ComponentInfo{
		Name:        ComponentName,
		Description: "Firestore Listen Collection",
		Info:        "Listens to changes of the collection",
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

	ref := db.Collection(req.Collection)
	q := ref.Query

	if len(req.Wheres) > 0 {
		for _, w := range req.Wheres {
			q = q.Where(w.Path, w.Operation, w.Value)
		}
	}

	iter := q.Snapshots(ctx)
	for {
		snap, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if status.Code(err) == codes.DeadlineExceeded {
			break
		}

		docs, err := snap.Documents.GetAll()
		if err != nil {
			continue
		}

		for _, doc := range docs {
			resp := Response{
				Context:  req.Context,
				Document: doc.Data(),
				Exists:   doc.Exists(),
			}
			if doc.Ref != nil {
				resp.RefID = doc.Ref.ID
			}
			_ = output(ctx, ResponsePort, resp)
		}
	}
	return nil
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
	//

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
