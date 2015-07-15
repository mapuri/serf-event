package serfer

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/hashicorp/serf/client"
)

// HandlerFunc is a function type to handle registered serf events
type HandlerFunc func(name string, payload []byte) error

// ResponderFunc is a function type to respond to registered serf queries
type ResponderFunc func(name string, request []byte) ([]byte, error)

// Router implements a serf event/query router. It provides for ease of
// registering structured event/query names by using sub-routers. The names
// are first looked against the immediate router for an exact match and then
// matched in sub-routers (picking the sub-router with longest prefix matched
// in case of overlapping prefixes).
type Router struct {
	sync.Mutex
	prefix     string
	handlers   map[string]interface{}
	subRouters map[string]*Router
}

// NewRouter instantiates an instance of main router.
func NewRouter() *Router {
	return &Router{
		handlers:   make(map[string]interface{}),
		subRouters: make(map[string]*Router),
	}
}

// NewSubRouter instantiates an instance of a sub-router under a router and
// associates it with the specified prefix.
func (r *Router) NewSubRouter(prefix string) *Router {
	var sr *Router

	sr = NewRouter()
	sr.prefix = prefix
	r.Lock()
	r.subRouters[prefix] = sr
	r.Unlock()
	return sr
}

// AddHandler registers a handler for the specified event
func (r *Router) AddHandler(name string, f HandlerFunc) {
	r.Lock()
	r.handlers[name] = f
	r.Unlock()
}

// AddMemberJoinHandler registers a handler for serf's member-join event
func (r *Router) AddMemberJoinHandler(f HandlerFunc) {
	r.AddHandler("member-join", f)
}

// AddMemberLeaveHandler registers a handler for serf's member-leave event
func (r *Router) AddMemberLeaveHandler(f HandlerFunc) {
	r.AddHandler("member-leave", f)
}

// AddMemberFailedHandler registers a handler for serf's member-failed event
func (r *Router) AddMemberFailedHandler(f HandlerFunc) {
	r.AddHandler("member-failed", f)
}

// AddResponder registers a responder for the specified query
func (r *Router) AddResponder(name string, f ResponderFunc) {
	r.Lock()
	r.handlers[name] = f
	r.Unlock()
}

func (r *Router) findHandlerFunc(name string) interface{} {
	var sortedKeys []string

	// try for exact match first
	if f, ok := r.handlers[name]; ok {
		return f
	}

	// else try in one of sub-routers.
	// Note: to perform longest prefix match, sort the keys in reverse order and
	// pick the first key with prefix overlap.
	for key := range r.subRouters {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(sortedKeys)))

	for _, p := range sortedKeys {
		sr := r.subRouters[p]
		if strings.HasPrefix(name, p) {
			if f := sr.findHandlerFunc(name[len(p):]); f != nil {
				return f
			}
		}
	}

	return nil
}

func (r *Router) handleEvent(event map[string]interface{}) {
	r.Lock()
	defer r.Unlock()
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
	r.Lock()
	defer r.Unlock()
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

// InitSerfAndServe initializes a serf client for agent running at specified
// IP address and enters the event/query serving loop.
// If an empty IP address the the client tries to reach the agent at 127.0.0.1 address
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

// InitSerfFromConfigAndServe initializes a serf client with specified serf's
// client configuration and enters the event/query serving loop.
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
