package lock

import (
	"context"
	"fmt"
	"os"

	"github.com/aman-churiwal/gridflow-shared/logger"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const electionKey = "/gridflow/optimizer/leader"
const sessionTTL = 10

type Election struct {
	session    *concurrency.Session
	election   *concurrency.Election
	instanceID string
	logger     *logger.Logger
}

func NewElection(client *clientv3.Client, appLogger *logger.Logger) (*Election, error) {
	hostname, _ := os.Hostname()
	instanceID := fmt.Sprintf("%s-%d", hostname, os.Getpid())

	session, err := concurrency.NewSession(client, concurrency.WithTTL(sessionTTL))
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd session: %w", err)
	}

	election := concurrency.NewElection(session, electionKey)

	return &Election{
		session:    session,
		election:   election,
		instanceID: instanceID,
		logger:     appLogger,
	}, nil
}

func (e *Election) Campaign(ctx context.Context) error {
	return e.election.Campaign(ctx, e.instanceID)
}

func (e *Election) SessionDone() <-chan struct{} {
	return e.session.Done()
}

func (e *Election) Resign(ctx context.Context) error {
	return e.election.Resign(ctx)
}

func (e *Election) Close() error {
	return e.session.Close()
}

func (e *Election) InstanceID() string {
	return e.instanceID
}
