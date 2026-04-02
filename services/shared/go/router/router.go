package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Middleware func(http.Handler) http.Handler

type Router struct {
	mux *chi.Mux
}

func New() *Router {
	return &Router{mux: chi.NewRouter()}
}

// Use appends middleware to the router's global middleware stack.
func (r *Router) Use(mw ...Middleware) {
	for _, m := range mw {
		r.mux.Use(m)
	}
}

// Group creates a new Group bound to this router's mux.
func (r *Router) Group() *Group {
	return NewGroup(r.mux)
}

func (r *Router) Get(path string, handlerFn http.HandlerFunc) {
	r.mux.Get(path, handlerFn)
}

func (r *Router) Post(path string, handlerFn http.HandlerFunc) {
	r.mux.Post(path, handlerFn)
}

func (r *Router) Put(path string, handlerFn http.HandlerFunc) {
	r.mux.Put(path, handlerFn)
}

func (r *Router) Delete(path string, handlerFn http.HandlerFunc) {
	r.mux.Delete(path, handlerFn)
}

func (r *Router) Patch(path string, handlerFn http.HandlerFunc) {
	r.mux.Patch(path, handlerFn)
}

func (r *Router) Head(path string, handlerFn http.HandlerFunc) {
	r.mux.Head(path, handlerFn)
}

// Handler returns the underlying http.Handler for use with http.Server.
func (r *Router) Handler() http.Handler {
	return r.mux
}

// Mux returns the underlying chi.Mux for use with docgen or direct access.
func (r *Router) Mux() *chi.Mux {
	return r.mux
}
