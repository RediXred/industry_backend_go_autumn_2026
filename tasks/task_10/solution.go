package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var taskId atomic.Uint64

type CreateTaskBody struct {
	Title *string `json:"title"`
}

type PatchTaskBody struct {
	Done *bool `json:"done"`
}

type Task struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type TaskRepo interface {
	Create(title string) (Task, error)
	Get(id string) (Task, bool)
	List(done bool) []Task
	SetDone(id string, done bool) (Task, error)
}
type Clock interface{ Now() time.Time }

var (
	ErrNotFound     = errors.New("task not found")
	ErrInvalidTitle = errors.New("invalid title")
)

type inMemoryTaskRepo struct {
	mu    sync.RWMutex
	clock Clock
	seq   uint64
	tasks map[string]Task
}

func NewInMemoryTaskRepo(clock Clock) TaskRepo {
	return &inMemoryTaskRepo{
		clock: clock,
		tasks: make(map[string]Task),
	}
}
func (r *inMemoryTaskRepo) Create(title string) (Task, error) {
	if title = strings.TrimSpace(title); title == "" {
		return Task{}, ErrInvalidTitle
	}

	task := Task{
		ID: strconv.FormatUint(taskId.Add(1), 10),
		Title: title,
		UpdatedAt: r.clock.Now(),
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = task

	return task, nil
}
func (r *inMemoryTaskRepo) Get(id string) (Task, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.tasks[id]
	return v, ok
}
func (r *inMemoryTaskRepo) List(done bool) []Task {
	r.mu.RLock()
	rez := make([]Task, 0, len(r.tasks))
	for _, v := range r.tasks {
		if v.Done == done {
			rez = append(rez, v)
		}
	}
	r.mu.RUnlock()
	slices.SortFunc(rez, func(a, b Task) int {
		if k := b.UpdatedAt.Compare(a.UpdatedAt); k != 0 {
			return k
		}

		return cmp.Compare(a.ID, b.ID)
	})
	return rez
}
func (r *inMemoryTaskRepo) SetDone(id string, done bool) (Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.tasks[id]
	if !ok {
		return Task{}, ErrNotFound
	}

	if v.Done != done {
		v.UpdatedAt = r.clock.Now()
		v.Done = done
		r.tasks[id] = v
	}
	return v, nil
}

type httpHandler struct{ repo TaskRepo }

func NewHTTPHandler(repo TaskRepo) http.Handler {
	return &httpHandler{repo}
}
func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/tasks" {
		if r.Method == http.MethodPost {
			h.handleCreate(w, r)
		} else if r.Method == http.MethodGet {
			h.handleList(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if strings.HasPrefix(r.URL.Path, "/tasks/") {
		id := strings.TrimPrefix(r.URL.Path, "/tasks/")
		if id == "" || strings.Index(id, "/") != -1 {
			http.NotFound(w, r)
			return
		}

		if r.Method == http.MethodGet {
			h.handleGet(w, r, id)
		} else if r.Method == http.MethodPatch {
			h.handlePatch(w, r, id)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	http.NotFound(w, r)
}
func (h *httpHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var body CreateTaskBody
	if err := decodeStrictJSON(r.Body, &body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if body.Title == nil {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	v, err := h.repo.Create(*body.Title)

	if err == nil {
		writeJSON(w, http.StatusCreated, v)
		return
	}
	if errors.Is(err, ErrInvalidTitle) {
		http.Error(w, "invalid title", http.StatusBadRequest)
	} else {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
func (h *httpHandler) handleGet(w http.ResponseWriter, r *http.Request, id string) {
	v, ok := h.repo.Get(id)
	if ok {
		writeJSON(w, http.StatusOK, v)
	} else {
		http.NotFound(w, r)
	}
}
func (h *httpHandler) handleList(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()["done"]
	if len(v) > 1 || (len(v) == 1 && v[0] != "true" && v[0] != "false") {
		http.Error(w, "invalid done param", http.StatusBadRequest)
		return
	}
	done := len(v) == 1 && v[0] == "true"
	t := h.repo.List(done)
	writeJSON(w, http.StatusOK, t)
}
func (h *httpHandler) handlePatch(w http.ResponseWriter, r *http.Request, id string) {
	var body PatchTaskBody
	if err := decodeStrictJSON(r.Body, &body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if body.Done == nil {
		http.Error(w, "done required", http.StatusBadRequest)
		return
	}

	v, err := h.repo.SetDone(id, *body.Done)
	if err == nil {
		writeJSON(w, http.StatusOK, v)
		return
	}
	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
	} else {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
func decodeStrictJSON(r io.Reader, v any) error {
	var ext any
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}

	err := dec.Decode(&ext)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return errors.New("trailing data")
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
