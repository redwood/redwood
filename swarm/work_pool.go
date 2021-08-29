package swarm

import (
	"context"

	"redwood.dev/process"
)

type WorkPool struct {
	process.Process
	jobs           map[interface{}]int
	chJobs         chan interface{}
	chJobsComplete chan struct{}
}

func NewWorkPool(jobs []interface{}) *WorkPool {
	p := &WorkPool{
		Process:        *process.New("work pool"),
		jobs:           make(map[interface{}]int),
		chJobs:         make(chan interface{}, len(jobs)),
		chJobsComplete: make(chan struct{}),
	}
	for i, job := range jobs {
		p.jobs[job] = i
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

	p.Process.Go(nil, "await completion", func(ctx context.Context) {
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

func (p *WorkPool) NextJob() (job interface{}, i int, ok bool) {
	select {
	case <-p.Ctx().Done():
		return nil, 0, false
	case job = <-p.chJobs:
		return job, p.jobs[job], true
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

func (p *WorkPool) NumJobs() int {
	return len(p.jobs)
}
