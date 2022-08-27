package health

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"time"

	"github.com/google/pprof/profile"

	"redwood.dev/log"
	"redwood.dev/process"
	"redwood.dev/utils"
)

type Nurse struct {
	process.Process

	cfg      NurseConfig
	checks   map[string]CheckFunc
	checksMu sync.RWMutex
	chGather chan gatherRequest
}

type NurseConfig struct {
	ProfileRoot    string
	PollInterval   time.Duration
	GatherDuration time.Duration
	MaxProfileSize utils.FileSize

	CPUProfileRate       int
	MemProfileRate       int
	BlockProfileRate     int
	MutexProfileFraction int

	MemThreshold       utils.FileSize
	GoroutineThreshold int
}

type CheckFunc func() (unwell bool, meta Meta)

type gatherRequest struct {
	reason string
	meta   Meta
}

type Meta []any

const profilePerms = 0666

func NewNurse(cfg NurseConfig) *Nurse {
	return &Nurse{
		Process:  *process.New("nurse"),
		cfg:      cfg,
		checks:   make(map[string]CheckFunc),
		chGather: make(chan gatherRequest, 1),
	}
}

func (n *Nurse) Start() error {
	err := n.Process.Start()
	if err != nil {
		return err
	}

	// This must be set *once*, and it must occur as early as possible
	runtime.MemProfileRate = n.cfg.MemProfileRate

	err = utils.EnsureDirAndMaxPerms(n.cfg.ProfileRoot, 0700)
	if err != nil {
		return err
	}

	n.AddCheck("mem", n.checkMem)
	n.AddCheck("goroutines", n.checkGoroutines)

	n.Process.Go(nil, "checker", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(n.cfg.PollInterval):
			}

			func() {
				n.checksMu.RLock()
				defer n.checksMu.RUnlock()
				for reason, checkFunc := range n.checks {
					if unwell, meta := checkFunc(); unwell {
						n.GatherVitals(reason, meta)
						break
					}
				}
			}()
		}
	})

	n.Process.Go(nil, "responder", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case req := <-n.chGather:
				n.gatherVitals(req.reason, req.meta)
			}
		}
	})

	return nil
}

func (n *Nurse) AddCheck(reason string, checkFunc CheckFunc) {
	n.checksMu.Lock()
	defer n.checksMu.Unlock()
	n.checks[reason] = checkFunc
}

func (n *Nurse) GatherVitals(reason string, meta Meta) {
	select {
	case n.chGather <- gatherRequest{reason, meta}:
	default:
	}
}

func (n *Nurse) checkMem() (bool, Meta) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	unwell := memStats.Alloc >= uint64(n.cfg.MemThreshold)
	if !unwell {
		return false, nil
	}
	return true, Meta{
		"mem_alloc", utils.FileSize(memStats.Alloc),
		"threshold", n.cfg.MemThreshold,
	}
}

func (n *Nurse) checkGoroutines() (bool, Meta) {
	num := runtime.NumGoroutine()
	unwell := num >= n.cfg.GoroutineThreshold
	if !unwell {
		return false, nil
	}
	return true, Meta{
		"num_goroutine", num,
		"threshold", n.cfg.GoroutineThreshold,
	}
}

func (n *Nurse) gatherVitals(reason string, meta Meta) {
	loggerFields := append([]any{"reason", reason}, meta...)

	n.Debugw("Nurse is gathering vitals", loggerFields...)

	size, err := n.totalProfileBytes()
	if err != nil {
		n.Errorw("could not fetch total profile bytes", append(loggerFields, "error", err)...)
		return
	} else if size >= uint64(n.cfg.MaxProfileSize) {
		n.Warnw("cannot write pprof profile, total profile size exceeds configured PPROF_MAX_PROFILE_SIZE",
			append(loggerFields, "total", size, "max", n.cfg.MaxProfileSize)...,
		)
		return
	}

	runtime.SetCPUProfileRate(n.cfg.CPUProfileRate)
	defer runtime.SetCPUProfileRate(0)
	runtime.SetBlockProfileRate(n.cfg.BlockProfileRate)
	defer runtime.SetBlockProfileRate(0)
	runtime.SetMutexProfileFraction(n.cfg.MutexProfileFraction)
	defer runtime.SetMutexProfileFraction(0)

	now := time.Now()

	var wg sync.WaitGroup
	wg.Add(9)

	go n.appendLog(now, reason, meta, &wg)
	go n.gatherCPU(now, &wg)
	go n.gatherTrace(now, &wg)
	go n.gather("allocs", now, &wg)
	go n.gather("block", now, &wg)
	go n.gather("goroutine", now, &wg)
	go n.gather("heap", now, &wg)
	go n.gather("mutex", now, &wg)
	go n.gather("threadcreate", now, &wg)

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		wg.Wait()
	}()

	select {
	case <-n.Process.Done():
	case <-ch:
	}
}

func (n *Nurse) appendLog(now time.Time, reason string, meta Meta, wg *sync.WaitGroup) {
	defer wg.Done()

	err := func() error {
		filename := filepath.Join(n.cfg.ProfileRoot, "nurse.log")
		mode := os.O_APPEND | os.O_CREATE | os.O_WRONLY

		file, err := os.OpenFile(filename, mode, profilePerms)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err = file.Write([]byte(fmt.Sprintf("==== %v\n", now))); err != nil {
			return err
		}
		if _, err = file.Write([]byte(fmt.Sprintf("reason: %v\n", reason))); err != nil {
			return err
		}
		for k, v := range meta {
			if _, err = file.Write([]byte(fmt.Sprintf("- %v: %v\n", k, v))); err != nil {
				return err
			}
		}
		_, err = file.Write([]byte("\n"))
		return err
	}()
	if err != nil {
		n.Errorw("could not append to log", log.Fields{"error": err})
		return
	}
}

func (n *Nurse) gatherCPU(now time.Time, wg *sync.WaitGroup) {
	defer wg.Done()

	file, err := n.openFile(now, "cpu")
	if err != nil {
		n.Errorw("could not write cpu profile", log.Fields{"error": err})
		return
	}
	defer file.Close()

	err = pprof.StartCPUProfile(file)
	if err != nil {
		n.Errorw("could not start cpu profile", log.Fields{"error": err})
		return
	}
	defer pprof.StopCPUProfile()

	select {
	case <-n.Process.Done():
	case <-time.After(n.cfg.GatherDuration):
	}
}

func (n *Nurse) gatherTrace(now time.Time, wg *sync.WaitGroup) {
	defer wg.Done()

	file, err := n.openFile(now, "trace")
	if err != nil {
		n.Errorw("could not write trace profile", log.Fields{"error": err})
		return
	}
	defer file.Close()

	err = trace.Start(file)
	if err != nil {
		n.Errorw("could not start trace profile", log.Fields{"error": err})
		return
	}
	defer trace.Stop()

	select {
	case <-n.Process.Done():
	case <-time.After(n.cfg.GatherDuration):
	}
}

func (n *Nurse) gather(typ string, now time.Time, wg *sync.WaitGroup) {
	defer wg.Done()

	p := pprof.Lookup(typ)
	if p == nil {
		n.Errorf("Invariant violation: pprof type '%v' does not exist", typ)
		return
	}

	p0, err := collectProfile(p)
	if err != nil {
		n.Errorw(fmt.Sprintf("could not collect %v profile", typ), log.Fields{"error": err})
		return
	}

	t := time.NewTimer(n.cfg.GatherDuration)
	defer t.Stop()

	select {
	case <-n.Process.Done():
		return
	case <-t.C:
	}

	p1, err := collectProfile(p)
	if err != nil {
		n.Errorw(fmt.Sprintf("could not collect %v profile", typ), log.Fields{"error": err})
		return
	}
	ts := p1.TimeNanos
	dur := p1.TimeNanos - p0.TimeNanos

	p0.Scale(-1)

	p1, err = profile.Merge([]*profile.Profile{p0, p1})
	if err != nil {
		n.Errorw(fmt.Sprintf("could not compute delta for %v profile", typ), log.Fields{"error": err})
		return
	}

	p1.TimeNanos = ts // set since we don't know what profile.Merge set for TimeNanos.
	p1.DurationNanos = dur

	file, err := n.openFile(now, typ)
	if err != nil {
		n.Errorw(fmt.Sprintf("could not write %v profile", typ), log.Fields{"error": err})
		return
	}
	defer file.Close()

	err = p1.Write(file)
	if err != nil {
		n.Errorw(fmt.Sprintf("could not write %v profile", typ), log.Fields{"error": err})
		return
	}
}

func collectProfile(p *pprof.Profile) (*profile.Profile, error) {
	var buf bytes.Buffer
	if err := p.WriteTo(&buf, 0); err != nil {
		return nil, err
	}
	ts := time.Now().UnixNano()
	p0, err := profile.Parse(&buf)
	if err != nil {
		return nil, err
	}
	p0.TimeNanos = ts
	return p0, nil
}

func (n *Nurse) openFile(now time.Time, typ string) (*os.File, error) {
	filename := filepath.Join(n.cfg.ProfileRoot, fmt.Sprintf("%v.%v.pprof", now, typ))
	mode := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	return os.OpenFile(filename, mode, profilePerms)
}

func (n *Nurse) totalProfileBytes() (uint64, error) {
	entries, err := os.ReadDir(n.cfg.ProfileRoot)
	if os.IsNotExist(err) {
		return 0, nil
	} else if err != nil {
		return 0, err
	}
	var size uint64
	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".pprof" && entry.Name() != "nurse.log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return 0, err
		}
		size += uint64(info.Size())
	}
	return size, nil
}
