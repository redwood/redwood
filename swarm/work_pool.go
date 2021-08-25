package swarm

import (
	"context"

	"redwood.dev/process"
)

type WorkPool struct {
	process.Process
	chJobs         chan interface{}
	chJobsComplete chan struct{}
}

func NewWorkPool(jobs []interface{}) *WorkPool {
	p := &WorkPool{
		Process:        *process.New("work pool"),
		chJobs:         make(chan interface{}, len(jobs)),
		chJobsComplete: make(chan struct{}),
	}
	for _, job := range jobs {
		p.chJobs <- job
	}
	return p
}

func (p *WorkPool) Start() error {
	err := p.Process.Start()
	if err != nil {
		return err
	}
	defer p.Process.Autoclose()

	p.Process.Go("await completion", func(ctx context.Context) {
		numJobs := len(p.chJobs)
		for i := 0; i < numJobs; i++ {
			select {
			case <-p.chJobsComplete:
			case <-ctx.Done():
				return
			}
		}
	})
	return nil
}

func (p *WorkPool) NextJob() (job interface{}, ok bool) {
	select {
	case <-p.Ctx().Done():
		return nil, false
	case job = <-p.chJobs:
		return job, true
	}
}

func (p *WorkPool) ReturnFailedJob(job interface{}) {
	select {
	case <-p.Ctx().Done():
	case p.chJobs <- job:
	}
}

func (p *WorkPool) MarkJobComplete() {
	select {
	case <-p.Ctx().Done():
	case p.chJobsComplete <- struct{}{}:
	}
}
