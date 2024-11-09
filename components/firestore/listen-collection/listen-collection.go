package listen_collection

import (
	"cloud.google.com/go/firestore"
	"context"
	"errors"
	firebase "firebase.google.com/go"
	"fmt"
	"github.com/tiny-systems/googleapis-module/components/etc"
	"github.com/tiny-systems/googleapis-module/components/firestore/utils"
	"github.com/tiny-systems/module/module"
	"github.com/tiny-systems/module/registry"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sync"
)

const (
	ComponentName = "firestore_listen_collection"
	ResponsePort  = "response"
	StartPort     = "start"
	StopPort      = "stop"
	ErrorPort     = "error"
)

type Context any

type StartControl struct {
	Status string `json:"status" title:"Status" readonly:"true"`
}

type StopControl struct {
	Stop   bool   `json:"stop" format:"button" title:"Stop" required:"true" description:"Stop listening"`
	Status string `json:"status" title:"Status" readonly:"true"`
}

type Stop struct {
}

type Settings struct {
	EnableErrorPort bool `json:"enableErrorPort" required:"true" title:"Enable Error Port" description:"If request may fail, error port will emit an error message"`
	EnableStopPort  bool `json:"enableStopPort" required:"true" title:"Enable stop port" description:"Stop port allows you to stop listener"`
}

type Component struct {
	settings Settings

	startSettings Start

	cancelFunc     context.CancelFunc
	cancelFuncLock *sync.Mutex

	runLock *sync.Mutex
}

type Start struct {
	Context    Context          `json:"context,omitempty" title:"Context" configurable:"true"`
	Config     etc.ClientConfig `json:"config" title:"Config"  required:"true" description:"Client Config"`
	Collection string           `json:"collection" title:"Collection" required:"true"`
	Wheres     []utils.Where    `json:"wheres,omitempty" title:"Where" description:"Where to filter. Leave empty if you want to listen the entire collection."`
}

type Response struct {
	Context  Context                `json:"context" title:"Context"`
	RefID    string                 `json:"refID"`
	Document map[string]interface{} `json:"document" title:"Document" description:"Document that changed"`
	Action   string                 `json:"action" title:"Action" enum:"added,modified,removed"`
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

func (g *Component) Handle(ctx context.Context, handler module.Handler, port string, msg interface{}) error {

	switch port {

	case module.SettingsPort:
		in, ok := msg.(Settings)
		if !ok {
			return fmt.Errorf("invalid settings")
		}
		g.settings = in
		return nil

	case module.ControlPort:
		if msg == nil {
			break
		}
		switch msg.(type) {
		case StartControl:
			return g.start(ctx, handler)

		case StopControl:
			return g.stop()
		}
	case StartPort:
		req, ok := msg.(Start)
		if !ok {
			return fmt.Errorf("invalid request")
		}

		g.startSettings = req
		return g.start(ctx, handler)
	}
	return fmt.Errorf("invalid port")
}

func (g *Component) start(ctx context.Context, handler module.Handler) error {

	g.runLock.Lock()
	defer g.runLock.Unlock()

	listenCtx, listenCancel := context.WithCancel(ctx)
	defer listenCancel()

	g.setCancelFunc(listenCancel)
	_ = handler(listenCtx, module.ReconcilePort, nil)

	defer func() {
		g.setCancelFunc(nil)
		_ = handler(context.Background(), module.ReconcilePort, nil)
	}()

	app, err := firebase.NewApp(listenCtx, nil, option.WithCredentialsJSON([]byte(g.startSettings.Config.Credentials)), option.WithScopes(g.startSettings.Config.Scopes...))
	if err != nil {
		// check err port
		if !g.settings.EnableErrorPort {
			return err
		}
		return handler(listenCtx, ErrorPort, Error{
			Context: g.startSettings.Context,
			Error:   err.Error(),
		})
	}

	db, err := app.Firestore(listenCtx)

	if err != nil {
		// check err port
		if !g.settings.EnableErrorPort {
			return err
		}
		return handler(listenCtx, ErrorPort, Error{
			Context: g.startSettings.Context,
			Error:   err.Error(),
		})
	}

	ref := db.Collection(g.startSettings.Collection)
	q := ref.Query

	if len(g.startSettings.Wheres) > 0 {
		for _, w := range g.startSettings.Wheres {
			q = q.Where(w.Path, w.Operation, w.Value)
		}
	}

	iter := q.Snapshots(listenCtx)
	for {

		snap, err := iter.Next()
		// DeadlineExceeded will be returned when ctx is cancelled.
		if status.Code(err) == codes.DeadlineExceeded {
			return nil
		}
		if errors.Is(listenCtx.Err(), context.Canceled) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("snapshots next: %w", err)
		}

		if snap == nil {
			continue
		}

		for _, change := range snap.Changes {

			var action string
			switch change.Kind {
			case firestore.DocumentAdded:
				action = "added"
			case firestore.DocumentModified:
				action = "modified"
			case firestore.DocumentRemoved:
				action = "removed"
			}

			resp := Response{
				Context: g.startSettings.Context,
				Action:  action,
			}
			if change.Doc != nil {
				resp.Document = change.Doc.Data()
				if change.Doc.Ref != nil {
					resp.RefID = change.Doc.Ref.ID
				}
			}
			_ = handler(listenCtx, ResponsePort, resp)
		}
	}

}

func (g *Component) stop() error {
	g.cancelFuncLock.Lock()
	defer g.cancelFuncLock.Unlock()
	if g.cancelFunc == nil {
		return nil
	}
	g.cancelFunc()

	return nil
}

func (g *Component) setCancelFunc(f func()) {
	g.cancelFuncLock.Lock()
	defer g.cancelFuncLock.Unlock()
	g.cancelFunc = f
}

func (g *Component) isListening() bool {
	g.cancelFuncLock.Lock()
	defer g.cancelFuncLock.Unlock()

	return g.cancelFunc != nil
}

func (g *Component) getControl() interface{} {
	if g.isListening() {
		return StopControl{
			Status: "Listening",
		}
	}
	return StartControl{
		Status: "Not listening",
	}
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
			Name:          StartPort,
			Label:         "Start",
			Position:      module.Left,
			Configuration: g.startSettings,
		},
		{
			Name:          module.ControlPort,
			Label:         "Dashboard",
			Configuration: g.getControl(),
		},
		{
			Source:        false,
			Name:          ResponsePort,
			Label:         "Response",
			Position:      module.Right,
			Configuration: Response{},
		},
	}

	// programmatically stop server
	if g.settings.EnableStopPort {
		ports = append(ports, module.Port{
			Position:      module.Left,
			Name:          StopPort,
			Label:         "Stop",
			Source:        true,
			Configuration: Stop{},
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
	return &Component{
		cancelFuncLock: &sync.Mutex{},
		runLock:        &sync.Mutex{},
		startSettings:  Start{},
	}
}

var _ module.Component = (*Component)(nil)

func init() {
	registry.Register(&Component{})
}
