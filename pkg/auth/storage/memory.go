package storage

import (
	"context"
	"errors"
	"sync"

	"github.com/ory/fosite"
	"github.com/ory/fosite/storage"
)

type ClientRegistry interface {
	SetClient(_ context.Context, c fosite.Client) error
	RemoveClient(_ context.Context, id string) error
}

var _ ClientRegistry = &MemoryStore{}

func NewStore() *MemoryStore {
	store := storage.NewMemoryStore()

	return &MemoryStore{
		MemoryStore: store,
		clients:     map[string]fosite.Client{},
	}
}

type MemoryStore struct {
	*storage.MemoryStore
	clients      map[string]fosite.Client
	clientsMutex sync.RWMutex
}

func (s *MemoryStore) GetClient(_ context.Context, id string) (fosite.Client, error) {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()

	cl, ok := s.clients[id]
	if !ok {
		return nil, fosite.ErrNotFound
	}
	return cl, nil
}

func (s *MemoryStore) SetTokenLifespans(clientID string, lifespans *fosite.ClientLifespanConfig) error {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	if client, ok := s.clients[clientID]; ok {
		if clc, ok := client.(*fosite.DefaultClientWithCustomTokenLifespans); ok {
			clc.SetTokenLifespans(lifespans)
			return nil
		}
		return fosite.ErrorToRFC6749Error(errors.New("failed to set token lifespans due to failed client type assertion"))
	}
	return fosite.ErrNotFound
}

func (s *MemoryStore) SetClient(_ context.Context, c fosite.Client) error {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	s.clients[c.GetID()] = c
	return nil
}

func (s *MemoryStore) RemoveClient(_ context.Context, id string) error {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	delete(s.clients, id)
	return nil
}
