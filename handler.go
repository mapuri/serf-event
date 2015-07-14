package serf_event

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/serf/client"
)

type HandlerFunc func(name string, payload []byte) error
type ResponderFunc func(name string, request []byte) ([]byte, error)

type Router struct {
	prefix     string
	handlers   map[string]interface{}
	subRouters map[string]*Router
}

func NewRouter() *Router {
	return &Router{
		handlers:   make(map[string]interface{}),
		subRouters: make(map[string]*Router),
	}
}

func (r *Router) NewSubRouter(prefix string) *Router {
	var sr *Router

	sr = NewRouter()
	sr.prefix = prefix
	r.subRouters[prefix] = sr
	return sr
}

func (r *Router) AddHandler(name string, f HandlerFunc) {
	r.handlers[name] = f
}

func (r *Router) AddMemberJoinHandler(f HandlerFunc) {
	r.handlers["member-join"] = f
}

func (r *Router) AddMemberLeaveHandler(f HandlerFunc) {
	r.handlers["member-leave"] = f
}

func (r *Router) AddMemberFailedHandler(f HandlerFunc) {
	r.handlers["member-failed"] = f
}

func (r *Router) AddQueryResponder(name string, f ResponderFunc) {
	r.handlers[name] = f
}

func (r *Router) findHandlerFunc(name string) interface{} {
	// try for exact match first
	if f, ok := r.handlers[name]; ok {
		return f
	}

	// else try in one of sub-routers
	for p, sr := range r.subRouters {
		if strings.HasPrefix(name, p) {
			if f := sr.findHandlerFunc(name[len(p):]); f != nil {
				return f
			}
		}
	}

	return nil
}

func (r *Router) handleEvent(event map[string]interface{}) {
	var (
		name        string
		payload     []byte
		handlerFunc HandlerFunc
		ok          bool
	)
	name = event["Name"].(string)
	payload = event["Payload"].([]byte)

	if f := r.findHandlerFunc(name); f == nil {
		log.Infof("no handler for event: %q", name)
		return
	} else if handlerFunc, ok = f.(HandlerFunc); !ok {
		log.Infof("no handler for event: %q", name)
		return
	}

	if err := handlerFunc(name, payload); err != nil {
		log.Infof("event handler failed. Error: %s", err)
		// failure returned by handlers are not considered fatal
		// TODO: handle panics inside event handlers as well
		return
	}
}

func (r *Router) handleQuery(serfClient *client.RPCClient, query map[string]interface{}) {
	var (
		name        string
		payload     []byte
		response    []byte
		handlerFunc ResponderFunc
		ok          bool
		err         error
	)
	name = query["Name"].(string)
	payload = query["Payload"].([]byte)

	if f := r.findHandlerFunc(name); f == nil {
		log.Infof("no handler for query: %q", name)
		return
	} else if handlerFunc, ok = f.(ResponderFunc); !ok {
		log.Infof("no handler for query: %q", name)
		return
	}

	if response, err = handlerFunc(name, payload); err != nil {
		log.Infof("query handler failed. Error: %s", err)
		// failure returned by handlers are not considered fatal
		// TODO: handle panics inside event handlers as well
		return
	}

	if err := serfClient.Respond(query["ID"].(uint64), response); err != nil {
		log.Errorf("responding to query failed. Response body: %v, Error: %s", response, err)
	}
}

func (r *Router) serve(serfClient *client.RPCClient) error {
	var (
		eventCh chan map[string]interface{}
	)

	// register for member events, user events and queries
	eventCh = make(chan map[string]interface{})
	if _, err := serfClient.Stream("member,user,query", eventCh); err != nil {
		return fmt.Errorf("failed to initialize event stream. Error: %s", err)
	}

	select {
	case e := <-eventCh:
		log.Infof("Event received: %+v", e)
		if e["Event"] == "query" {
			r.handleQuery(serfClient, e)
		} else {
			r.handleEvent(e)
		}
	}

	return fmt.Errorf("Unexpected code path!")
}

func (r *Router) InitSerfAndServe(addr string) error {
	var (
		c   *client.RPCClient
		err error
	)

	if c, err = client.NewRPCClient(addr); err != nil {
		return err
	}
	return r.serve(c)
}

func (r *Router) InitSerfFromConfigAndServe(serfConfig *client.Config) error {
	var (
		c   *client.RPCClient
		err error
	)

	if c, err = client.ClientFromConfig(serfConfig); err != nil {
		return err
	}
	return r.serve(c)
}
